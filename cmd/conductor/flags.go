package main

import (
	"flag"
	"fmt"
	"strings"
)

type Subcommand = struct {
	F           func(usage func(), name []string, args []string) error
	Args        string
	Description string
}

func new_flag_set(name []string, usage func()) *flag.FlagSet {
	fl := flag.NewFlagSet(strings.Join(name, " "), flag.ContinueOnError)
	fl.Usage = func() {
		if usage != nil {
			usage()
		} else {
			fmt.Fprintf(fl.Output(), "Usage of %s:\nversion %s\n\n", name[0], version)
		}
		has_flags := false
		fl.VisitAll(func(_ *flag.Flag) { has_flags = true })
		if has_flags {
			fmt.Fprintf(fl.Output(), "Options for %s:\n", strings.Join(name, " "))
			fl.PrintDefaults()
		}
	}
	return fl
}

func run_subcommand(name []string, args0 []string, flag *flag.FlagSet, subcommands map[string]Subcommand) error {
	var cmd string
	var next_args []string

	usage := flag.Usage
	flag.Usage = func() {
		usage()
		fmt.Fprintf(flag.Output(), "\nCommands:\n")
		for cmdname, subcmd := range subcommands {
			fmt.Fprintf(flag.Output(), "  %s %s %s\n    \t%s\n", strings.Join(name, " "), cmdname, subcmd.Args, subcmd.Description)
		}
		fmt.Fprintf(flag.Output(), "\n")
	}

	flag.Parse(args0)
	args := flag.Args()
	if len(args) == 0 {
		cmd, next_args = "", args0
	} else {
		cmd, next_args = args[0], args[1:]
	}

	if subcommand, ok := subcommands[cmd]; ok {
		return subcommand.F(usage, append(name, cmd), next_args)
	} else {
		// Try with a prefix
		var subcommand Subcommand
		var nmatch = 0
		for cmdname, subcmd := range subcommands {
			if strings.HasPrefix(cmdname, cmd) {
				subcommand = subcmd
				nmatch = nmatch + 1
			}
		}
		if nmatch == 1 {
			return subcommand.F(usage, append(name, cmd), next_args)
		} else {
			flag.Usage()
			return fmt.Errorf("Unknown command %s %s", strings.Join(name, " "), cmd)
		}
	}
}
