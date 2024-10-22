package main

import (
	"fmt"
	"strings"

	"github.com/mildred/conductor.go/src/deployment_internal"
)

func private_deployment_prepare(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment_internal.Prepare()
}

func private_deployment_start(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment_internal.Start()
}

func private_deployment_stop(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment_internal.Stop()
}

func private_deployment_cleanup(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment_internal.Cleanup()
}

func private_deployment_register(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment_internal.CaddyRegister(true, ".")
}

func private_deployment_deregister(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment_internal.CaddyRegister(false, ".")
}

func private_deployment_template(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a template", strings.Join(name, " "))
	}

	return deployment_internal.Template(".", flag.Arg(0))
}

func private_deployment(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)

	return run_subcommand(name, args, flag, map[string]Subcommand{
		"prepare":    {private_deployment_prepare, "", "Prepare a deployment before starting it"},
		"start":      {private_deployment_start, "", "Start a deployment"},
		"stop":       {private_deployment_stop, "", "Stop a deployment"},
		"cleanup":    {private_deployment_cleanup, "", "Clean up deployment after it has stopped"},
		"register":   {private_deployment_register, "", "Register deployment to load balancer"},
		"deregister": {private_deployment_deregister, "", "Deregister deployment from load balancer"},
		"template":   {private_deployment_template, "TEMPLATE", "Run a template in the current deployment context"},
	})
}
