package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/blang/semver"
	"github.com/integrii/flaggy"
	"github.com/rhysd/go-github-selfupdate/selfupdate"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/deployment_public"
	"github.com/mildred/conductor.go/src/install"
	"github.com/mildred/conductor.go/src/policies"
	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/service_public"
)

var version = "dev"

func cmd_private_policy_server() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("policy-server")
	cmd.Description = "Run a policy server"

	cmd.CommandUsed = Hook(func() error {
		return policies.RunServer()
	})
	return cmd

}

func cmd_private() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("_")
	cmd.Description = "Internal commands"
	cmd.AttachSubcommand(cmd_private_service(), 1)
	cmd.AttachSubcommand(cmd_private_deployment(), 1)
	cmd.AttachSubcommand(cmd_private_policy_server(), 1)
	cmd.RequireSubcommand = true
	return cmd
}

func cmd_reload() *flaggy.Subcommand {
	var inclusive bool

	cmd := flaggy.NewSubcommand("reload")
	cmd.Description = "Reload and start services in well-known locations"
	cmd.AdditionalHelpPrepend = "\nReload and start services in well-known directories:"
	for _, dir := range service.ServiceDirs {
		cmd.AdditionalHelpPrepend += "\n  - " + dir
	}

	cmd.Bool(&inclusive, "", "inclusive", "Allow services from other directories (do not stop them)")

	cmd.CommandUsed = Hook(func() error {
		return service_public.ReloadServices(inclusive)
	})
	return cmd
}

func cmd_system_install() *flaggy.Subcommand {
	var destdir string

	cmd := flaggy.NewSubcommand("install")
	cmd.Description = "Install system services"
	cmd.String(&destdir, "", "destdir", "Directory root where to install")

	cmd.CommandUsed = Hook(func() error {
		return install.Install(destdir)
	})

	return cmd
}

func cmd_system_uninstall() *flaggy.Subcommand {
	var destdir string

	cmd := flaggy.NewSubcommand("uninstall")
	cmd.Description = "Uninstall system services"
	cmd.String(&destdir, "", "destdir", "Directory root where to uninstall")

	cmd.CommandUsed = Hook(func() error {
		return install.Uninstall(destdir)
	})

	return cmd
}

func cmd_system_upgrade() *flaggy.Subcommand {
	var check bool = false

	cmd := flaggy.NewSubcommand("upgrade")
	cmd.Description = "Upgrade to new version"
	cmd.Bool(&check, "c", "check", "Only check for new release")

	cmd.CommandUsed = Hook(func() error {
		if check || version == "dev" {
			rel, found, err := selfupdate.DetectLatest("mildred/conductor.go")
			if err != nil {
				return err
			}
			if found {
				log.Println("Latest version is", rel.Version)
			} else {
				log.Println("Latest release not found")
			}
			return nil
		}

		v := semver.MustParse(version)
		latest, err := selfupdate.UpdateSelf(v, "mildred/conductor.go")
		if err != nil {
			log.Println("Binary update failed:", err)
			return nil
		}
		if check || version == "dev" {
			log.Println("Latest version is", latest.Version)
		} else if latest.Version.Equals(v) {
			// latest version is the same as current version. It means current binary is up to date.
			log.Println("Current binary is the latest version", version)
		} else {
			log.Println("Successfully updated to version", latest.Version)
			log.Println("Release note:\n", latest.ReleaseNotes)

		}
		return nil
	})

	return cmd
}

func cmd_system() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("system")
	cmd.Description = "System management"
	cmd.RequireSubcommand = true
	cmd.AttachSubcommand(cmd_system_install(), 1)
	cmd.AttachSubcommand(cmd_system_uninstall(), 1)
	cmd.AttachSubcommand(cmd_system_upgrade(), 1)
	return cmd
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

func cmd_run() *flaggy.Subcommand {
	var d, s string
	var env envlist
	var direct bool
	var cmdname string
	var args []string

	cmd := flaggy.NewSubcommand("run")
	cmd.ShortName = "r"
	cmd.Description = "Run commands in a deployment"
	cmd.String(&d, "d", "deployment", "Specify deployment")
	cmd.String(&s, "s", "service", "Specify service")
	cmd.Var(&env, "e", "env", "Environment to add to the command")
	cmd.Bool(&direct, "", "direct", "If command fails, do not add error message and keep exit status")
	cmd.AddPositionalValue(&cmdname, "command", 1, false, "Command to run")
	cmd.AddExtraValues(&args, "args", "Command arguments")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		if d != "" && s == "" {
			depl, err := deployment.ReadDeploymentByName(d)
			if err != nil {
				return err
			}

			if cmdname == "" {
				return deployment_public.PrintListCommands(depl)
			} else {
				return deployment_public.RunDeploymentCommand(depl, direct, env, cmdname, args...)
			}
		} else if d == "" && s != "" {
			service, err := service.LoadServiceByName(s)
			if err != nil {
				return err
			}

			if cmdname == "" {
				return service_public.PrintListCommands(service)
			} else {
				return service_public.RunServiceCommand(service, direct, env, cmdname, args...)
			}
		}

		return fmt.Errorf("You must specify a deployment using the -d flag or a service using the -s flag")
	})

	return cmd
}

func Main(ctx context.Context) error {
	f := flaggy.NewParser(os.Args[0])
	f.Version = version
	f.AttachSubcommand(cmd_service(), 1)
	f.AttachSubcommand(cmd_deployment(), 1)
	f.AttachSubcommand(cmd_function(), 1)
	f.AttachSubcommand(cmd_run(), 1)
	f.AttachSubcommand(cmd_reload(), 1)
	f.AttachSubcommand(cmd_system(), 1)
	f.AttachSubcommand(cmd_private(), 1)
	f.RequireSubcommand = true
	err := f.Parse()
	if err != nil {
		return err
	}

	return nil
}

func main() {
	ctx := context.Background()
	err := Main(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
