package deployment_public

import (
	"context"
	"os"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/utils"
)

func Start(ctx context.Context, deployment_name string) error {
	depl, err := deployment.ReadDeploymentByName(deployment_name, true)
	if err != nil {
		return err
	}

	cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "start", depl.MainUnit())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return utils.RunWithJournal(ctx, cmd, depl.MainUnit())
}
