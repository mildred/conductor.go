package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/service"
)

var version = "dev"

func service_start(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, func() {
		usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n"+
			"Start a service by using an already starting deployment that has the\n"+
			"correct configuration or by starting a new deployment. Once the\n"+
			"deployment is started, old deployments for the service are stopped.\n\n")
	})
	flag.Parse(args)

	return service.StartOrRestart()
}

func service_restart(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return service.StartOrRestart()
}

func service_stop(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return service.Stop()
}

func service_cleanup(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return service.Cleanup()
}

func private_service(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)

	return run_cubcommand(name, args, flag, map[string]Subcommand{
		"start":   {service_start, "", "Start a service"},
		"restart": {service_restart, "", "Restart a service"},
		"stop":    {service_stop, "", "Stop a service"},
		"cleanup": {service_cleanup, "", "Clean up service after it has stopped"},
	})
}

func deployment_prepare(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment.Prepare()
}

func deployment_start(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment.Start()
}

func deployment_stop(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment.Stop()
}

func deployment_cleanup(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment.Cleanup()
}

func private_deployment(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)

	return run_cubcommand(name, args, flag, map[string]Subcommand{
		"prepare": {deployment_prepare, "", "Prepare a deployment before starting it"},
		"start":   {deployment_start, "", "Start a deployment"},
		"stop":    {deployment_stop, "", "Stop a deployment"},
		"cleanup": {deployment_cleanup, "", "Clean up deployment after it has stopped"},
	})
}

func cmd_private(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)

	return run_cubcommand(name, args, flag, map[string]Subcommand{
		"service":    {private_service, "...", "Manage conductor services"},
		"deployment": {private_deployment, "...", "Manage conductor deployments"},
	})
}

func cmd_reload(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return service.Reload()
}

func Main() error {
	flag := new_flag_set(os.Args[0:1], nil)

	return run_cubcommand(os.Args[0:1], os.Args[1:], flag, map[string]Subcommand{
		"reload": {cmd_reload, "", "Reload and start services in well-known locations"},
		"_":      {cmd_private, "...", "Internal commands"},
	})
}

func main() {
	err := Main()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
