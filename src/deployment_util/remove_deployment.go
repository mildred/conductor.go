package deployment_util

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/taigrr/systemctl"
	"github.com/taigrr/systemctl/properties"

	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/utils"
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

	load_state, err := systemctl.Show(ctx0, DeploymentServiceUnit(deployment_name), properties.LoadState, systemctl.Options{UserMode: !dirs.AsRoot})
	has_deployment := load_state == "loaded"
	if err != nil {
		return err
	}

	load_state, err = systemctl.Show(ctx0, DeploymentSocketUnit(deployment_name), properties.LoadState, systemctl.Options{UserMode: !dirs.AsRoot})
	has_socket := load_state == "loaded"
	if err != nil {
		return err
	}

	load_state, err = systemctl.Show(ctx0, DeploymentSocketConfigUnit(deployment_name), properties.LoadState, systemctl.Options{UserMode: !dirs.AsRoot})
	has_socket_config := load_state == "loaded"
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

	cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "stop", DeploymentSocketUnit(deployment_name), DeploymentServiceUnit(deployment_name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !has_socket {
		cmd.Stderr = nil
	}
	err = utils.RunWithJournal(ctx, cmd, DeploymentSocketUnit(deployment_name), DeploymentServiceUnit(deployment_name))
	if err != nil && ctx.Err() == nil && has_socket {
		return err
	}

	cancel()
	if timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx0, timeout)
		defer cancel() // should not be needed but else golang prints a warning
	}

	cmd = utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "stop", DeploymentServiceUnit(deployment_name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = utils.RunWithJournal(ctx, cmd, DeploymentServiceUnit(deployment_name))
	if err != nil && ctx.Err() == nil && (has_deployment || has_socket) {
		return err
	}

	failed_to_stop := timeout != 0 && ctx.Err() != nil

	if failed_to_stop && term_timeout != 0 {
		// Restart timeout for the SIGTERM signal
		cancel()
		ctx, cancel = context.WithTimeout(ctx0, term_timeout)
		defer cancel() // should not be needed but else golang prints a warning

		cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "kill", DeploymentServiceUnit(deployment_name))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := utils.RunWithJournal(ctx, cmd, DeploymentServiceUnit(deployment_name))
		if err != nil && ctx.Err() == nil && has_deployment {
			return err
		}
	}

	if failed_to_stop {
		cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "kill", "--signal=SIGKILL", DeploymentServiceUnit(deployment_name))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := utils.RunWithJournal(ctx, cmd, DeploymentServiceUnit(deployment_name))
		if err != nil && has_deployment {
			log.Printf("Error killing: %v", err)
		}

		cmd = utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "reset-failed", DeploymentServiceUnit(deployment_name))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	cmd = utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "stop", DeploymentConfigUnit(deployment_name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !has_config {
		cmd.Stderr = nil
	}
	err = utils.RunWithJournal(ctx, cmd, DeploymentConfigUnit(deployment_name))
	if err != nil && has_config {
		return err
	}

	cmd = utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "stop", DeploymentSocketConfigUnit(deployment_name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !has_socket_config {
		cmd.Stderr = nil
	}
	err = utils.RunWithJournal(ctx, cmd, DeploymentSocketConfigUnit(deployment_name))
	if err != nil && has_socket_config {
		return err
	}

	cmd = utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "stop", CGIFunctionSocketUnit(deployment_name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !has_cgi_function {
		cmd.Stderr = nil
	}
	err = utils.RunWithJournal(ctx, cmd, CGIFunctionSocketUnit(deployment_name))
	if err != nil && has_cgi_function {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -rf %q\n", DeploymentDirByNameOnly(deployment_name))
	err = os.RemoveAll(DeploymentDirByNameOnly(deployment_name))
	if err != nil {
		return err
	}

	systemd_run_dirs := []string{
		dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), DeploymentServiceUnit(deployment_name)+".d"),
		dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), DeploymentSocketUnit(deployment_name)),
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
