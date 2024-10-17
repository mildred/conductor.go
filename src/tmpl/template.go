package tmpl

import (
	"log"
	"os/exec"
)

func RunTemplate(fname string, vars []string) (string, error) {
	if fname == "" {
		return "", nil
	}

	log.Printf("templating: execute %s\n", fname)
	cmd := exec.Command(fname, vars...)
	cmd.Env = append(cmd.Environ(), vars...)
	res, err := cmd.Output()
	return string(res), err
}
