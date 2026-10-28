package deployment_public

import (
	"context"
	"fmt"
	"os"

	"github.com/taigrr/systemctl"
	"github.com/taigrr/systemctl/properties"

	. "github.com/mildred/conductor.go/src/deployment"
	. "github.com/mildred/conductor.go/src/deployment_util"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/utils"
)

func listStopUnits(ctx context.Context, deployment_name string, include_sockets bool) ([]string, error) {
	var stop_units []string

	load_state, err := systemctl.Show(ctx, DeploymentServiceUnit(deployment_name), properties.LoadState, systemctl.Options{UserMode: !dirs.AsRoot})
	has_deployment := load_state == "loaded"
	if err != nil {
		return nil, err
	} else if has_deployment {
		stop_units = append(stop_units, DeploymentServiceUnit(deployment_name))
	}

	if include_sockets {
		load_state, err = systemctl.Show(ctx, DeploymentSocketUnit(deployment_name), properties.LoadState, systemctl.Options{UserMode: !dirs.AsRoot})
		has_socket := load_state == "loaded"
		if err != nil {
			return nil, err
		} else if has_socket {
			stop_units = append(stop_units, DeploymentSocketUnit(deployment_name))
		}

		load_state, err = systemctl.Show(ctx, CGIFunctionSocketUnit(deployment_name), properties.LoadState, systemctl.Options{UserMode: !dirs.AsRoot})
		has_cgi_function := load_state == "loaded"
		if err != nil {
			return nil, err
		} else if has_cgi_function {
			stop_units = append(stop_units, CGIFunctionSocketUnit(deployment_name))
		}
	}

	return stop_units, nil
}

func Stop(ctx context.Context, deployment_name string) error {
	stop_units, err := listStopUnits(ctx, deployment_name, true)
	if err != nil {
		return err
	}

	cmd := utils.LogCommand(ctx, "systemctl", append([]string{dirs.SystemdModeFlag(), "stop"}, stop_units...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return utils.RunWithJournal(ctx, cmd, stop_units...)
}

func Remove(deployment_name string) error {
	return RemoveTimeout(context.Background(), deployment_name, 0, 0)
}

func Kill(ctx context.Context, deployment_name string, signal string) error {
	units, err := listStopUnits(ctx, deployment_name, false)
	if err != nil {
		return err
	}

	args := []string{dirs.SystemdModeFlag(), "kill"}
	if signal != "" {
		args = append(args, fmt.Sprint("--signal=%s", signal))
	}
	args = append(args, units...)

	cmd := utils.LogCommand(ctx, "systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return utils.RunWithJournal(ctx, cmd, units...)
}
