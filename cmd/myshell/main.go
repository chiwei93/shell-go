package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"slices"
	"strconv"
	"strings"
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
		input, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			fmt.Printf("Error parsing user input: %s", err.Error())
			os.Exit(1)
		}

		input = strings.TrimSpace(input)
		args := parseUserInput(input)
		if len(args) == 0 {
			fmt.Println("Please provide a command")
			continue
		}

		command := args[0]
		args = args[1:]
		redirectIndex, redirectArgs := getRedirectArgs(args)
		if redirectIndex >= 0 {
			args = args[:redirectIndex]
		}

		cmdFn, exist := builtinCmd[command]
		if exist {
			stdOutput, err := cmdFn(args)
			if err != nil {
				fmt.Fprint(os.Stderr, err.Error()+"\n")
				continue
			}

			if redirectIndex >= 0 {
				writeFile(stdOutput, redirectArgs)
			} else {
				fmt.Fprint(os.Stdout, stdOutput)
			}
		} else {
			output, errMsgs := executeProgram(command, args)
			if len(errMsgs) > 0 {
				for _, msg := range errMsgs {
					fmt.Fprint(os.Stderr, msg)
				}
			}

			if redirectIndex >= 0 {
				writeFile(output, redirectArgs)
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

func writeFile(result string, args []string) error {
	if len(args) == 0 {
		return errors.New("please provide an argument for the redirection")
	}

	err := os.WriteFile(args[0], []byte(result), 0644)
	if err != nil {
		return err
	}

	return nil
}

func getRedirectArgs(args []string) (int, []string) {
	redirectIndex := slices.IndexFunc(args, func(n string) bool {
		return strings.EqualFold(n, "1>") || strings.EqualFold(n, ">")
	})
	if redirectIndex < 0 {
		return redirectIndex, []string{}
	}

	return redirectIndex, args[redirectIndex+1:]
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

func executeProgram(command string, args []string) (string, []string) {
	if isInPath(command) {
		cmd := exec.Command(command, args...)
		output, err := cmd.Output()
		errMsgs := []string{}
		if err != nil {
			if stderr, ok := err.(*exec.ExitError); ok {
				errMsgs = append(errMsgs, string(stderr.Stderr))
			} else {
				errMsgs = append(errMsgs, err.Error())
			}
		}

		return string(output), errMsgs
	}

	return fmt.Sprintf("%s: command not found\n", command), []string{}
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
