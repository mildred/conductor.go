package main

import (
	"github.com/integrii/flaggy"

	"github.com/mildred/conductor.go/src/deployment_internal"
)

func cmd_private_deployment_prepare() *flaggy.Subcommand {
	var deployment_name string = "."

	cmd := flaggy.NewSubcommand("prepare")
	cmd.Description = "Prepare a deployment before starting it"
	cmd.AddPositionalValue(&deployment_name, "deployment", 1, false, "The deployment, default to the current directory deployment")

	cmd.CommandUsed = Hook(func() error {
		return deployment_internal.Prepare(deployment_name)
	})
	return cmd
}

func cmd_private_deployment_start() *flaggy.Subcommand {
	var deployment_name string = "."
	var function bool

	cmd := flaggy.NewSubcommand("start")
	cmd.Bool(&function, "", "function", "Start a function")
	cmd.Description = "Start a deployment"
	cmd.AddPositionalValue(&deployment_name, "deployment", 1, false, "The deployment, default to the current directory deployment")

	cmd.CommandUsed = Hook(func() error {
		return deployment_internal.Start(deployment_name, function)
	})
	return cmd
}

func cmd_private_deployment_stop() *flaggy.Subcommand {
	var deployment_name string = "."
	var function bool

	cmd := flaggy.NewSubcommand("stop")
	cmd.Bool(&function, "", "function", "Stop a function")
	cmd.Description = "Stop a deployment"
	cmd.AddPositionalValue(&deployment_name, "deployment", 1, false, "The deployment, default to the current directory deployment")

	cmd.CommandUsed = Hook(func() error {
		return deployment_internal.Stop(deployment_name, function)
	})
	return cmd
}

func cmd_private_deployment_cleanup() *flaggy.Subcommand {
	var deployment_name string = "."

	cmd := flaggy.NewSubcommand("cleanup")
	cmd.Description = "Clean up deployment after it has stopped"
	cmd.AddPositionalValue(&deployment_name, "deployment", 1, false, "The deployment, default to the current directory deployment")

	cmd.CommandUsed = Hook(func() error {
		return deployment_internal.Cleanup(deployment_name)
	})
	return cmd
}

func cmd_private_deployment_register() *flaggy.Subcommand {
	var deployment_name string = "."

	cmd := flaggy.NewSubcommand("register")
	cmd.Description = "Register deployment to load balancer"
	cmd.AddPositionalValue(&deployment_name, "deployment", 1, false, "The deployment, default to the current directory deployment")

	cmd.CommandUsed = Hook(func() error {
		return deployment_internal.CaddyRegister(true, deployment_name)
	})
	return cmd
}

func cmd_private_deployment_deregister() *flaggy.Subcommand {
	var deployment_name string = "."

	cmd := flaggy.NewSubcommand("deregister")
	cmd.Description = "Deregister deployment from load balancer"
	cmd.AddPositionalValue(&deployment_name, "deployment", 1, false, "The deployment, default to the current directory deployment")

	cmd.CommandUsed = Hook(func() error {
		return deployment_internal.CaddyRegister(false, deployment_name)
	})
	return cmd
}

func cmd_private_deployment_template() *flaggy.Subcommand {
	var deployment_name string = "."
	var template string

	cmd := flaggy.NewSubcommand("template")
	cmd.Description = "Run a template in the current deployment context"
	cmd.AddPositionalValue(&deployment_name, "deployment", 1, true, "The deployment, default to the current directory deployment")
	cmd.AddPositionalValue(&template, "template", 2, false, "The template file to run")

	cmd.CommandUsed = Hook(func() error {
		if template == "" && deployment_name != "." {
			template = deployment_name
			deployment_name = "."
		}

		return deployment_internal.Template(deployment_name, template)
	})
	return cmd
}

func cmd_private_deployment() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("deployment")
	cmd.Description = "Manage conductor deployments"
	cmd.AttachSubcommand(cmd_private_deployment_prepare(), 1)
	cmd.AttachSubcommand(cmd_private_deployment_start(), 1)
	cmd.AttachSubcommand(cmd_private_deployment_stop(), 1)
	cmd.AttachSubcommand(cmd_private_deployment_cleanup(), 1)
	cmd.AttachSubcommand(cmd_private_deployment_register(), 1)
	cmd.AttachSubcommand(cmd_private_deployment_deregister(), 1)
	cmd.AttachSubcommand(cmd_private_deployment_template(), 1)
	cmd.RequireSubcommand = true
	return cmd
}
