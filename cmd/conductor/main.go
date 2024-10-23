package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mildred/conductor.go/src/install"
	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/service_public"
)

var version = "dev"

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

	return service_public.ReloadServices(*inclusive)
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
