package deployment_public

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/dirs"
)

func Start(deployment_name string) error {
	depl, err := deployment.ReadDeploymentByName(deployment_name, true)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ systemctl %s start %q\n", dirs.SystemdModeFlag(), depl.MainUnit())
	cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "start", depl.MainUnit())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
