package deployment_public

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	. "github.com/mildred/conductor.go/src/deployment"
	. "github.com/mildred/conductor.go/src/deployment_util"
)

func Stop(deployment_name string) error {
	fmt.Fprintf(os.Stderr, "+ systemctl stop %s %s\n", DeploymentUnit(deployment_name), CGIFunctionSocketUnit(deployment_name))
	cmd := exec.Command("systemctl", "stop", DeploymentUnit(deployment_name), CGIFunctionSocketUnit(deployment_name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Remove(deployment_name string) error {
	return RemoveTimeout(context.Background(), deployment_name, 0, 0)
}
