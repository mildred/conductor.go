package deployment_util

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	. "github.com/mildred/conductor.go/src/deployment"
)

func RemoveTimeout(ctx0 context.Context, deployment_name string, timeout, term_timeout time.Duration) error {
	var cancel context.CancelFunc = func() {}
	var ctx = ctx0
	if timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx0, timeout)
		defer cancel()
	}

	fmt.Fprintf(os.Stderr, "+ systemctl stop %s\n", DeploymentUnit(deployment_name))
	cmd := exec.CommandContext(ctx, "systemctl", "stop", DeploymentUnit(deployment_name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil && ctx.Err() == nil {
		return err
	}

	failed_to_stop := timeout != 0 && ctx.Err() != nil

	if failed_to_stop && term_timeout != 0 {
		// Restart timeout for the SIGTERM signal
		cancel()
		ctx, cancel = context.WithTimeout(ctx0, term_timeout)
		defer cancel() // should not be needed but else golang prints a warning

		fmt.Fprintf(os.Stderr, "+ systemctl kill %s\n", DeploymentUnit(deployment_name))
		cmd := exec.CommandContext(ctx, "systemctl", "kill", DeploymentUnit(deployment_name))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil && ctx.Err() == nil {
			return err
		}
	}

	if failed_to_stop {
		fmt.Fprintf(os.Stderr, "+ systemctl kill --signal=SIGKILL %q\n", DeploymentUnit(deployment_name))
		cmd := exec.Command("systemctl", "kill", "--signal=SIGKILL", DeploymentUnit(deployment_name))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			log.Printf("Error killing: %v", err)
		}

		fmt.Fprintf(os.Stderr, "+ systemctl reset-failed %q\n", DeploymentUnit(deployment_name))
		cmd = exec.Command("systemctl", "reset-failed", DeploymentUnit(deployment_name))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	fmt.Fprintf(os.Stderr, "+ systemctl stop %s %s\n", DeploymentConfigUnit(deployment_name), CGIFunctionSocketUnit(deployment_name))
	cmd = exec.Command("systemctl", "stop", DeploymentConfigUnit(deployment_name), CGIFunctionSocketUnit(deployment_name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -rf %q\n", DeploymentDirByName(deployment_name))
	err = os.RemoveAll(DeploymentDirByName(deployment_name))
	if err != nil {
		return err
	}

	systemd_run_dirs := []string{
		"/run/systemd/system/" + DeploymentUnit(deployment_name) + ".d",
		"/run/systemd/system/" + CGIFunctionSocketUnit(deployment_name) + ".d",
		"/run/systemd/system/" + CGIFunctionSocketUnit(deployment_name),
		"/run/systemd/system/" + CGIFunctionServiceUnit(deployment_name, ""),
	}
	for _, systemd_run_dir := range systemd_run_dirs {
		fmt.Fprintf(os.Stderr, "+ rm -rf %q\n", systemd_run_dir)
		err = os.RemoveAll(systemd_run_dir)
		if err != nil {
			return err
		}
	}

	return nil
}
