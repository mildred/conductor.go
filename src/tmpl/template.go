package tmpl

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
)

func RunTemplate(fname string, vars []string) (string, error) {
	if fname == "" {
		return "", nil
	}

	log.Printf("templating: execute %s\n", fname)
	cmd := exec.Command(fname, vars...)
	cmd.Env = append(cmd.Environ(), vars...)
	cmd.Stderr = os.Stderr
	res, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("while reading template output from %+v, %v", fname, err)
	}

	return string(res), nil
}

func RunTemplateStdout(fname string, vars []string) error {
	if fname == "" {
		return nil
	}

	cmd := exec.Command(fname, vars...)
	cmd.Env = append(cmd.Environ(), vars...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunTemplateJSON(fname string, vars []string, res interface{}) error {
	data, err := RunTemplate(fname, vars)
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(data), res)
	if err != nil {
		return fmt.Errorf("while decoding JSON from template %+v, %v", fname, err)
	}

	return nil
}
