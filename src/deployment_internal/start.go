package deployment_internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"

	. "github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/utils"
)

func RunStartPostHooks(ctx context.Context, depl *Deployment) error {

	//
	// Run the post-start hook (via systemd-run specific scope)
	//
	//     this can generate prometheus config for monitoring
	//     (generic systemd-run command or script)
	//
	//     a generic script can wait for the pod to be ready
	//     (better as a command inside a container or HTTP request)
	//
	//     this can also perform data migrations
	//

	_, err := daemon.SdNotify(false, "EXTEND_TIMEOUT_USEC=60000000") // 60s
	if err != nil {
		return err
	}

	log.Printf("executing post-start hooks...")
	err = depl.RunHooks(ctx, "post-start", depl.PartName, depl.Vars(), 60*time.Second)
	if err != nil {
		log.Printf("post-start hooks failed, continuing...")
	}
	return nil
}

func StartDeploymentConfig(ctx context.Context, depl *Deployment) error {
	//
	// Add IP address to load balancer
	//
	// notify load balancer of ip address, hook into existing load balancer
	// configuration for the service and add the IP address
	//

	log.Printf("Adding deployment to load-balancer...\n")
	cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "start", DeploymentConfigUnit(depl.DeploymentName))

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := utils.RunWithJournal(ctx, cmd, DeploymentConfigUnit(depl.DeploymentName))
	if err != nil {
		err = fmt.Errorf("failed to add deployment to load-balancer, %v", err)
		SdNotifyOrLog(err.Error())
		return err
	}

	return nil
}
