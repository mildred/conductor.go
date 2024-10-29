package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/deployment_public"
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

type envlist []string

// String is an implementation of the flag.Value interface
func (i *envlist) String() string {
	return fmt.Sprintf("%v", *i)
}

// Set is an implementation of the flag.Value interface
func (i *envlist) Set(value string) error {
	if strings.Contains(value, "=") {
		*i = append(*i, value)
	} else {
		*i = append(*i, value+"="+os.Getenv(value))
	}
	return nil
}

func cmd_run(usage func(), name []string, args []string) error {
	flag := new_flag_set(name, usage)
	var env envlist
	d := flag.String("d", "", "Specify deployment")
	s := flag.String("s", "", "Specify service")
	flag.Var(&env, "e", "Environment to add to the command")
	direct := flag.Bool("direct", false, "If command fails, do not add error message and keep exit status")
	flag.Parse(args)

	log.Default().SetOutput(io.Discard)

	if *d != "" && *s == "" {
		depl, err := deployment.ReadDeploymentByName(*d)
		if err != nil {
			return err
		}

		if flag.NArg() == 0 {
			return deployment_public.PrintListCommands(depl)
		} else {
			return deployment_public.RunDeploymentCommand(depl, *direct, env, flag.Arg(0), flag.Args()[1:]...)
		}
	} else if *d == "" && *s != "" {
		service, err := service.LoadServiceByName(*s)
		if err != nil {
			return err
		}

		if flag.NArg() == 0 {
			return service_public.PrintListCommands(service)
		} else {
			return service_public.RunServiceCommand(service, *direct, env, flag.Arg(0), flag.Args()[1:]...)
		}
	}

	return fmt.Errorf("You must specify a deployment using the -d flag or a service using the -s flag")
}

func Main() error {
	flag := new_flag_set(os.Args[0:1], nil)

	return run_subcommand(os.Args[0:1], os.Args[1:], flag, map[string]Subcommand{
		"reload":     {cmd_reload, "", "Reload and start services in well-known locations"},
		"system":     {cmd_system, "...", "System management"},
		"service":    {cmd_service, "...", "Service commands"},
		"deployment": {cmd_deployment, "...", "Deployment commands"},
		"function":   {cmd_function, "...", "Function commands"},
		"run":        {cmd_run, "...", "Run commands in a deployment"},
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
