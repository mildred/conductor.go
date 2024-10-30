package main

import (
	"github.com/integrii/flaggy"

	"github.com/mildred/conductor.go/src/deployment_internal"
)

func cmd_private_deployment_prepare() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("prepare")
	cmd.Description = "Prepare a deployment before starting it"

	cmd.CommandUsed = Hook(func() error {
		return deployment_internal.Prepare()
	})
	return cmd
}

func cmd_private_deployment_start() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("start")
	cmd.Description = "Start a deployment"

	cmd.CommandUsed = Hook(func() error {
		return deployment_internal.Start()
	})
	return cmd
}

func cmd_private_deployment_stop() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("stop")
	cmd.Description = "Stop a deployment"

	cmd.CommandUsed = Hook(func() error {
		return deployment_internal.Stop()
	})
	return cmd
}

func cmd_private_deployment_cleanup() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("cleanup")
	cmd.Description = "Clean up deployment after it has stopped"

	cmd.CommandUsed = Hook(func() error {
		return deployment_internal.Cleanup()
	})
	return cmd
}

func cmd_private_deployment_register() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("register")
	cmd.Description = "Register deployment to load balancer"

	cmd.CommandUsed = Hook(func() error {
		return deployment_internal.CaddyRegister(true, ".")
	})
	return cmd
}

func cmd_private_deployment_deregister() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("deregister")
	cmd.Description = "Deregister deployment from load balancer"

	cmd.CommandUsed = Hook(func() error {
		return deployment_internal.CaddyRegister(false, ".")
	})
	return cmd
}

func cmd_private_deployment_template() *flaggy.Subcommand {
	var template string

	cmd := flaggy.NewSubcommand("template")
	cmd.Description = "Run a template in the current deployment context"
	cmd.AddPositionalValue(&template, "template", 1, true, "The template file to run")

	cmd.CommandUsed = Hook(func() error {
		return deployment_internal.Template(".", template)
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
