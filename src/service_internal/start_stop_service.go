package service_internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"slices"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/mildred/conductor.go/src/caddy"
	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/deployment_public"
	"github.com/mildred/conductor.go/src/deployment_util"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/tmpl"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/service"
)

// Note for services: a linked proxy config is set up when the service itself is
// enabled. The service depends on this proxy config

func StartOrRestart(restart bool, service_name string, max_deployment_index int) error {
	var prefix string = "start"
	var err error
	var ctx = context.Background()

	//
	// [restart] Notify systemd reload in progress
	//

	if restart {
		prefix = "restart"

		var notified bool
		notified, err = daemon.SdNotify(false, daemon.SdNotifyStopping)
		if err != nil {
			return err
		}

		if notified {
			defer func() {
				_, er := daemon.SdNotify(false, daemon.SdNotifyReady)
				if er != nil && err != nil {
					err = fmt.Errorf("%v; additionally, there was an error notifying the end of the reloading process: %v", err, er)
				} else if er != nil {
					err = er
				}
			}()
		}
	}

	//
	// Fetch service config
	//

	service, err := LoadServiceByName(service_name)
	if err != nil {
		return err
	}

	//
	// Find or create a suitable deployment
	//

	parts, err := service.Parts()
	if err != nil {
		return err
	}

	var started_deployments []*deployment.Deployment
	var depl_names []string

	for _, part_name := range parts {

		log.Printf("%s: Loaded service, find new or existing deployments for part %q...\n", prefix, part_name)

		seed, err := deployment.SeedFromService(service, part_name)
		if err != nil {
			return err
		}

		depl, depl_status, err := deployment_util.StartNewOrExistingFromService(ctx, service, seed, max_deployment_index)
		if err != nil {
			return err
		}

		started_deployments = append(started_deployments, depl)
		depl_names = append(depl_names, depl.DeploymentName)

		ctx, cancel := context.WithCancel(context.Background())
		go utils.ExtendTimeout(ctx, 60*time.Second)

		err = func() error {
			defer cancel()

			if depl_status == "active" {
				log.Printf("%s: Found started deployment %s", prefix, depl.DeploymentName)
			} else if depl_status == "activating" || depl_status == "inactive" {
				log.Printf("%s: Found %s deployment %s, waiting to be started...", prefix, depl_status, depl.DeploymentName)
				fmt.Fprintf(os.Stderr, "+ systemctl start %q\n", deployment.DeploymentUnit(depl.DeploymentName))
				cmd := exec.Command("systemctl", "start", deployment.DeploymentUnit(depl.DeploymentName))
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				err = cmd.Run()
				if err != nil {
					return err
				}
			} else {
				log.Printf("%s: Starting new deployment %s...", prefix, depl.DeploymentName)
				fmt.Fprintf(os.Stderr, "+ systemctl start %q\n", deployment.DeploymentUnit(depl.DeploymentName))
				cmd := exec.Command("systemctl", "start", deployment.DeploymentUnit(depl.DeploymentName))
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				err = cmd.Run()
				if err != nil {
					return err
				}
			}

			return nil
		}()
		if err != nil {
			return err
		}

	}

	//
	// TODO [ if restarting ] if failing to start, exit uncleanly
	// TODO [ if starting ] if failing, stop all deployments and exit uncleanly
	//

	//
	// Stop all deployments that are of older config version
	//

	log.Printf("%s: Removing obsolete deployments (except %v)...\n", prefix, depl_names)

	deployments, err := deployment_util.List(deployment_util.ListOpts{
		FilterServiceDir: service.BasePath,
	})
	if err != nil {
		return err
	}

	for _, d := range deployments {
		if slices.Contains(depl_names, d.DeploymentName) {
			continue
		}

		log.Printf("%s: Removing deployment %s...\n", prefix, d.DeploymentName)

		ctx, cancel := context.WithCancel(context.Background())
		go utils.ExtendTimeout(ctx, 60*time.Second)

		err = func() error {
			defer cancel()
			return deployment_public.Remove(d.DeploymentName)
		}()
		if err != nil {
			log.Printf("%s: ERROR removing deployment %s (but continuing): %v", prefix, d.DeploymentName, err)
		}
	}

	//
	// Notify systemd ready
	//

	if restart {
		log.Printf("restart: Restart sequence completed\n")
	} else {

		_, err = daemon.SdNotify(false, daemon.SdNotifyReady)
		if err != nil {
			return err
		}
		log.Printf("start: Start sequence completed, start to monitor deployments\n")

		//
		// Keep running in the background, and monitor the deployments
		// (exit with an error if a deployment is missing)
		//

		for {
			deployments, err := deployment_util.List(deployment_util.ListOpts{
				FilterServiceDir: service.BasePath,
				FilterServiceId:  service.Id,
			})
			if err != nil {
				return err
			}

			if len(deployments) == 0 {
				return fmt.Errorf("Deployment has gone missing")
			}

			time.Sleep(30 * time.Second)
		}

	}
	return err
}

func Stop(service_name string) error {
	//
	// Notify stop in progress
	//

	_, err := daemon.SdNotify(false, daemon.SdNotifyStopping)
	if err != nil {
		return err
	}

	//
	// Fetch service config
	//

	service, err := LoadServiceByName(service_name)
	if err != nil {
		return err
	}

	//
	// Stop all deployments
	//

	log.Printf("stop: Stopping all deployments...\n")

	deployments, err := deployment_util.List(deployment_util.ListOpts{
		FilterServiceDir: service.BasePath,
	})
	if err != nil {
		return err
	}

	for _, d := range deployments {
		log.Printf("stop: Stopping deployment %s...\n", d.DeploymentName)

		ctx, cancel := context.WithCancel(context.Background())
		go utils.ExtendTimeout(ctx, 60*time.Second)

		func() {
			defer cancel()
			deployment_public.Stop(d.DeploymentName)
		}()
	}

	log.Printf("stop: Stop sequence completed\n")
	return nil
}

func Cleanup(service_name string) error {
	//
	// Remove temp files if there is any
	//

	// log.Printf("cleanup: Cleaning up...\n")

	log.Printf("cleanup: Cleanup sequence completed\n")
	return nil
}

var LookupPaths []string = dirs.MultiJoin("services", append(append([]string{dirs.SelfRuntimeDir}, dirs.SelfConfigDirs...), dirs.SelfDataDirs...)...)

func CaddyRegister(register bool, service_name string) error {
	var prefix = "register"
	if !register {
		prefix = "deregister"
	}

	service, err := LoadServiceByName(service_name)
	if err != nil {
		return err
	}

	if service.ProxyConfigTemplate == "" {
		return nil
	}

	var configs []caddy.ConfigItem

	caddy, err := caddy.NewClient(service.CaddyLoadBalancer.ApiEndpoint)
	if err != nil {
		return err
	}

	config, err := tmpl.RunTemplate(service.ProxyConfigTemplate, service.Vars())
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(config), &configs)
	if err != nil {
		return err
	}

	if register {
		log.Printf("register: Registering service...")
	} else {
		log.Printf("deregister: Deregistering service...")
	}

	err = caddy.Register(register, configs)
	if err != nil {
		return err
	}

	log.Printf("%s: Completed", prefix)
	return nil
}

func Template(service_name string, template string) error {
	service, err := LoadServiceByName(service_name)
	if err != nil {
		return err
	}

	return tmpl.RunTemplateStdout(template, service.Vars())
}
