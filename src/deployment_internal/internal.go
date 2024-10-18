package deployment_internal

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"

	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/mildred/conductor.go/src/caddy"
	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/tmpl"

	. "github.com/mildred/conductor.go/src/deployment"
)

// Note for deployment hooks, there could be different ways to hook:
//
// - using system commands or script, executed into a specific scope via systemd-run
//
// - using podman exec on a container (except pre-start and post-stop), this is
// already scoped in the podman pod.
//
// - or as a HTTP query to the pod IP address with a retry mechanism

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
	fmt.Fprintf(os.Stderr, "+ systemctl start %q\n", DeploymentConfigUnit(depl.DeploymentName))
	cmd := exec.Command("systemctl", "start", DeploymentConfigUnit(depl.DeploymentName))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
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
	fmt.Fprintf(os.Stderr, "+ systemctl stop %q\n", DeploymentConfigUnit(depl.DeploymentName))
	cmd := exec.Command("systemctl", "stop", DeploymentConfigUnit(depl.DeploymentName))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
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

	log.Printf("cleanup: Files left behind in %q\n", cwd)
	log.Printf("cleanup: Cleanup sequence completed (deployment removal is up to the service)\n")
	return nil
}

func CaddyRegister(register bool, dir string) error {
	var prefix = "register"
	if !register {
		prefix = "deregister"
	}

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
		unit_name := fmt.Sprintf(service.ServiceConfigUnit(depl.ServiceDir))
		log.Printf("%s: Ensure the service config %s is registered", prefix, unit_name)

		fmt.Fprintf(os.Stderr, "+ systemctl start %q\n", unit_name)
		cmd := exec.Command("systemctl", "start", unit_name)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}

		log.Printf("register: Registering pod IP %s", depl.PodIpAddress)
	} else {
		log.Printf("deregister: Deregistering pod IP %s", depl.PodIpAddress)
	}

	err = caddy.Register(register, configs)
	if err != nil {
		return err
	}

	log.Printf("%s: Completed", prefix)
	return nil
}

func Template(dir string, template string) error {
	depl, err := LoadDeployment(path.Join(dir, ConfigName))
	if err != nil {
		return err
	}

	return tmpl.RunTemplateStdout(template, depl.Vars())
}
