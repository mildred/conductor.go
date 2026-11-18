package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/mildred/conductor.go/src/deployment_internal"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/deployment"
)

func (p *DaemonDeploymentPart) StartDeployment(ctx context.Context, depl *Deployment) error {
	var vars []string

	vars, err := depl.Vars(ctx)
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
		deployment_internal.SdNotifyOrLog(err.Error())
		return err
	}

	//
	// Notify that the deployment is ready
	//

	log.Printf("start: Startup sequence completed\n")
	_, err = daemon.SdNotify(false, daemon.SdNotifyReady)
	return err
}

func (p *DaemonDeploymentPart) StartPostDeployment(ctx context.Context, depl *Deployment) error {
	err := deployment_internal.RunStartPostHooks(ctx, depl)
	if err != nil {
		return err
	}

	err = deployment_internal.StartDeploymentConfig(ctx, depl)
	if err != nil {
		return err
	}

	return nil
}

func (p *DaemonDeploymentPart) StopDeployment(ctx context.Context, depl *Deployment) error {
	defer utils.AddPrefix(log.Default(), "stop daemon: ")()

	//
	// Run the pre-stop hooks (via systemd-run specific scope)
	//
	//    notify the service it should be stopping and should no longer accept
	//    connections or jobs
	//    (command inside the container or HTTP request)
	//
	//    wait for the service to be quiet
	//    (command inside the container or HTTP request)
	//
	vars, err := depl.Vars(ctx)
	if err != nil {
		return err
	}

	log.Printf("executing pre-stop hooks...")
	err = depl.RunHooks(ctx, "pre-stop", depl.PartName, vars, 60*time.Second)
	if err != nil {
		log.Printf("pre-stop hooks failed, continuing...")
	}

	//
	// Remove the IP address from the load balancer
	//

	err = deployment_internal.StopDeploymentConfig(ctx, depl)
	if err != nil {
		log.Printf("ERROR when removing from the load-balancer: %v\n", err)
	}

	err = deployment_internal.TerminateMainProcess(ctx, depl)
	if err != nil {
		deployment_internal.SdNotifyOrLog(err.Error())
		return err
	}

	log.Printf("Stop sequence completed\n")
	return nil
}
