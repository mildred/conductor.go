package utils

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"strings"
)

var ValidYes = []string{"Y", "y"}
var ValidNo = []string{"N", "n"}

func CliQuestionString(prompt string) (string, error) {
	os.Stdout.Sync()
	os.Stderr.Sync()

	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s ", prompt)

	response, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	response = strings.TrimSpace(response)
	return response, nil
}

func CliQuestionChar(prompt string, valid_answers []string, valid_resp bool) (bool, error) {
	os.Stdout.Sync()
	os.Stderr.Sync()

	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s ", prompt)

	resp, _, err := reader.ReadRune()
	if err != nil {
		return false, err
	}

	response := strings.ToLower(string(resp))

	if slices.Contains(valid_answers, response) {
		return valid_resp, nil
	} else {
		return !valid_resp, nil
	}
}
