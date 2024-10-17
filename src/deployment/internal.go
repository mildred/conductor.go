package deployment

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/coreos/go-systemd/v22/unit"

	"github.com/mildred/conductor.go/src/caddy"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/tmpl"
)

// Note for deployment hooks, there could be different ways to hook:
//
// - using system commands or script, executed into a specific scope via systemd-run
//
// - using podman exec on a container (except pre-start and post-stop), this is
// already scoped in the podman pod.
//
// - or as a HTTP query to the pod IP address with a retry mechanism

var DeploymentRunDir = dirs.Join(dirs.SelfRuntimeDir, "deployments")

func Prepare() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	deployment := path.Base(cwd)

	log.Printf("prepare: Prepare deployment %s\n", cwd)

	//
	// Create deployment config from service and run the templates
	//

	depl, err := ReadDeployment(".", deployment)
	if err != nil {
		return err
	}

	err = depl.TemplateAll()
	if err != nil {
		return err
	}

	log.Printf("prepare: Saving deployment config %s\n", ConfigName)
	err = depl.Save(ConfigName)
	if err != nil {
		return err
	}

	//
	// Run the pre-start hooks (via systemd-run specific scope)
	//

	err = depl.RunHooks("pre-start")
	if err != nil {
		return err
	}

	log.Printf("prepare: Preparation sequence completed\n")

	return nil
}

func Start() error {
	depl, err := LoadDeployment(ConfigName)
	if err != nil {
		return err
	}

	//
	// Start the pod or fail
	//

	log.Printf("start: Start the deployment pod\n")
	err = depl.StartStopPod(true, ".")
	if err != nil {
		return err
	}

	//
	// Find the pod IP address, add it to config
	//

	log.Printf("start: Looking up pod IP address...\n")
	addr, err := depl.FindPodIPAddress()
	if err != nil {
		return err
	}
	log.Printf("start: Found pod IP address: %s\n", addr)

	depl.PodIpAddress = addr

	err = depl.Save(ConfigName)
	if err != nil {
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

	err = depl.RunHooks("post-start")
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
	cmd := exec.Command("systemctl", "start", "conductor-deployment-config@"+depl.DeploymentName+".service")
	err = cmd.Run()
	if err != nil {
		return err
	}

	//
	// Notify that the deployment is ready
	//

	log.Printf("start: Startup sequence completed\n")
	_, err = daemon.SdNotify(false, daemon.SdNotifyReady)
	return err
}

func Stop() error {
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

	depl, err := LoadDeployment(ConfigName)
	if err != nil {
		return err
	}

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

	err = depl.RunHooks("pre-stop")
	if err != nil {
		log.Printf("stop: pre-stop hooks failed, continuing...")
	}

	//
	// Remove the IP address from the load balancer
	//

	log.Printf("stop: Removing deployment from load-balancer...\n")
	cmd := exec.Command("systemctl", "stop", "conductor-deployment-config@"+depl.DeploymentName+".service")
	err = cmd.Run()
	if err != nil {
		return err
	}

	if err != nil {
		log.Printf("stop: ERROR when removing from the load-balancer: %v\n", err)
	}

	//
	// Stop the containers
	//

	log.Printf("stop: Stopping the containers...\n")
	err = depl.StartStopPod(true, ".")
	if err != nil {
		return err
	}

	log.Printf("stop: Stop sequence completed\n")
	return nil
}

func Cleanup() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	log.Printf("cleanup: Cleaning up %s\n", cwd)

	//
	// Run the post-stop hooks (via systemd-run specific scope), just in case
	//

	depl, err := LoadDeployment(ConfigName)
	if err != nil {
		return err
	}

	err = depl.RunHooks("post-stop")
	if err != nil {
		return err
	}

	//
	// Remove deployment files
	//

	err = os.Chdir("/")
	if err != nil {
		return err
	}

	err = os.RemoveAll(cwd)
	if err != nil {
		return err
	}

	log.Printf("cleanup: Cleanup sequence completed\n", cwd)
	return nil
}

func CaddyRegister(register bool, dir string) error {
	depl, err := LoadDeployment(ConfigName)
	if err != nil {
		return err
	}

	if depl.ProxyConfigTemplate == "" {
		return nil
	}

	var configs []caddy.ConfigItem

	caddy, err := caddy.NewClient(depl.CaddyLoadBalancer.ApiEndpoint)
	if err != nil {
		return err
	}

	config, err := tmpl.RunTemplate(depl.ProxyConfigTemplate, depl.Vars())
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(config), &configs)
	if err != nil {
		return err
	}

	if register {
		unit_name := fmt.Sprintf("conductor-service-config@%s.service", unit.UnitNamePathEscape(depl.ServiceDir))
		log.Printf("register: Ensure the service config %s is registered", unit_name)

		err = exec.Command("systemctl", "start", unit_name).Run()
		if err != nil {
			return err
		}

		log.Printf("register: Register pod IP %s", depl.PodIpAddress)
	} else {
		log.Printf("register: Deregister pod IP %s", depl.PodIpAddress)
	}

	return caddy.Register(register, configs)
}
