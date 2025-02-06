package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Fprint(os.Stdout, "$ ")
	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		fmt.Printf("Error parsing user input: %v", err)
		os.Exit(1)
	}

	input = strings.TrimSpace(input)
	fmt.Printf("%s: command not found\n", input)
}
