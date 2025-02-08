package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/term"
)

const PATH_ENV = "PATH"
const PWD_ENV = "PWD"
const HOME_ENV = "HOME"

type CmdFn = func([]string) (string, error)

var builtinCmd = map[string]CmdFn{}

func main() {
	initCommands()
	for {
		fmt.Fprint(os.Stdout, "$ ")
		input := readInput(os.Stdin)

		input = strings.TrimSpace(input)
		args := parseUserInput(input)
		if len(args) == 0 {
			fmt.Println("Please provide a command")
			continue
		}

		command := args[0]
		args = args[1:]
		redirectIndex := slices.IndexFunc(args, func(n string) bool {
			return isRedirectOperator(n)
		})
		redirectArgs := []string{}
		if redirectIndex >= 0 {
			redirectArgs = args[redirectIndex:]
			args = args[:redirectIndex]
		}

		cmdFn, exist := builtinCmd[command]
		if exist {
			stdOutput, err := cmdFn(args)
			var errorMsg string
			if err != nil {
				errorMsg = err.Error()
				fmt.Fprint(os.Stderr, errorMsg+"\n")
			}

			if redirectIndex >= 0 {
				redirect(stdOutput, errorMsg, redirectArgs)
			} else {
				fmt.Fprint(os.Stdout, stdOutput)
			}
		} else {
			output, errMsg := executeProgram(command, args)
			if errMsg != "" {
				if redirectIndex >= 0 && redirectArgs[0] != "2>" && redirectArgs[0] != "2>>" {
					fmt.Fprint(os.Stderr, errMsg)
				}
			}

			if redirectIndex >= 0 {
				redirect(output, errMsg, redirectArgs)
			} else {
				fmt.Fprint(os.Stdout, output)
			}
		}
	}
}

func initCommands() {
	registerCmd("exit", exitCmd)
	registerCmd("echo", echoCmd)
	registerCmd("type", typeCmd)
	registerCmd("pwd", pwdCmd)
	registerCmd("cd", cdCmd)
}

func registerCmd(key string, cmdFn CmdFn) {
	builtinCmd[key] = cmdFn
}

func readInput(rd io.Reader) string {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)
	reader := bufio.NewReader(rd)
	var input string

loop:
	for {
		char, _, err := reader.ReadRune()
		if err != nil {
			fmt.Println(err)
			continue
		}

		switch char {
		// ctrl + c
		case '\x03':
			fmt.Print("^C\n\r$ ")
			os.Exit(0)
		// enter
		case '\n', '\r':
			fmt.Fprintf(os.Stdout, "\r\n")
			break loop
		// backspace
		case '\x7F':
			if len(input) > 0 {
				input = input[:len(input)-1]
				fmt.Fprint(os.Stdout, "\b \b")
			}
		// tab
		case '\t':
			completion := autocomplete(input)
			if completion != "" {
				input = completion
			}

			fmt.Fprintf(os.Stdout, "\r\033[K$ %s", input)
		default:
			input += string(char)
			fmt.Fprint(os.Stdout, string(char))
		}
	}

	return input
}

func autocomplete(input string) string {
	if input == "" {
		return ""
	}

	r := len(input) - 1
	for r >= 0 && input[r] != ' ' {
		r -= 1
	}

	r++
	prefix := input[r:]
	first := input[:r]
	res := "" + first
	for key := range builtinCmd {
		if strings.Contains(key, prefix) {
			return res + key + " "
		}
	}

	directories := strings.Split(os.Getenv(PATH_ENV), ":")
	for _, directory := range directories {
		files, err := os.ReadDir(directory)
		if err == nil {
			for _, file := range files {
				if file.IsDir() {
					continue
				}

				fileName := file.Name()
				if strings.HasPrefix(fileName, prefix) {
					return res + fileName + " "
				}
			}
		}
	}

	return res + prefix + "\a"
}

func redirect(output, errorOutput string, redirectedArgs []string) {
	if len(redirectedArgs) < 2 {
		fmt.Fprint(os.Stdout, "please provide valid arguments for redirection")
		return
	}

	redirectOperator := redirectedArgs[0]
	filePath := redirectedArgs[1]
	switch redirectOperator {
	case "2>":
		if output != "" {
			fmt.Fprint(os.Stdout, output)
		}

		err := os.WriteFile(filePath, []byte(errorOutput), 0644)
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
		}
	case "2>>":
		if output != "" {
			fmt.Fprint(os.Stdout, output)
		}

		file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return
		}

		defer file.Close()
		file.WriteString(errorOutput)
	case ">>", "1>>":
		file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
			return
		}

		defer file.Close()
		file.WriteString(output)
	default:
		err := os.WriteFile(filePath, []byte(output), 0644)
		if err != nil {
			fmt.Fprint(os.Stdout, err.Error())
		}
	}
}

