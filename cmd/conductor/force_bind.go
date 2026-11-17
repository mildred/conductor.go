package main

import (
	"context"
	"log"

	"github.com/integrii/flaggy"

	"github.com/mildred/conductor.go/src/force_bind"
)

func cmd_force_bind(ctx context.Context) *flaggy.Subcommand {
	var opts force_bind.ForceBindOpts
	var permissive bool
	var cmdname string
	var args []string

	opts.Rules = &force_bind.Rules{}

	cmd := flaggy.NewSubcommand("force-bind")
	cmd.Bool(&permissive, "p", "permissive", "Allow all bind attempts outside of the rules")
	cmd.Var(opts.Rules, "b", "bind", "Bind rule in form of IP[/PREFIX][:PORT]=RULE, RULE can be ADDRESS[:PORT] or fd=N or sd=N")
	cmd.Description = "Force child process sockets to bind according to rules"
	cmd.NoFlagsAfterArgs = true
	cmd.AddPositionalValue(&cmdname, "command", 1, true, "Command to run")
	cmd.AddExtraValues(&args, "args", "Command arguments")

	cmd.CommandUsed = Hook(func() error {

		opts.PreventUnknownBind = !permissive

		err := force_bind.ForceBindCommand(ctx, log.Default(), append([]string{cmdname}, args...), opts)
		if err != nil {
			return err
		}

		return nil
	})
	return cmd
}
