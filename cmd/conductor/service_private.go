package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mildred/conductor.go/src/service_internal"
)

func private_service_start(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, func() {
		usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n"+
			"Start a service by using an already starting deployment that has the\n"+
			"correct configuration or by starting a new deployment. Once the\n"+
			"deployment is started, old deployments for the service are stopped.\n\n")
	})
	max_deployment_index := flag.Int("max-deployment-index", 10, "Service will fail to deploy if it cannot find a deployment number below this")
	flag.Parse(args)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	return service_internal.StartOrRestart(false, flag.Arg(0), *max_deployment_index)
}

func private_service_restart(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	max_deployment_index := flag.Int("max-deployment-index", 10, "Service will fail to deploy if it cannot find a deployment number below this")
	flag.Parse(args)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	return service_internal.StartOrRestart(true, flag.Arg(0), *max_deployment_index)
}

func private_service_stop(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	return service_internal.Stop(flag.Arg(0))
}

func private_service_cleanup(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	return service_internal.Cleanup(flag.Arg(0))
}

func private_service_register(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	return service_internal.CaddyRegister(true, flag.Arg(0))
}

func private_service_deregister(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	return service_internal.CaddyRegister(false, flag.Arg(0))
}

func private_service_template(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	if flag.NArg() != 2 {
		return fmt.Errorf("Command %s must take a service definition and a template", strings.Join(name, " "))
	}

	return service_internal.Template(flag.Arg(0), flag.Arg(1))
}

func private_service(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)

	return run_subcommand(name, args, flag, map[string]Subcommand{
		"start":      {private_service_start, "SERVICE", "Start a service"},
		"restart":    {private_service_restart, "SERVICE", "Restart a service"},
		"stop":       {private_service_stop, "SERVICE", "Stop a service"},
		"cleanup":    {private_service_cleanup, "SERVICE", "Clean up service after it has stopped"},
		"register":   {private_service_register, "SERVICE", "Register service to load balancer"},
		"deregister": {private_service_deregister, "SERVICE", "Deregister service from load balancer"},
		"template":   {private_service_template, "SERVICE TEMPLATE", "Run a template in the context of a service"},
	})
}
