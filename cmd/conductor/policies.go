package main

import (
	"io"
	"log"

	"github.com/integrii/flaggy"

	"github.com/mildred/conductor.go/src/policies"
)

func cmd_policy_list() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("list")
	cmd.ShortName = "ls"
	cmd.Description = "List policies in well-known locations"
	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		return policies.PrintList()
	})
	return cmd
}

func cmd_policy_create() *flaggy.Subcommand {
	var policy_dir string

	cmd := flaggy.NewSubcommand("create")
	cmd.Description = "Create a new policy by name or by path"
	cmd.AddPositionalValue(&policy_dir, "policy", 1, true, "The policy to create, name or path")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		return policies.CreateCommand(policy_dir)
	})
	return cmd
}

func cmd_policy_inspect() *flaggy.Subcommand {
	var policy_dir string

	cmd := flaggy.NewSubcommand("inspect")
	cmd.Description = "Inspect a policy"
	cmd.AddPositionalValue(&policy_dir, "policy", 1, true, "The policy to inspect, path or name")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		return policies.InspectCommand(policy_dir)
	})
	return cmd
}

func cmd_policy_show() *flaggy.Subcommand {
	var policy_dir string

	cmd := flaggy.NewSubcommand("show")
	cmd.Description = "Show policy"
	cmd.AddPositionalValue(&policy_dir, "policy", 1, true, "The policy to show, path or name")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		return policies.Print(policy_dir)
	})
	return cmd
}

func cmd_policy() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("policy")
	cmd.ShortName = "p"
	cmd.Description = "Policy commands"
	cmd.AttachSubcommand(cmd_policy_list(), 1)
	cmd.AttachSubcommand(cmd_policy_create(), 1)
	cmd.AttachSubcommand(cmd_policy_show(), 1)
	cmd.AttachSubcommand(cmd_policy_inspect(), 1)
	cmd.RequireSubcommand = true
	return cmd
}
