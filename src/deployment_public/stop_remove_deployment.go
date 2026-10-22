package deployment_public

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/taigrr/systemctl"
	"github.com/taigrr/systemctl/properties"

	. "github.com/mildred/conductor.go/src/deployment"
	. "github.com/mildred/conductor.go/src/deployment_util"
	"github.com/mildred/conductor.go/src/dirs"
)

func Stop(ctx0 context.Context, deployment_name string) error {
	var stop_units []string

	load_state, err := systemctl.Show(ctx0, DeploymentServiceUnit(deployment_name), properties.LoadState, systemctl.Options{UserMode: !dirs.AsRoot})
	has_deployment := load_state == "loaded"
	if err != nil {
		return err
	} else if has_deployment {
		stop_units = append(stop_units, DeploymentServiceUnit(deployment_name))
	}

	load_state, err = systemctl.Show(ctx0, DeploymentSocketUnit(deployment_name), properties.LoadState, systemctl.Options{UserMode: !dirs.AsRoot})
	has_socket := load_state == "loaded"
	if err != nil {
		return err
	} else if has_socket {
		stop_units = append(stop_units, DeploymentSocketUnit(deployment_name))
	}

	load_state, err = systemctl.Show(ctx0, CGIFunctionSocketUnit(deployment_name), properties.LoadState, systemctl.Options{UserMode: !dirs.AsRoot})
	has_cgi_function := load_state == "loaded"
	if err != nil {
		return err
	} else if has_cgi_function {
		stop_units = append(stop_units, CGIFunctionSocketUnit(deployment_name))
	}

	fmt.Fprintf(os.Stderr, "+ systemctl %s stop %v\n", dirs.SystemdModeFlag(), stop_units)
	cmd := exec.CommandContext(ctx0, "systemctl", append([]string{dirs.SystemdModeFlag(), "stop"}, stop_units...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Remove(deployment_name string) error {
	return RemoveTimeout(context.Background(), deployment_name, 0, 0)
}
