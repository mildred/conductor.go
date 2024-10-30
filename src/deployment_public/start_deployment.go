package deployment_public

import (
	"fmt"
	"os"
	"os/exec"

	. "github.com/mildred/conductor.go/src/deployment"
)

func Start(deployment_name string) error {
	fmt.Fprintf(os.Stderr, "+ systemctl start %q\n", DeploymentUnit(deployment_name))
	cmd := exec.Command("systemctl", "start", DeploymentUnit(deployment_name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
