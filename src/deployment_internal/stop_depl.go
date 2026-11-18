package deployment_internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/parts"
	"github.com/mildred/conductor.go/src/utils"
)

func Stop(ctx context.Context, deployment_name string, function bool) error {
	//
	// Notify the deployment is stopping
	//

	_, err := daemon.SdNotify(false, daemon.SdNotifyStopping)
	if err != nil {
		return err
	}

	//
	// Load deployment configuration
	//

	depl, err := deployment.ReadDeploymentByName(deployment_name, true, false)
	if err != nil {
		return err
	}

	log.Printf("stop: Loaded deployment %s, service %s-%s\n", depl.DeploymentName, depl.AppName, depl.InstanceName)

	if function && depl.PartType != parts.FunctionType {
		return fmt.Errorf("cannot start function on %+v part", depl.PartType)
	}

	return depl.StopDeployment(ctx, depl)
}

func StopDeploymentConfig(ctx context.Context, depl *deployment.Deployment) error {
	log.Printf(" Removing deployment from load-balancer...\n")
	cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "stop", deployment.DeploymentConfigUnit(depl.DeploymentName))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := utils.RunWithJournal(ctx, cmd, deployment.DeploymentConfigUnit(depl.DeploymentName))
	return err
}

func TerminateMainProcess(ctx context.Context, depl *deployment.Deployment) error {
	mainpid, err := strconv.ParseInt(os.Getenv("MAINPID"), 10, 0)
	if err != nil {
		return fmt.Errorf("Cannot parse MAINPID=%v, %v", os.Getenv("MAINPID"), err)
	}

	err = syscall.Kill(int(mainpid), syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("Cannot kill -TERM %v, %v", int(mainpid), err)
	}

	return nil
}

func Cleanup(deployment_name_or_dir string) error {
	ctx := context.Background()

	dir, deployment_name, err := deployment.DeploymentDirByName(deployment_name_or_dir, true, false)
	if err != nil {
		return err
	}

	log.Printf("cleanup: Cleaning up %s\n", dir)

	//
	// Run the post-stop hooks (via systemd-run specific scope), just in case
	//

	depl, err := deployment.ReadDeployment(dir, deployment_name)
	if err != nil {
		return err
	}

	log.Printf("cleanup: Loaded deployment %s, service %s-%s\n", depl.DeploymentName, depl.AppName, depl.InstanceName)

	vars, err := depl.Vars(ctx)
	if err != nil {
		return err
	}

	log.Printf("cleanup: executing post-stop hooks...")
	err = depl.RunHooks(ctx, "post-stop", depl.PartName, vars, 60*time.Second)
	if err != nil {
		return err
	}

	//
	// Remove deployment files
	//

	log.Printf("cleanup: Files left behind in %q\n", dir)
	log.Printf("cleanup: Cleanup sequence completed (deployment removal is up to the service)\n")
	return nil
}
