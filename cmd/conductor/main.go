package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

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
	flag.Parse(args)

	return service_internal.StartOrRestart(false)
}

func private_service_restart(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return service_internal.StartOrRestart(true)
}

func private_service_stop(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return service_internal.Stop()
}

func private_service_cleanup(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return service_internal.Cleanup()
}

func private_service_register(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return service_internal.CaddyRegister(true, ".")
}

func private_service_deregister(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return service_internal.CaddyRegister(false, ".")
}

func private_service(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)

	return run_subcommand(name, args, flag, map[string]Subcommand{
		"start":      {private_service_start, "", "Start a service"},
		"restart":    {private_service_restart, "", "Restart a service"},
		"stop":       {private_service_stop, "", "Stop a service"},
		"cleanup":    {private_service_cleanup, "", "Clean up service after it has stopped"},
		"register":   {private_service_register, "", "Register service to load balancer"},
		"deregister": {private_service_deregister, "", "Deregister service from load balancer"},
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

func cmd_service_inspect(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	original_paths := flag.Bool("original-paths", true, "Paths are relative to the service directory")
	flag.Parse(args)

	return service_public.PrintInspect(!*original_paths, flag.Args()...)
}

func cmd_service(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)

	return run_subcommand(name, args, flag, map[string]Subcommand{
		"start":   {cmd_service_start, "", "Declare and start a service"},
		"inspect": {cmd_service_inspect, "", "Inspect a service in current directory or on the command-line"},
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

func private_deployment(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)

	return run_subcommand(name, args, flag, map[string]Subcommand{
		"prepare":    {private_deployment_prepare, "", "Prepare a deployment before starting it"},
		"start":      {private_deployment_start, "", "Start a deployment"},
		"stop":       {private_deployment_stop, "", "Stop a deployment"},
		"cleanup":    {private_deployment_cleanup, "", "Clean up deployment after it has stopped"},
		"register":   {private_deployment_register, "", "Register deployment to load balancer"},
		"deregister": {private_deployment_deregister, "", "Deregister deployment from load balancer"},
	})
}

func cmd_deployment_ls(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment_public.PrintList()
}

func cmd_deployment_inspect(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	flag.Parse(args)

	return deployment_public.PrintInspect(flag.Args()...)
}

func cmd_deployment(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)

	return run_subcommand(name, args, flag, map[string]Subcommand{
		"ls":      {cmd_deployment_ls, "", "List all deployments"},
		"inspect": {cmd_deployment_inspect, "", "Inspect deployment in current directory or on the command-line"},
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
