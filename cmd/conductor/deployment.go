package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/deployment_public"
)

func cmd_deployment_ls(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	unit := flag.Bool("unit", false, "Show systemd units column")
	flag.Parse(args)

	log.Default().SetOutput(io.Discard)

	return deployment_public.PrintList(deployment_public.PrintListSettings{
		Unit:        *unit,
		ServiceUnit: *unit,
	})
}

func cmd_deployment_rm(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	log.Default().SetOutput(io.Discard)

	for _, arg := range flag.Args() {
		err := deployment_public.Remove(arg)
		if err != nil {
			return err
		}
	}
	return nil
}

func cmd_deployment_inspect(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	log.Default().SetOutput(io.Discard)

	return deployment_public.PrintInspect(flag.Args()...)
}

func cmd_deployment_unit(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	log.Default().SetOutput(io.Discard)

	ids := flag.Args()
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
}

var cmd_deployment_status = cmd_deployment_systemd("status")
var cmd_deployment_start = cmd_deployment_systemd("start")
var cmd_deployment_stop = cmd_deployment_systemd("stop")
var cmd_deployment_restart = cmd_deployment_systemd("restart")
var cmd_deployment_kill = cmd_deployment_systemd("kill")

func cmd_deployment_systemd(cmd_name string) func(usage func(), name []string, args []string) error {
	return func(usage func(), name []string, args []string) error {
		flag := new_flag_set(name, usage)
		flag.Parse(args)

		log.Default().SetOutput(io.Discard)

		ids := flag.Args()
		if len(ids) == 0 {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			ids = append(ids, path.Base(cwd))
		}

		var cli []string = []string{cmd_name}
		for _, id := range ids {
			cli = append(cli, deployment.DeploymentUnit(id))
		}

		fmt.Fprintf(os.Stderr, "+ systemctl %s\n", strings.Join(cli, " "))
		cmd := exec.Command("systemctl", cli...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		return cmd.Run()
	}
}

func cmd_deployment_env(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	log.Default().SetOutput(io.Discard)

	ids := flag.Args()
	if len(ids) == 0 {
		ids = append(ids, ".")
	}
	if len(ids) != 1 {
		return fmt.Errorf("Command %s must take a single deployment", strings.Join(name, " "))
	}

	depl, err := deployment.ReadDeploymentByName(ids[0])
	if err != nil {
		return err
	}

	for _, v := range depl.Vars() {
		fmt.Printf("%s\n", v)
	}

	return nil
}

func cmd_deployment(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)

	return run_subcommand(name, args, flag, map[string]Subcommand{
		"ls":      {cmd_deployment_ls, "", "List all deployments"},
		"rm":      {cmd_deployment_rm, "[DEPLOYMENT...]", "Remove a deployment"},
		"inspect": {cmd_deployment_inspect, "[DEPLOYMENT...]", "Inspect deployment in current directory or on the command-line"},
		"status":  {cmd_deployment_status, "[DEPLOYMENT...]", "Status from systemctl"},
		"start":   {cmd_deployment_start, "[DEPLOYMENT...]", "Start with systemctl"},
		"stop":    {cmd_deployment_stop, "[DEPLOYMENT...]", "Stop with systemctl"},
		"restart": {cmd_deployment_restart, "[DEPLOYMENT...]", "Restart with systemctl"},
		"kill":    {cmd_deployment_kill, "[DEPLOYMENT...]", "Kill with systemctl"},
		"unit":    {cmd_deployment_unit, "[DEPLOYMENT...]", "Print systemd unit"},
		"env":     {cmd_deployment_env, "[DEPLOYMENT]", "Print templating environment variables"},
	})
}
