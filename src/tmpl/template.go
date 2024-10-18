package tmpl

import (
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
	return string(res), err
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
