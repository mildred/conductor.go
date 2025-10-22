package tmpl

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

var DefaultTimeout time.Duration = 30 * time.Second

func RunTemplate(ctx context.Context, fname string, vars []string) (string, error) {
	if fname == "" {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	log.Printf("templating: execute %s\n", fname)
	cmd := exec.CommandContext(ctx, fname, vars...)
	cmd.Env = append(cmd.Environ(), vars...)
	cmd.Stderr = os.Stderr
	res, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("while reading template output from %+v, %v", fname, err)
	}

	return string(res), nil
}

func RunTemplateStdout(ctx context.Context, fname string, vars []string) error {
	if fname == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, fname, vars...)
	cmd.Env = append(cmd.Environ(), vars...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	defer cancel()
	return cmd.Run()
}

func RunTemplateJSON(ctx context.Context, fname string, vars []string, res interface{}) error {
	data, err := RunTemplate(ctx, fname, vars)
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(data), res)
	if err != nil {
		return fmt.Errorf("while decoding JSON from template %+v, %v", fname, err)
	}

	return nil
}
