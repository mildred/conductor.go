package main

import (
	"io"
	"log"

	"github.com/integrii/flaggy"

	"github.com/mildred/conductor.go/src/deployment_public"
)

func cmd_function_caddy_config() *flaggy.Subcommand {
	opts := deployment_public.FuncFunctionCaddyConfigOpts{}

	cmd := flaggy.NewSubcommand("caddy-config")
	cmd.Description = "Generates Caddy configuration"
	cmd.String(&opts.DeploymentName, "", "deployment", "Deployment name [CONDUCTOR_DEPLOYMENT]")
	cmd.String(&opts.SnippetId, "", "snippet-id", "Snippet id [generated from deployment name]")
	cmd.String(&opts.FunctionId, "", "function", "Function id [CONDUCTOR_FUNCTION_ID]")
	cmd.String(&opts.SocketPath, "", "socket", "Socket path [CONDUCTOR_FUNCTION_SOCKET]")
	cmd.StringSlice(&opts.Policies, "", "policies", "Policies (in the form \"policy_name/authorization\") [CONDUCTOR_FUNCTION_POLICIES split by spaces]")
	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		return deployment_public.FunctionCaddyConfig(opts)
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
