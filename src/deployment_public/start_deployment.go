package deployment_public

import (
	"fmt"
	"os"
	"os/exec"

	. "github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/dirs"
)

func Start(deployment_name string) error {
	fmt.Fprintf(os.Stderr, "+ systemctl %s start %q\n", dirs.SystemdModeFlag(), DeploymentUnit(deployment_name))
	cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "start", DeploymentUnit(deployment_name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
