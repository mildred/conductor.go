package socket

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"

	"github.com/coreos/go-systemd/v22/activation"

	"github.com/mildred/conductor.go/src/deployment_internal"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/deployment"
)

func (p *SocketDeploymentPart) StartDeployment(ctx context.Context, depl *Deployment) (err error) {
	defer utils.AddPrefix(log.Default(), "[start %s] ", depl.DeploymentName)()

	//
	// Register tubes
	//

	vars, err := depl.Vars(ctx)
	if err != nil {
		return err
	}

	for i, f := range activation.Files(false) {
		var tubes []string
		if i < len(depl.Socket.RegisterTubes) {
			tubes = depl.Socket.RegisterTubes[i]
		}
		for _, tube := range utils.ExpandAllVars(tubes, vars) {
			cmd := exec.Command("tubectl", "register", tube)
			cmd.ExtraFiles = []*os.File{f}
			cmd.Env = append(os.Environ(), "LISTEN_FDS=1")

			utils.LogCmd("+ %s 3<&%d", cmd, f.Fd())

			wait := utils.CommandLogOutput(cmd)
			defer wait()

			e := cmd.Run()
			if e != nil {
				errors.Join(err, e)
			}
		}

	}

	return err
}

func (p *SocketDeploymentPart) StartPostDeployment(ctx context.Context, depl *Deployment) error {
	defer utils.AddPrefix(log.Default(), "[post-start %s] ", depl.DeploymentName)()

	err := deployment_internal.RunStartPostHooks(ctx, depl)
	if err != nil {
		return err
	}

	return nil
}

func (p *SocketDeploymentPart) StopDeployment(ctx context.Context, depl *Deployment) error {
	defer utils.AddPrefix(log.Default(), "[stop %s] ", depl.DeploymentName)()

	//
	// TODO: unregister tubes (might not be needed)
	//

	return nil
}
