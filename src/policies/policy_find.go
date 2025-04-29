package policies

import (
	"fmt"
	"os"
	"path"
	"strings"
)

func PolicyFind(name string) (string, error) {
	if strings.Contains(name, "/") {
		return PolicyRealpath(name)
	}

	for _, dir := range PolicyDirs {
		policy_dir := path.Join(dir, name)
		_, err := os.Stat(path.Join(policy_dir, ConfigName))
		if err != nil && !os.IsNotExist(err) {
			return "", err
		} else if err != nil {
			// ignore error, this is not a valid service dir
			continue
		}

		policy_dir, err = PolicyRealpath(policy_dir)
		if err != nil {
			return "", err
		}

		return policy_dir, nil
	}

	return "", &PolicyNotFoundError{name}
}

type PolicyNotFoundError struct {
	Name string
}

func (err *PolicyNotFoundError) Error() string {
	return fmt.Sprintf("policy %+v not found", err.Name)
}
