package service_util

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/mildred/conductor.go/src/deployment"
	. "github.com/mildred/conductor.go/src/service"
)

type ServiceCommandRunner struct {
	*Service
}

func (s *ServiceCommandRunner) RunCommandGetValue(c *ServiceCommand, cmd_name string, args ...string) (string, error) {
	cmd, err := PrepareCommand(c, s.BasePath, s.Vars(), cmd_name, args...)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer

	cmd.Stdout = &buf
	err = cmd.Run()
	return buf.String(), err
}

type DeploymentCommandRunner struct {
	*deployment.Deployment
}

func (depl *DeploymentCommandRunner) RunCommandGetValue(c *ServiceCommand, cmd_name string, args ...string) (string, error) {
	cmd, err := PrepareCommand(c, deployment.DeploymentDirByNameOnly(depl.DeploymentName), depl.Vars(), cmd_name, args...)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer

	cmd.Stdout = &buf
	err = cmd.Run()
	return buf.String(), err
}

func PrepareCommand(command *ServiceCommand, cwd string, vars []string, cmd_name string, args ...string) (*exec.Cmd, error) {
	if command == nil {
		return nil, fmt.Errorf("Command %q does not exists", cmd_name)
	}

	if len(command.Exec) == 0 {
		return nil, fmt.Errorf("Command %q is not executable", cmd_name)
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
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

	return cmd, nil
}

func RunCommand(command *ServiceCommand, direct bool, cwd string, vars []string, cmd_name string, args ...string) error {
	cmd, err := PrepareCommand(command, cwd, vars, cmd_name, args...)
	if err != nil {
		return err
	}

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
