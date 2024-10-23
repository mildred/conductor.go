package service_util

import (
	"fmt"
	"os"
	"os/exec"

	. "github.com/mildred/conductor.go/src/service"
)

func RunCommand(command *ServiceCommand, direct bool, cwd string, vars []string, cmd_name string, args ...string) error {
	if command == nil {
		return fmt.Errorf("Command %q does not exists", cmd_name)
	}

	if len(command.Exec) == 0 {
		return fmt.Errorf("Command %q is not executable", cmd_name)
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	args = append(command.Exec, args...)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = cwd
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("CONDUCTOR_COMMAND=%s", cmd_name))
	cmd.Env = append(cmd.Env, fmt.Sprintf("CONDUCTOR_COMMAND_DIR=%s", wd))
	cmd.Env = append(cmd.Env, vars...)

	err = cmd.Run()
	if !direct {
		return err
	}

	e := err.(*exec.ExitError)
	if e == nil {
		return err
	}

	if !e.Exited() {
		return err
	}

	if direct {
		os.Exit(e.ExitCode())
	}

	return err
}