func isRedirectOperator(operator string) bool {
	operators := []string{"1>", ">", "2>", ">>", "1>>", "2>>"}
	return slices.Index(operators, operator) >= 0
}

func parseUserInput(input string) []string {
	args := []string{}
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false
	var current strings.Builder
	for _, char := range input {
		switch char {
		case '\\':
			if escaped || inSingleQuote {
				current.WriteRune(char)
				escaped = false
			} else {
				escaped = true
			}
		case '"':
			if escaped || inSingleQuote {
				current.WriteRune(char)
			} else {
				inDoubleQuote = !inDoubleQuote
			}

			escaped = false
		case '\'':
			if inDoubleQuote && escaped {
				current.WriteRune('\\')
			}

			if escaped || inDoubleQuote {
				current.WriteRune(char)
			} else {
				inSingleQuote = !inSingleQuote
			}

			escaped = false
		case ' ':
			if inDoubleQuote && escaped {
				current.WriteRune('\\')
			}

			if escaped || inSingleQuote || inDoubleQuote {
				current.WriteRune(char)
			} else if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}

			escaped = false
		default:
			if escaped && inDoubleQuote {
				current.WriteRune('\\')
			}

			current.WriteRune(char)
			escaped = false
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

func executeProgram(command string, args []string) (string, string) {
	if isInPath(command) {
		cmd := exec.Command(command, args...)
		output, err := cmd.Output()
		var errMsg string
		if err != nil {
			if stderr, ok := err.(*exec.ExitError); ok {
				errMsg = string(stderr.Stderr)
			} else {
				errMsg = err.Error()
			}
		}

		return string(output), errMsg
	}

	return fmt.Sprintf("%s: command not found\n", command), ""
}

func isInPath(command string) bool {
	paths := strings.Split(os.Getenv(PATH_ENV), ":")
	for _, p := range paths {
		filePath := path.Join(p, command)
		_, err := os.Stat(filePath)
		if !errors.Is(err, os.ErrNotExist) {
			return true
		}
	}

	return false
}

func cdCmd(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("please provide an argument for the cd command")
	}

	dirPath := args[0]
	if !path.IsAbs(dirPath) {
		if strings.Contains(dirPath, "~") {
			dirPath = strings.ReplaceAll(dirPath, "~", os.Getenv(HOME_ENV))
		} else {
			dirPath = path.Join(os.Getenv(PWD_ENV), dirPath)
		}
	}

	if _, err := os.Stat(dirPath); errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("cd: %s: No such file or directory", dirPath)
	}

	os.Setenv(PWD_ENV, dirPath)
	return "", nil
}

func pwdCmd(args []string) (string, error) {
	if len(args) > 0 {
		return "", errors.New("pwd: too many arguments")
	}

	res := os.Getenv(PWD_ENV)
	if res == "" {
		return "", errors.New("cannot get current working directory")
	}

	return res + "\n", nil
}

func typeCmd(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("please provide an argument for the type command")
	}

	command := args[0]
	_, exists := builtinCmd[command]
	if exists {
		return fmt.Sprintf("%s is a shell builtin\n", command), nil
	}

	paths := strings.Split(os.Getenv(PATH_ENV), ":")
	output := fmt.Sprintf("%s: not found", command)
	for _, p := range paths {
		filePath := path.Join(p, command)
		_, err := os.Stat(filePath)
		if !errors.Is(err, os.ErrNotExist) {
			output = fmt.Sprintf("%s is %s", command, filePath)
			break
		}
	}

	return output + "\n", nil
}

func echoCmd(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("please provide an argument for the echo command")
	}

	return strings.Join(args, " ") + "\n", nil
}

func exitCmd(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("please provide a status code for the exit command")
	}

	if len(args) > 1 {
		return "", errors.New("too many argument provided for the exit command")
	}

	code, err := strconv.Atoi(args[0])
	if err != nil {
		return "", err
	}

	os.Exit(code)
	return "", nil
}
