package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

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
		args := strings.Fields(input)
		if len(args) == 0 {
			fmt.Println("Please provide a command")
			continue
		}

		command := args[0]
		args = args[1:]
		cmdFn, exist := builtinCmd[command]
		if exist {
			stdOutput, err := cmdFn(args)
			if err != nil {
				fmt.Fprint(os.Stderr, err.Error())
			}

			fmt.Fprint(os.Stdout, stdOutput)
		} else {
			fmt.Printf("%s: command not found\n", input)
		}
	}
}

func initCommands() {
	registerCmd("exit", exitCmd)
	registerCmd("echo", echoCmd)
}

func registerCmd(key string, cmdFn CmdFn) {
	builtinCmd[key] = cmdFn
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
