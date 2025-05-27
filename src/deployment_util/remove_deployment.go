package deployment_util

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/taigrr/systemctl"
	"github.com/taigrr/systemctl/properties"

	"github.com/mildred/conductor.go/src/dirs"
	_ "github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/deployment"
)

func RemoveTimeout(ctx0 context.Context, deployment_name string, timeout, term_timeout time.Duration) error {
	// sd, err := utils.NewSystemdClient(ctx0)
	// if err != nil {
	//   return err
	// }

	// statuses, err := sd.ListUnitsByNamesContext(ctx0, []string{
	// 	DeploymentUnit(deployment_name),
	// 	DeploymentConfigUnit(deployment_name),
	// 	CGIFunctionSocketUnit(deployment_name)})
	// if err != nil {
	// 	return err
	// }

	// var has_deployment = false
	// var has_config = false
	// var has_cgi_function = false
	// for _, status := range statuses {
	// 	if status.Name == DeploymentUnit(deployment_name) {
	// 		has_deployment = status.LoadState == "loaded"
	// 	} else if status.Name == DeploymentConfigUnit(deployment_name) {
	// 		has_config = status.LoadState == "loaded"
	// 	} else if status.Name == CGIFunctionSocketUnit(deployment_name) {
	// 		has_cgi_function = status.LoadState == "loaded"
	// 	}
	// }

	load_state, err := systemctl.Show(ctx0, DeploymentUnit(deployment_name), properties.LoadState, systemctl.Options{UserMode: !dirs.AsRoot})
	has_deployment := load_state == "loaded"
	if err != nil {
		return err
	}

	load_state, err = systemctl.Show(ctx0, DeploymentConfigUnit(deployment_name), properties.LoadState, systemctl.Options{UserMode: !dirs.AsRoot})
	has_config := load_state == "loaded"
	if err != nil {
		return err
	}

	load_state, err = systemctl.Show(ctx0, CGIFunctionSocketUnit(deployment_name), properties.LoadState, systemctl.Options{UserMode: !dirs.AsRoot})
	has_cgi_function := load_state == "loaded"
	if err != nil {
		return err
	}

	var cancel context.CancelFunc = func() {}
	var ctx = ctx0
	if timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx0, timeout)
		defer cancel()
	}

	fmt.Fprintf(os.Stderr, "+ systemctl %s stop %s\n", dirs.SystemdModeFlag(), DeploymentUnit(deployment_name))
	cmd := exec.CommandContext(ctx, "systemctl", dirs.SystemdModeFlag(), "stop", DeploymentUnit(deployment_name))
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

		fmt.Fprintf(os.Stderr, "+ systemctl %s kill %s\n", dirs.SystemdModeFlag(), DeploymentUnit(deployment_name))
		cmd := exec.CommandContext(ctx, "systemctl", dirs.SystemdModeFlag(), "kill", DeploymentUnit(deployment_name))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil && ctx.Err() == nil && has_deployment {
			return err
		}
	}

	if failed_to_stop {
		fmt.Fprintf(os.Stderr, "+ systemctl %s kill --signal=SIGKILL %q\n", dirs.SystemdModeFlag(), DeploymentUnit(deployment_name))
		cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "kill", "--signal=SIGKILL", DeploymentUnit(deployment_name))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil && has_deployment {
			log.Printf("Error killing: %v", err)
		}

		fmt.Fprintf(os.Stderr, "+ systemctl %s reset-failed %q\n", dirs.SystemdModeFlag(), DeploymentUnit(deployment_name))
		cmd = exec.Command("systemctl", dirs.SystemdModeFlag(), "reset-failed", DeploymentUnit(deployment_name))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	fmt.Fprintf(os.Stderr, "+ systemctl %s stop %s\n", dirs.SystemdModeFlag(), DeploymentConfigUnit(deployment_name))
	cmd = exec.Command("systemctl", dirs.SystemdModeFlag(), "stop", DeploymentConfigUnit(deployment_name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !has_config {
		cmd.Stderr = nil
	}
	err = cmd.Run()
	if err != nil && has_config {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ systemctl %s stop %s\n", dirs.SystemdModeFlag(), CGIFunctionSocketUnit(deployment_name))
	cmd = exec.Command("systemctl", dirs.SystemdModeFlag(), "stop", CGIFunctionSocketUnit(deployment_name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !has_cgi_function {
		cmd.Stderr = nil
	}
	err = cmd.Run()
	if err != nil && has_cgi_function {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -rf %q\n", DeploymentDirByName(deployment_name, false))
	err = os.RemoveAll(DeploymentDirByName(deployment_name, false))
	if err != nil {
		return err
	}

	systemd_run_dirs := []string{
		dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), DeploymentUnit(deployment_name)+".d"),
		dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), CGIFunctionSocketUnit(deployment_name)+".d"),
		dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), CGIFunctionSocketUnit(deployment_name)),
		dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), CGIFunctionServiceUnitSingle(deployment_name)),
		dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), CGIFunctionServiceUnit(deployment_name, "")),
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
