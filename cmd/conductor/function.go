package main

import (
	"io"
	"log"

	"github.com/integrii/flaggy"

	"github.com/mildred/conductor.go/src/deployment_public"
)

func cmd_function_caddy_config() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("caddy-config")
	cmd.Description = "Generates Caddy configuration"
	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		return deployment_public.FunctionCaddyConfig(deployment_public.FuncFunctionCaddyConfigOpts{})
	})
	return cmd
}

func cmd_function() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("function")
	cmd.ShortName = "f"
	cmd.Description = "Function commands"
	cmd.AttachSubcommand(cmd_function_caddy_config(), 1)
	cmd.RequireSubcommand = true
	return cmd
}
