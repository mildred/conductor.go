package deployment_internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/coreos/go-systemd/v22/daemon"

	. "github.com/mildred/conductor.go/src/deployment"
)

func StartDaemon(ctx context.Context, depl *Deployment) error {
	var vars []string

	vars, err := depl.Vars()
	if err != nil {
		return err
	}

	cmd := exec.Command(depl.Daemon.Exec[0], depl.Daemon.Exec[1:]...)
	cmd.Env = append(cmd.Environ(), vars...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		err = fmt.Errorf("failed to start process %s, %v", strings.Join(cmd.Args, " "), err)
		SdNotifyOrLog(err.Error())
		return err
	}

	//
	// Notify that the deployment is ready
	//

	log.Printf("start: Startup sequence completed\n")
	_, err = daemon.SdNotify(false, daemon.SdNotifyReady)
	return err
}

func StartPostDaemon(ctx context.Context, depl *Deployment) error {
	err := RunStartPostHooks(ctx, depl)
	if err != nil {
		return err
	}

	err = StartDeploymentConfig(ctx, depl)
	if err != nil {
		return err
	}

	return nil
}
