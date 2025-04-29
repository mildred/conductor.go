package main

import (
	"io"
	"log"

	"github.com/integrii/flaggy"

	"github.com/mildred/conductor.go/src/peers"
)

func cmd_peer_list() *flaggy.Subcommand {
	var policy string = "peers"

	cmd := flaggy.NewSubcommand("list")
	cmd.ShortName = "ls"
	cmd.Description = "List registered peers"
	cmd.String(&policy, "", "policy", "Policy to use")
	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		return peers.PrintList(policy)
	})
	return cmd
}

func cmd_peer_invite() *flaggy.Subcommand {
	var policy string = "peers"

	cmd := flaggy.NewSubcommand("invite")
	cmd.Description = "Get secret key for invites"
	cmd.String(&policy, "", "policy", "Policy to use")
	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		return peers.PrintInviteToken(policy)
	})
	return cmd
}

/*
func cmd_peer_add() *flaggy.Subcommand {
	var policy string = "peers"
	var policy_dir string

	cmd := flaggy.NewSubcommand("add")
	cmd.Description = "Add a new peer"
	cmd.String(&policy, "", "policy", "Policy to use")
	cmd.AddPositionalValue(&policy_dir, "policy", 1, true, "The policy to create, name or path")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		return peers.CreateCommand(policy_dir)
	})
	return cmd
}

func cmd_peer_inspect() *flaggy.Subcommand {
	var policy string = "peers"
	var policy_dir string

	cmd := flaggy.NewSubcommand("inspect")
	cmd.Description = "Inspect a peer"
	cmd.String(&policy, "", "policy", "Policy to use")
	cmd.AddPositionalValue(&policy_dir, "policy", 1, true, "The policy to inspect, path or name")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		return peers.InspectCommand(policy_dir)
	})
	return cmd
}

func cmd_peer_show() *flaggy.Subcommand {
	var policy string = "peers"
	var policy_dir string

	cmd := flaggy.NewSubcommand("show")
	cmd.Description = "Show peer"
	cmd.String(&policy, "", "policy", "Policy to use")
	cmd.AddPositionalValue(&policy_dir, "policy", 1, true, "The policy to show, path or name")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		return peers.Print(policy_dir)
	})
	return cmd
}
*/

func cmd_peer() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("peer")
	cmd.ShortName = "pe"
	cmd.Description = "Peers command"
	cmd.AttachSubcommand(cmd_peer_list(), 1)
	cmd.AttachSubcommand(cmd_peer_invite(), 1)
	// cmd.AttachSubcommand(cmd_peer_add(), 1)
	// cmd.AttachSubcommand(cmd_peer_show(), 1)
	// cmd.AttachSubcommand(cmd_peer_inspect(), 1)
	cmd.RequireSubcommand = true
	return cmd
}
