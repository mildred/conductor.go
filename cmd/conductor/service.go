package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/deployment_util"
	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/service_public"
)

func cmd_service_start(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	return service_public.Start(flag.Arg(0))
}

func cmd_service_stop(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	return service_public.Stop(flag.Arg(0))
}

func cmd_service_restart(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	return service_public.Reload(flag.Arg(0))
}

func cmd_service_deploy(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	if flag.NArg() > 2 || flag.NArg() < 1 {
		return fmt.Errorf("Command %s must take a single service definition and a free deployment name as argument", strings.Join(name, " "))
	}

	service, err := service.LoadServiceByName(flag.Arg(0))
	if err != nil {
		return err
	}

	var depl_name string
	if flag.NArg() == 2 {
		depl_name = flag.Arg(1)
	}

	if depl_name == "" {
		depl, status, err := deployment_util.StartNewOrExistingFromService(context.Background(), service, 10)
		if err != nil {
			return err
		}

		fmt.Printf("Deployment (%s): %s\n", status, depl.DeploymentName)
		fmt.Printf("You can start it with: systemctl start %s\n", deployment.DeploymentUnit(depl.DeploymentName))
		return nil
	} else {
		dir, err := deployment_util.CreateDeploymentFromService(depl_name, service)
		if err != nil {
			return err
		}

		fmt.Printf("Deployment created in: %s\n", dir)
		fmt.Printf("You can start it with: systemctl start %s\n", deployment.DeploymentUnit(depl_name))
		return nil
	}
}

func cmd_service_inspect(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	log.Default().SetOutput(io.Discard)

	return service_public.PrintInspect(flag.Args()...)
}

func cmd_service_ls(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	unit := flag.Bool("unit", false, "Show systemd unit column")
	flag.Parse(args)

	log.Default().SetOutput(io.Discard)

	return service_public.PrintList(service_public.PrintListSettings{
		Unit: *unit,
	})
}

func cmd_service_show(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	log.Default().SetOutput(io.Discard)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	return service_public.PrintService(flag.Arg(0))
}

func cmd_service_unit(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	log.Default().SetOutput(io.Discard)

	for _, arg := range flag.Args() {
		unit, err := service.ServiceUnitByName(arg)
		if err != nil {
			return err
		}

		fmt.Println(unit)
	}
	return nil
}

func cmd_service_status(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	log.Default().SetOutput(io.Discard)

	var cli []string = []string{"status"}
	for _, arg := range flag.Args() {
		unit, err := service.ServiceUnitByName(arg)
		if err != nil {
			return err
		}

		cli = append(cli, unit)
	}

	cmd := exec.Command("systemctl", cli...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func cmd_service_env(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	log.Default().SetOutput(io.Discard)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	service, err := service.LoadServiceByName(flag.Arg(0))
	if err != nil {
		return err
	}

	for _, v := range service.Vars() {
		fmt.Printf("%s\n", v)
	}

	return nil
}

func cmd_service_config_get(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	log.Default().SetOutput(io.Discard)

	if flag.NArg() < 1 {
		return fmt.Errorf("Command %s must take a service definition as argument", strings.Join(name, " "))
	}

	service, err := service.LoadServiceByName(flag.Arg(0))
	if err != nil {
		return err
	}

	var failures []string

	for i := 1; i < flag.NArg(); i++ {
		arg := flag.Arg(i)
		if val, ok := service.Config[arg]; ok {
			fmt.Printf("%s\n", val)
		} else {
			failures = append(failures, arg)
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("could not get configuration for %v", failures)
	}

	return nil
}

func cmd_service_config_set(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	file_flag := flag.String("f", "", "Write in this file instead of the service file")
	no_restart_flag := flag.Bool("n", false, "Do not restart service")
	flag.Parse(args)

	log.Default().SetOutput(io.Discard)

	if flag.NArg() < 1 {
		return fmt.Errorf("Command %s must take a service definition as argument", strings.Join(name, " "))
	}

	service_descr := flag.Arg(0)
	service, err := service.LoadServiceByName(service_descr)
	if err != nil {
		return err
	}

	filename := *file_flag
	if filename == "" {
		filename = service.FileName
	}

	changed_args := map[string]string{}
	for i := 1; i < flag.NArg(); i++ {
		splits := strings.SplitN(flag.Arg(i), "=", 2)
		if len(splits) < 2 {
			continue
		}

		key, value := splits[0], splits[1]
		if service.Config[key] != value {
			changed_args[key] = value
		}
	}

	if len(changed_args) == 0 {
		return nil
	}

	//
	// Add config
	//

	err = service_public.ServiceSetConfig(filename, changed_args)
	if err != nil {
		return err
	}

	//
	// restart service
	//

	if !*no_restart_flag {
		return service_public.Reload(service_descr)
	}

	return nil
}

func cmd_service_config_ls(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	log.Default().SetOutput(io.Discard)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	service, err := service.LoadServiceByName(flag.Arg(0))
	if err != nil {
		return err
	}

	for k, v := range service.Config {
		fmt.Printf("%s=%s\n", k, v)
	}

	return nil
}

func cmd_service_config(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)

	return run_subcommand(name, args, flag, map[string]Subcommand{
		"ls":  {cmd_service_config_ls, "SERVICE", "List service configuration variables"},
		"get": {cmd_service_config_get, "SERVICE VAR...", "Get service configuration variable"},
		"set": {cmd_service_config_set, "SERVICE [VAR=VAL...]", "Set service configuration variable"},
	})
}

func cmd_service(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)

	return run_subcommand(name, args, flag, map[string]Subcommand{
		"start":   {cmd_service_start, "SERVICE", "Declare and start a service"},
		"stop":    {cmd_service_stop, "SERVICE", "Stop a service"},
		"restart": {cmd_service_restart, "SERVICE", "Restart a service"},
		"deploy":  {cmd_service_deploy, "SERVICE [DEPLOYMENT_NAME]", "Manually create a deployment, do not start it"},
		"inspect": {cmd_service_inspect, "", "Inspect a service in current directory or on the command-line"},
		"ls":      {cmd_service_ls, "", "List all services"},
		"show":    {cmd_service_show, "SERVICE", "Show service"},
		"status":  {cmd_service_status, "", "Status from systemctl"},
		"unit":    {cmd_service_unit, "", "Print systemd unit"},
		"config":  {cmd_service_config, "...", "Manage service configuration"},
		"env":     {cmd_service_env, "SERVICE", "Print service template environment variables"},
	})
}
