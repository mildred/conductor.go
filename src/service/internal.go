package service

import (
	"encoding/json"
	"log"
	"path"

	"github.com/mildred/conductor.go/src/caddy"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/tmpl"
)

// Note for services: a linked proxy config is set up when the service itself is
// enabled. The service depends on this proxy config

func StartOrRestart() error {
	// [ if restarting ] notify systemd reload in progress
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
	// notify systemd ready
	//
	// stop all deployments that are of older config version
	//
	// keep running in the background and monitor that the deployments are still
	// running, else exit with an error code
	return nil
}

func Stop() error {
	// notify stop in progress
	//
	// stop all deployments
	return nil
}

func Cleanup() error {
	// remove temp files if there is any
	return nil
}

var LookupPaths []string = dirs.MultiJoin("services", append(append([]string{dirs.SelfRuntimeDir}, dirs.SelfConfigDirs...), dirs.SelfDataDirs...)...)

func Reload() error {
	// Loop through location and set up services
	return nil
}

func Declare(definition_path string) error {
	// Loop through location and set up services
	return nil
}

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
