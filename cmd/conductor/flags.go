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
	flag := flag.NewFlagSet(strings.Join(name, " "), flag.ContinueOnError)
	flag.Usage = func() {
		if usage != nil {
			usage()
		} else {
			fmt.Fprintf(flag.Output(), "Usage of %s:\nversion %s\n\n", name[0], version)
		}
		if flag.NFlag() > 0 {
			fmt.Fprintf(flag.Output(), "Options for %s:\n", strings.Join(name, " "))
			flag.PrintDefaults()
		}
	}
	return flag
}

func run_cubcommand(name []string, args0 []string, flag *flag.FlagSet, subcommands map[string]Subcommand) error {
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
		flag.Usage()
		return fmt.Errorf("Unknown command %s %s", strings.Join(name, " "), cmd)
	}
}
