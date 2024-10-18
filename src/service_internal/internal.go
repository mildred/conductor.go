package service_internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"path"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/mildred/conductor.go/src/caddy"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/tmpl"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/deployment_public"
	"github.com/mildred/conductor.go/src/deployment_util"
	. "github.com/mildred/conductor.go/src/service"
)

// Note for services: a linked proxy config is set up when the service itself is
// enabled. The service depends on this proxy config

func StartOrRestart(restart bool) error {
	var prefix string = "start"
	var err error
	var ctx = context.Background()
	var dir = "."

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

	service, err := LoadServiceAndFillDefaults(path.Join(dir, ConfigName), true)
	if err != nil {
		return err
	}

	//
	// Find or create a suitable deployment
	//

	depl, depl_status, err := deployment_util.StartNewOrExistingFromService(ctx, service)
	if err != nil {
		return err
	}

	if depl_status == "started" {
		log.Printf("%s: Found started deployment %s", prefix, depl.DeploymentName)
	} else if depl_status == "starting" {
		log.Printf("%s: Found starting deployment %s, waiting to be started...", prefix, depl.DeploymentName)
		err = exec.Command("systemctl", "start", deployment.DeploymentUnit(depl.DeploymentName)).Run()
		if err != nil {
			return err
		}
	} else {
		log.Printf("%s: Starting new deployment %s...", prefix, depl.DeploymentName)
		err = exec.Command("systemctl", "start", deployment.DeploymentUnit(depl.DeploymentName)).Run()
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

	log.Printf("%s: Stopping obsolete deployments...\n", prefix)

	deployments, err := deployment_util.List()
	if err != nil {
		return err
	}

	for _, d := range deployments {
		if d.ServiceDir != service.BasePath || d.DeploymentName == depl.DeploymentName {
			continue
		}
		log.Printf("%s: Stopping deployment %s...\n", prefix, d.DeploymentName)
		deployment_public.Stop(d.DeploymentName)
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
			deployments, err := deployment_util.List()
			if err != nil {
				return err
			}

			found := false
			for _, d := range deployments {
				if d.ServiceDir == service.BasePath && d.DeploymentName == depl.DeploymentName {
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("Deployment has gone missing")
			}

			time.Sleep(30 * time.Second)
		}

	}
	return err
}

func Stop() error {
	var dir = "."

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

	service, err := LoadServiceAndFillDefaults(path.Join(dir, ConfigName), true)
	if err != nil {
		return err
	}

	//
	// Stop all deployments
	//

	log.Printf("stop: Stopping all deployments...\n")

	deployments, err := deployment_util.List()
	if err != nil {
		return err
	}

	for _, d := range deployments {
		if d.ServiceDir != service.BasePath {
			continue
		}
		log.Printf("stop: Stopping deployment %s...\n", d.DeploymentName)
		deployment_public.Stop(d.DeploymentName)
	}

	log.Printf("stop: Stop sequence completed\n")
	return nil
}

func Cleanup() error {
	//
	// Remove temp files if there is any
	//

	// log.Printf("cleanup: Cleaning up...\n")

	log.Printf("cleanup: Cleanup sequence completed\n")
	return nil
}

var LookupPaths []string = dirs.MultiJoin("services", append(append([]string{dirs.SelfRuntimeDir}, dirs.SelfConfigDirs...), dirs.SelfDataDirs...)...)

func CaddyRegister(register bool, dir string) error {
	service, err := LoadServiceAndFillDefaults(path.Join(dir, ConfigName), true)
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
		log.Printf("register: Register service")
	} else {
		log.Printf("register: Deregister service")
	}

	return caddy.Register(register, configs)
}
