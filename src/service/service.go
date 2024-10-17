package service

import (
	"github.com/mildred/conductor.go/src/dirs"
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
