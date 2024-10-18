package service_internal

import (
	"encoding/json"
	"fmt"
	"log"
	"path"

	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/mildred/conductor.go/src/caddy"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/tmpl"

	. "github.com/mildred/conductor.go/src/service"
)

// Note for services: a linked proxy config is set up when the service itself is
// enabled. The service depends on this proxy config

func StartOrRestart(restart bool) error {
	var prefix string = "start"
	var err error

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
	// if there is a deployment starting or started with the identical config,
	// use it and wait for it to be started, else:
	//
	//     find a free deployment Name
	//
	//     copy all the current service config over to the deployment, it should
	//     appear in the list
	//
	//     start the deployment, wait for started
	//
	// [ if restarting ] if failing to start, exit uncleanly
	// [ if starting ] if failing, stop all deployments and exit uncleanly
	//

	log.Printf("%s: Starting new deployment...\n", prefix)
	// TODO

	//
	// Stop all deployments that are of older config version
	//

	log.Printf("%s: Stopping obsolete deployments...\n", prefix)
	// TODO

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

		// TODO

	}
	return err
}

func Stop() error {
	//
	// Notify stop in progress
	//

	_, err := daemon.SdNotify(false, daemon.SdNotifyStopping)
	if err != nil {
		return err
	}

	//
	// Stop all deployments
	//

	log.Printf("stop: Stopping all deployments...\n")
	// TODO

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
