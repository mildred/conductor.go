package deployment_internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/mildred/conductor.go/src/tmpl"

	. "github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/dirs"
)

// Note for deployment hooks, there could be different ways to hook:
//
// - using system commands or script, executed into a specific scope via systemd-run
//
// - using podman exec on a container (except pre-start and post-stop), this is
// already scoped in the podman pod.
//
// - or as a HTTP query to the pod IP address with a retry mechanism

func StartPod(ctx context.Context, depl *Deployment) error {
	//
	// Start the pod or fail
	//

	log.Printf("start: Start the deployment pod\n")
	err := depl.StartStopPod(true, ".")
	if err != nil {
		err = fmt.Errorf("failed to start deployment pod, %v", err)
		SdNotifyOrLog(err.Error())
		return err
	}

	//
	// Find the pod IP address, add it to config
	//

	log.Printf("start: Looking up pod IP address...\n")
	addr, err := depl.FindPodIPAddress()
	if err != nil {
		err = fmt.Errorf("failed to find pod IP address, %v", err)
		SdNotifyOrLog(err.Error())
		return err
	}
	log.Printf("start: Found pod IP address: %s\n", addr)

	depl.Pod.IPAddress = addr

	err = depl.Save(ConfigName)
	if err != nil {
		err = fmt.Errorf("failed to save pod IP address, %v", err)
		SdNotifyOrLog(err.Error())
		return err
	}

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

	_, err = daemon.SdNotify(false, "EXTEND_TIMEOUT_USEC=60000000") // 60s
	if err != nil {
		return err
	}

	log.Printf("start: executing post-start hooks...")
	err = depl.RunHooks(ctx, "post-start", depl.Vars(), 60*time.Second)
	if err != nil {
		log.Printf("start: post-start hooks failed, continuing...")
	}

	//
	// Add IP address to load balancer
	//
	// notify load balancer of ip address, hook into existing load balancer
	// configuration for the service and add the IP address
	//

	log.Printf("start: Adding deployment to load-balancer...\n")
	fmt.Fprintf(os.Stderr, "+ systemctl %s start %q\n", dirs.SystemdModeFlag(), DeploymentConfigUnit(depl.DeploymentName))
	cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "start", DeploymentConfigUnit(depl.DeploymentName))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		err = fmt.Errorf("failed to add deployment to load-balancer, %v", err)
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

func StopPod(ctx context.Context, depl *Deployment) error {
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

	log.Printf("stop: executing pre-stop hooks...")
	err := depl.RunHooks(ctx, "pre-stop", depl.Vars(), 60*time.Second)
	if err != nil {
		log.Printf("stop: pre-stop hooks failed, continuing...")
	}

	//
	// Remove the IP address from the load balancer
	//

	log.Printf("stop: Removing deployment from load-balancer...\n")
	fmt.Fprintf(os.Stderr, "+ systemctl %s stop %q\n", dirs.SystemdModeFlag(), DeploymentConfigUnit(depl.DeploymentName))
	cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "stop", DeploymentConfigUnit(depl.DeploymentName))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Printf("stop: ERROR when removing from the load-balancer: %v\n", err)
	}

	//
	// Stop the containers
	//

	log.Printf("stop: Stopping the containers...\n")
	err = depl.StartStopPod(false, ".")
	if err != nil {
		err = fmt.Errorf("failed to stop pod, %v", err)
		SdNotifyOrLog(err.Error())
		return err
	}

	log.Printf("stop: Stop sequence completed\n")
	return nil
}

func Template(dir string, template string) error {
	depl, err := LoadDeployment(path.Join(dir, ConfigName))
	if err != nil {
		return err
	}

	return tmpl.RunTemplateStdout(context.Background(), template, depl.Vars())
}
