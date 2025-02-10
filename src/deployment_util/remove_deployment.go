package deployment_util

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"

	. "github.com/mildred/conductor.go/src/deployment"
)

func RemoveTimeout(ctx0 context.Context, deployment_name string, timeout, term_timeout time.Duration) error {
	sd, err := dbus.NewWithContext(ctx0)
	if err != nil {
		return err
	}

	statuses, err := sd.ListUnitsByNamesContext(ctx0, []string{
		DeploymentUnit(deployment_name),
		DeploymentConfigUnit(deployment_name),
		CGIFunctionSocketUnit(deployment_name)})
	if err != nil {
		return err
	}

	var has_deployment = false
	var has_config = false
	var has_cgi_function = false
	for _, status := range statuses {
		if status.Name == DeploymentUnit(deployment_name) {
			has_deployment = status.LoadState == "loaded"
		} else if status.Name == DeploymentConfigUnit(deployment_name) {
			has_config = status.LoadState == "loaded"
		} else if status.Name == CGIFunctionSocketUnit(deployment_name) {
			has_cgi_function = status.LoadState == "loaded"
		}
	}

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
	err = cmd.Run()
	if err != nil && ctx.Err() == nil && has_deployment {
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
		if err != nil && ctx.Err() == nil && has_deployment {
			return err
		}
	}

	if failed_to_stop {
		fmt.Fprintf(os.Stderr, "+ systemctl kill --signal=SIGKILL %q\n", DeploymentUnit(deployment_name))
		cmd := exec.Command("systemctl", "kill", "--signal=SIGKILL", DeploymentUnit(deployment_name))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil && has_deployment {
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

	fmt.Fprintf(os.Stderr, "+ systemctl stop %s\n", DeploymentConfigUnit(deployment_name))
	cmd = exec.Command("systemctl", "stop", DeploymentConfigUnit(deployment_name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !has_config {
		cmd.Stderr = nil
	}
	err = cmd.Run()
	if err != nil && has_config {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ systemctl stop %s\n", CGIFunctionSocketUnit(deployment_name))
	cmd = exec.Command("systemctl", "stop", CGIFunctionSocketUnit(deployment_name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !has_cgi_function {
		cmd.Stderr = nil
	}
	err = cmd.Run()
	if err != nil && has_cgi_function {
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
