package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/integrii/flaggy"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/deployment_public"
	"github.com/mildred/conductor.go/src/dirs"
)

func cmd_deployment_ls() *flaggy.Subcommand {
	var unit, terse bool

	cmd := flaggy.NewSubcommand("ls")
	cmd.Bool(&unit, "", "unit", "Show systemd units column")
	cmd.Bool(&terse, "", "terse", "Show minimum details")
	cmd.Description = "List all deployments"

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		return deployment_public.PrintList(deployment_public.PrintListSettings{
			Unit:         unit,
			ServiceUnit:  unit,
			ConfigStatus: !terse,
		})
	})
	return cmd
}

func cmd_deployment_rm() *flaggy.Subcommand {
	var ids []string

	cmd := flaggy.NewSubcommand("rm") // "[DEPLOYMENT...]",
	cmd.Description = "Remove a deployment"
	cmd.AddExtraValues(&ids, "deployment", "The deployment to use")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		for _, arg := range ids {
			err := deployment_public.Remove(arg)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return cmd
}

func cmd_deployment_inspect() *flaggy.Subcommand {
	var ids []string

	cmd := flaggy.NewSubcommand("inspect") // "[DEPLOYMENT...]",
	cmd.Description = "Inspect deployment in current directory or on the command-line"
	cmd.AddExtraValues(&ids, "deployment", "The deployment to use")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		return deployment_public.PrintInspect(ids...)
	})
	return cmd
}

var cmd_deployment_status = cmd_deployment_systemd("status", "Status from systemctl")
var cmd_deployment_start = cmd_deployment_systemd("start", "Start with systemctl")
var cmd_deployment_stop = cmd_deployment_systemd("stop", "Stop with systemctl")
var cmd_deployment_restart = cmd_deployment_systemd("restart", "Restart with systemctl")
var cmd_deployment_kill = cmd_deployment_systemd("kill", "Kill with systemctl")

func cmd_deployment_systemd(cmd_name, descr string) func() *flaggy.Subcommand {
	return func() *flaggy.Subcommand {
		var all bool
		var ids []string
		var signal string

		cmd := flaggy.NewSubcommand(cmd_name)
		cmd.AddExtraValues(&ids, "deployment", "The deployment to use")
		cmd.Description = descr

		switch cmd_name {
		case "kill":
			cmd.String(&signal, "", "signal", "Signal to send")
		case "status":
			cmd.Bool(&all, "a", "all", "All units")
		}

		cmd.CommandUsed = Hook(func() error {
			log.Default().SetOutput(io.Discard)

			if len(ids) == 0 {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				ids = append(ids, path.Base(cwd))
			}

			var cli []string = []string{dirs.SystemdModeFlag(), cmd_name}
			if cmd_name == "kill" && signal != "" {
				cli = append(cli, "--signal="+signal)
			}
			for _, id := range ids {
				cli = append(cli, deployment.DeploymentUnit(id))
				if all {
					cli = append(cli,
						deployment.DeploymentConfigUnit(id),
						deployment.CGIFunctionSocketUnit(id),
						deployment.CGIFunctionServiceUnit(id, "*"))
				}
			}

			fmt.Fprintf(os.Stderr, "+ systemctl %s\n", strings.Join(cli, " "))
			cmd := exec.Command("systemctl", cli...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			return cmd.Run()
		})
		return cmd
	}
}

func cmd_deployment_unit() *flaggy.Subcommand {
	var ids []string

	cmd := flaggy.NewSubcommand("unit")
	cmd.Description = "Print systemd unit"
	cmd.AddExtraValues(&ids, "deployment", "The deployment to use")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		if len(ids) == 0 {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			ids = append(ids, path.Base(cwd))
		}

		for _, id := range ids {
			fmt.Println(deployment.DeploymentUnit(id))
		}
		return nil
	})
	return cmd
}

func cmd_deployment_env() *flaggy.Subcommand {
	var depl_name string

	cmd := flaggy.NewSubcommand("env")
	cmd.Description = "Print templating environment variables"
	cmd.AddPositionalValue(&depl_name, "deployment", 1, true, "The deployment to use")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		depl, err := deployment.ReadDeploymentByName(depl_name, true)
		if err != nil {
			return err
		}

		for _, v := range depl.Vars() {
			fmt.Printf("%s\n", v)
		}

		return nil
	})
	return cmd
}

func cmd_deployment_show(name string) *flaggy.Subcommand {
	var depl_name string

	cmd := flaggy.NewSubcommand(name) // "SERVICE",
	cmd.Description = "Show deployment"
	cmd.AddPositionalValue(&depl_name, "deployment", 1, true, "The deployment to use")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		return deployment_public.Print(depl_name, deployment_public.PrintSettings{})
	})
	return cmd
}

func cmd_deployment() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("deployment")
	cmd.ShortName = "d"
	cmd.Description = "Deployment commands"
	cmd.AttachSubcommand(cmd_deployment_ls(), 1)
	cmd.AttachSubcommand(cmd_deployment_rm(), 1)
	cmd.AttachSubcommand(cmd_deployment_inspect(), 1)
	cmd.AttachSubcommand(cmd_deployment_status(), 1)
	cmd.AttachSubcommand(cmd_deployment_start(), 1)
	cmd.AttachSubcommand(cmd_deployment_stop(), 1)
	cmd.AttachSubcommand(cmd_deployment_restart(), 1)
	cmd.AttachSubcommand(cmd_deployment_kill(), 1)
	cmd.AttachSubcommand(cmd_deployment_unit(), 1)
	cmd.AttachSubcommand(cmd_deployment_env(), 1)
	cmd.AttachSubcommand(cmd_deployment_show("show"), 1)
	cmd.AttachSubcommand(cmd_deployment_show("print"), 1)
	cmd.RequireSubcommand = true
	return cmd
}
