package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/deployment_internal"
	"github.com/mildred/conductor.go/src/deployment_public"
	"github.com/mildred/conductor.go/src/install"
	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/service_internal"
	"github.com/mildred/conductor.go/src/service_public"
)

var version = "dev"

func private_service_start(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, func() {
		usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n"+
			"Start a service by using an already starting deployment that has the\n"+
			"correct configuration or by starting a new deployment. Once the\n"+
			"deployment is started, old deployments for the service are stopped.\n\n")
	})
	max_deployment_index := flag.Int("max-deployment-index", 10, "Service will fail to deploy if it cannot find a deployment number below this")
	flag.Parse(args)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	return service_internal.StartOrRestart(false, flag.Arg(0), *max_deployment_index)
}

func private_service_restart(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	max_deployment_index := flag.Int("max-deployment-index", 10, "Service will fail to deploy if it cannot find a deployment number below this")
	flag.Parse(args)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	return service_internal.StartOrRestart(true, flag.Arg(0), *max_deployment_index)
}

func private_service_stop(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	return service_internal.Stop(flag.Arg(0))
}

func private_service_cleanup(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	return service_internal.Cleanup(flag.Arg(0))
}

func private_service_register(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	return service_internal.CaddyRegister(true, flag.Arg(0))
}

func private_service_deregister(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a single service definition as argument", strings.Join(name, " "))
	}

	return service_internal.CaddyRegister(false, flag.Arg(0))
}

func private_service_template(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	if flag.NArg() != 2 {
		return fmt.Errorf("Command %s must take a service definition and a template", strings.Join(name, " "))
	}

	return service_internal.Template(flag.Arg(0), flag.Arg(1))
}

func private_service(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)

	return run_subcommand(name, args, flag, map[string]Subcommand{
		"start":      {private_service_start, "SERVICE", "Start a service"},
		"restart":    {private_service_restart, "SERVICE", "Restart a service"},
		"stop":       {private_service_stop, "SERVICE", "Stop a service"},
		"cleanup":    {private_service_cleanup, "SERVICE", "Clean up service after it has stopped"},
		"register":   {private_service_register, "SERVICE", "Register service to load balancer"},
		"deregister": {private_service_deregister, "SERVICE", "Deregister service from load balancer"},
		"template":   {private_service_template, "SERVICE TEMPLATE", "Run a template in the context of a service"},
	})
}

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

	return service_public.Restart(flag.Arg(0))
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

func cmd_service(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)

	return run_subcommand(name, args, flag, map[string]Subcommand{
		"start":   {cmd_service_start, "SERVICE", "Declare and start a service"},
		"stop":    {cmd_service_stop, "SERVICE", "Stop a service"},
		"restart": {cmd_service_restart, "SERVICE", "Restart a service"},
		"inspect": {cmd_service_inspect, "", "Inspect a service in current directory or on the command-line"},
		"ls":      {cmd_service_ls, "", "List all services"},
		"status":  {cmd_service_status, "", "Status from systemctl"},
		"unit":    {cmd_service_unit, "", "Print systemd unit"},
		"env":     {cmd_service_env, "SERVICE", "Print service template environment variables"},
	})
}

func private_deployment_prepare(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment_internal.Prepare()
}

func private_deployment_start(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment_internal.Start()
}

func private_deployment_stop(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment_internal.Stop()
}

func private_deployment_cleanup(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment_internal.Cleanup()
}

func private_deployment_register(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment_internal.CaddyRegister(true, ".")
}

func private_deployment_deregister(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment_internal.CaddyRegister(false, ".")
}

func private_deployment_template(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	if flag.NArg() != 1 {
		return fmt.Errorf("Command %s must take a template", strings.Join(name, " "))
	}

	return deployment_internal.Template(".", flag.Arg(0))
}

func private_deployment(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)

	return run_subcommand(name, args, flag, map[string]Subcommand{
		"prepare":    {private_deployment_prepare, "", "Prepare a deployment before starting it"},
		"start":      {private_deployment_start, "", "Start a deployment"},
		"stop":       {private_deployment_stop, "", "Stop a deployment"},
		"cleanup":    {private_deployment_cleanup, "", "Clean up deployment after it has stopped"},
		"register":   {private_deployment_register, "", "Register deployment to load balancer"},
		"deregister": {private_deployment_deregister, "", "Deregister deployment from load balancer"},
		"template":   {private_deployment_template, "TEMPLATE", "Run a template in the current deployment context"},
	})
}

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

func cmd_deployment_status(usage func(), name []string, args []string) error {
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

	var cli []string = []string{"status"}
	for _, id := range ids {
		cli = append(cli, deployment.DeploymentUnit(id))
	}

	cmd := exec.Command("systemctl", cli...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
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

	depl, err := deployment.LoadDeploymentDir(ids[0])
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
		"unit":    {cmd_deployment_unit, "[DEPLOYMENT...]", "Print systemd unit"},
		"env":     {cmd_deployment_env, "[DEPLOYMENT]", "Print templating environment variables"},
	})
}

func cmd_private(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)

	return run_subcommand(name, args, flag, map[string]Subcommand{
		"service":    {private_service, "...", "Manage conductor services"},
		"deployment": {private_deployment, "...", "Manage conductor deployments"},
	})
}

func cmd_reload(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, func() {
		usage()
		fmt.Fprintf(flag.CommandLine.Output(), "\n"+
			"Reload and start services in well-known directories:\n")
		for _, dir := range service.ServiceDirs {
			fmt.Fprintf(flag.CommandLine.Output(), "  - "+dir+"\n")
		}
		fmt.Fprintf(flag.CommandLine.Output(), "\n")
	})

	inclusive := flag.Bool("inclusive", false, "Allow services from other directories (do not stop them)")
	flag.Parse(args)

	return service_public.Reload(*inclusive)
}

func cmd_system_install(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	destdir := flag.String("destdir", "", "Directory root where to perform installation")
	flag.Parse(args)

	return install.Install(*destdir)
}

func cmd_system_uninstall(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	destdir := flag.String("destdir", "", "Directory root where to uninstall")
	flag.Parse(args)

	return install.Uninstall(*destdir)
}

func cmd_system(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)

	return run_subcommand(name, args, flag, map[string]Subcommand{
		"install":   {cmd_system_install, "", "Install system services"},
		"uninstall": {cmd_system_uninstall, "", "Uninstall system services"},
	})
}

func Main() error {
	flag := new_flag_set(os.Args[0:1], nil)

	return run_subcommand(os.Args[0:1], os.Args[1:], flag, map[string]Subcommand{
		"reload":     {cmd_reload, "", "Reload and start services in well-known locations"},
		"system":     {cmd_system, "...", "System management"},
		"service":    {cmd_service, "...", "Service commands"},
		"deployment": {cmd_deployment, "...", "Deployment commands"},
		"_":          {cmd_private, "...", "Internal commands"},
	})
}

func main() {
	err := Main()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
