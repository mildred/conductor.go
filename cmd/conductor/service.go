package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/integrii/flaggy"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/deployment_util"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/service_internal"
	"github.com/mildred/conductor.go/src/service_public"
)

type strlist []string

// String is an implementation of the flag.Value interface
func (l *strlist) String() string {
	return fmt.Sprintf("%v", *l)
}

// Set is an implementation of the flag.Value interface
func (l *strlist) Set(value string) error {
	*l = append(*l, value)
	return nil
}

type configlist map[string][]service_public.Selector

// String is an implementation of the flag.Value interface
func (c *configlist) String() string {
	return ""
}

// Set is an implementation of the flag.Value interface
func (c *configlist) Set(value string) error {
	if !strings.Contains(value, "=") {
		return fmt.Errorf("configuration filter must contain a key and value separated by '='")
	}
	splits := strings.SplitN(value, "=", 2)
	if len(splits) != 2 || len(splits[0]) < 1 {
		return fmt.Errorf("invalid configuration filter")
	}
	key := splits[0]
	selector := service_public.Selector{
		Selector: "=",
		Value:    splits[1],
	}
	if key = strings.TrimSuffix(key, "="); key != splits[0] {
		// ensure "==" does not match any fancy selector
		selector.Selector = "="
	} else if key = strings.TrimSuffix(key, "!"); key != splits[0] {
		selector.Selector = "="
		selector.Negate = true
	} else if key = strings.TrimSuffix(key, "*"); key != splits[0] {
		selector.Selector = "*="
	} else if key = strings.TrimSuffix(key, "~json"); key != splits[0] {
		selector.Selector = "~json="
	} else if key = strings.TrimSuffix(key, "~jsonpath"); key != splits[0] {
		selector.Selector = "~jsonpath="
	} else if key = strings.TrimSuffix(key, "~"); key != splits[0] {
		selector.Selector = "~="
	} else if key = strings.TrimSuffix(key, "^"); key != splits[0] {
		selector.Selector = "^="
	} else if key = strings.TrimSuffix(key, "$"); key != splits[0] {
		selector.Selector = "$="
	}

	if selector.Selector != "=" {
		// do not allow "!≃="
		key0 := key
		if key = strings.TrimSuffix(key, "!"); key != key0 {
			selector.Negate = true
		}
	}

	(*c)[key] = append((*c)[key], selector)
	return nil
}

func cmd_service_enable() *flaggy.Subcommand {
	var service string
	var now bool

	cmd := flaggy.NewSubcommand("enable") // "SERVICE",
	cmd.Description = "Declare and enable a service"
	cmd.Bool(&now, "", "now", "start the service")
	cmd.AddPositionalValue(&service, "service", 1, true, "The service to act on")

	cmd.CommandUsed = Hook(func() error {
		return service_public.Enable(service, now)
	})
	return cmd
}

func cmd_service_disable() *flaggy.Subcommand {
	var service string
	var now bool

	cmd := flaggy.NewSubcommand("disable") // "SERVICE",
	cmd.Description = "Disable a service"
	cmd.Bool(&now, "", "now", "stop the service")
	cmd.AddPositionalValue(&service, "service", 1, true, "The service to act on")

	cmd.CommandUsed = Hook(func() error {
		return service_public.Disable(service, now)
	})
	return cmd
}

func cmd_service_start() *flaggy.Subcommand {
	var service string

	cmd := flaggy.NewSubcommand("start") // "SERVICE",
	cmd.Description = "Declare and start a service"
	cmd.AddPositionalValue(&service, "service", 1, true, "The service to act on")

	cmd.CommandUsed = Hook(func() error {
		return service_public.Start(service)
	})
	return cmd
}

func cmd_service_stop() *flaggy.Subcommand {
	var service string
	var no_block bool
	var force bool

	cmd := flaggy.NewSubcommand("stop") // "SERVICE",
	cmd.Description = "Stop a service"
	cmd.Bool(&no_block, "", "no-block", "Do not block while restarting")
	cmd.Bool(&force, "", "force", "Do not block and remove all deployments")
	cmd.AddPositionalValue(&service, "service", 1, true, "The service to act on")

	cmd.CommandUsed = Hook(func() error {
		return service_public.Stop(service, service_public.StopOpts{
			NoBlock:              no_block || force,
			RemoveAllDeployments: force,
		})
	})
	return cmd
}

func cmd_service_reload() *flaggy.Subcommand {
	var service string
	var no_block bool

	cmd := flaggy.NewSubcommand("reload") // "SERVICE",
	cmd.Description = "Reload a service"
	cmd.Bool(&no_block, "", "no-block", "Do not block while restarting")
	cmd.AddPositionalValue(&service, "service", 1, true, "The service to act on")

	cmd.CommandUsed = Hook(func() error {
		return service_public.Reload(service, service_public.ReloadOpts{
			NoBlock: no_block,
		})
	})
	return cmd
}

func cmd_service_rolling_restart() *flaggy.Subcommand {
	var service string
	var max_index int = 10
	var stop_timeout, term_timeout time.Duration = 0, 5 * time.Second

	cmd := flaggy.NewSubcommand("rolling-restart") // "SERVICE",
	cmd.Description = "Restart a service by starting the new deployment before terminating the old one"
	cmd.Int(&max_index, "", "max-deployment-index", "max deployment index to use before erroring out")
	cmd.Duration(&stop_timeout, "", "stop-timeout", "max duration to wait for the stop to complete (0 to disable timeout)")
	cmd.Duration(&term_timeout, "", "term-timeout", "max duration to wait for the SIGTERM to kill (0 to disable timeout)")
	cmd.AddPositionalValue(&service, "service", 1, true, "The service to act on")

	cmd.CommandUsed = Hook(func() error {
		return service_internal.StartOrReload(service, service_internal.StartOrReloadOpts{
			Restart:            true,
			WantsFresh:         true,
			MaxDeploymentIndex: max_index,
			StopTimeout:        stop_timeout,
			TermTimeout:        term_timeout,
		})
	})
	return cmd
}

func cmd_service_restart() *flaggy.Subcommand {
	var service string
	var no_block bool

	cmd := flaggy.NewSubcommand("restart") // "SERVICE",
	cmd.Description = "Restart a service"
	cmd.Bool(&no_block, "", "no-block", "Do not block while restarting")
	cmd.AddPositionalValue(&service, "service", 1, true, "The service to act on")

	cmd.CommandUsed = Hook(func() error {
		return service_public.Restart(service, service_public.RestartOpts{
			NoBlock: no_block,
		})
	})
	return cmd
}

func cmd_service_deploy() *flaggy.Subcommand {
	var service_def, depl_name, part string
	var start, fresh bool
	var max_index int = 10

	cmd := flaggy.NewSubcommand("deploy") // "SERVICE [DEPLOYMENT_NAME]",
	cmd.Description = "Manually create a deployment, do not start it"
	cmd.String(&part, "", "part", "Service part to deploy")
	cmd.Bool(&start, "", "start", "Start deployment")
	cmd.Bool(&fresh, "", "fresh", "Do not reuse a started deployment is possible")
	cmd.Int(&max_index, "", "max-deployment-index", "max deployment index to use before erroring out")
	cmd.AddPositionalValue(&service_def, "service", 1, true, "The service to act on")
	cmd.AddPositionalValue(&depl_name, "deployment-name", 2, false, "A deployment name")

	cmd.CommandUsed = Hook(func() error {
		service, err := service.LoadServiceByName(service_def)
		if err != nil {
			return err
		}

		seed, err := deployment.SeedFromService(service, part)
		if err != nil {
			return err
		}

		if depl_name == "" {
			ctx := context.Background()
			depl, status, err := deployment_util.StartNewOrExistingFromService(ctx, service, seed, deployment_util.StartNewOrExistingOpts{
				MaxIndex:  max_index,
				WantFresh: fresh,
			})
			if err != nil {
				return err
			}

			fmt.Printf("Deployment (%s): %s\n", status, depl.DeploymentName)
			depl_name = depl.DeploymentName
		} else {
			dir, err := deployment_util.CreateDeploymentFromService(depl_name, service, seed)
			if err != nil {
				return err
			}

			fmt.Printf("Deployment created in: %s\n", dir)
		}

		if start {
			fmt.Fprintf(os.Stderr, "+ systemctl %s start %s\n", dirs.SystemdModeFlag(), deployment.DeploymentUnit(depl_name))
			cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "start", deployment.DeploymentUnit(depl_name))
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err := cmd.Run()
			if err != nil {
				return err
			}
		} else {
			fmt.Printf("You can start it with: systemctl start %s\n", deployment.DeploymentUnit(depl_name))
		}
		return nil
	})
	return cmd
}

func cmd_service_inspect() *flaggy.Subcommand {
	var args []string

	cmd := flaggy.NewSubcommand("inspect")
	cmd.Description = "Inspect a service in current directory or on the command-line"
	cmd.AddExtraValues(&args, "service", "The service to act on")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		return service_public.PrintInspect(args...)
	})
	return cmd
}

func cmd_service_ls() *flaggy.Subcommand {
	var c = configlist{}
	var filter_jsonpaths = strlist{}
	var app_flag string
	var json_flag bool
	var jsons_flag bool
	var unit bool
	var csv bool
	var all bool
	var csv_sep string = ","
	var stop_b string
	var stop_a string
	var resume_b string
	var resume_a string
	var jsonpath string

	cmd := flaggy.NewSubcommand("ls")
	cmd.Description = "List all services"
	cmd.String(&app_flag, "", "app", "Filter by app name")
	cmd.Bool(&json_flag, "", "json", "Return a JSON array")
	cmd.Bool(&jsons_flag, "", "jsons", "Return a list of JSON objects")
	cmd.Bool(&unit, "u", "unit", "Show systemd unit column (do not use shorthand -u in scripts)")
	cmd.Var(&c, "c", "config", "Filter by configuration, same key multiple times is an OR, allowed selectors: '=', '~=', '~json=', '*=', '^=', '$='")
	cmd.Var(&filter_jsonpaths, "", "filter-jsonpath", "Filter by JSONPath returning boolean, multiple filteres are ORed")
	cmd.Bool(&csv, "", "csv", "Print as CSV")
	cmd.Bool(&all, "", "all", "Include disabled services that does not match the conditions")
	cmd.String(&csv_sep, "", "csv-sep", "CSV separator")
	cmd.String(&stop_b, "", "stop-before", "Stop list before this item as specified by JSONPath returning boolean")
	cmd.String(&stop_a, "", "stop-after", "Stop list after this item as specified by JSONPath returning boolean")
	cmd.String(&resume_b, "", "resume-before", "Resume list before this item as specified by JSONPath returning boolean")
	cmd.String(&resume_a, "", "resume-after", "Resume list after this item as specified by JSONPath returning boolean")
	cmd.String(&jsonpath, "", "jsonpath", "Evaluate this JSONPath for each row")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		if csv_sep == "\\t" {
			csv_sep = "\t"
		}

		var resume_before, resume_after, stop_before, stop_after *service_public.Selector
		if resume_b != "" {
			resume_before = &service_public.Selector{Selector: "jsonpath", Value: resume_b}
		}
		if resume_a != "" {
			resume_after = &service_public.Selector{Selector: "jsonpath", Value: resume_a}
		}
		if stop_b != "" {
			stop_before = &service_public.Selector{Selector: "jsonpath", Value: stop_b}
		}
		if stop_a != "" {
			stop_after = &service_public.Selector{Selector: "jsonpath", Value: stop_a}
		}

		return service_public.PrintList(service_public.PrintListSettings{
			Unit:              unit,
			FilterApplication: app_flag,
			FilterConfig:      c,
			FilterJSONPaths:   filter_jsonpaths,
			JSON:              json_flag,
			JSONs:             jsons_flag,
			CSV:               csv,
			CSVSeparator:      csv_sep,
			All:               all,
			ResumeBefore:      resume_before,
			ResumeAfter:       resume_after,
			StopBefore:        stop_before,
			StopAfter:         stop_after,
			JSONPath:          jsonpath,
		})
	})
	return cmd
}

func cmd_service_show(name string) *flaggy.Subcommand {
	var service string

	cmd := flaggy.NewSubcommand(name) // "SERVICE",
	cmd.Description = "Show service"
	cmd.AddPositionalValue(&service, "service", 1, true, "The service to act on")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		return service_public.PrintService(service, service_public.PrintSettings{})
	})
	return cmd
}

func cmd_service_status() *flaggy.Subcommand {
	var args []string
	var all bool

	cmd := flaggy.NewSubcommand("status")
	cmd.Description = "Status from systemctl"
	cmd.Bool(&all, "a", "all", "All units")
	cmd.AddExtraValues(&args, "service", "The service to act on")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		var cli []string = []string{dirs.SystemdModeFlag(), "status"}
		for _, arg := range args {
			service_dir, err := service.ServiceDirByName(arg)
			if err != nil {
				return err
			}
			cli = append(cli, service.ServiceUnit(service_dir))
			if all {
				cli = append(cli, service.ServiceConfigUnit(service_dir))
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

func cmd_service_unit() *flaggy.Subcommand {
	var services []string

	cmd := flaggy.NewSubcommand("unit")
	cmd.Description = "Print systemd unit"
	cmd.AddExtraValues(&services, "SERVICE", "The service to act on")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		for _, arg := range services {
			unit, err := service.ServiceUnitByName(arg)
			if err != nil {
				return err
			}

			fmt.Println(unit)
		}
		return nil
	})
	return cmd
}

func cmd_service_config_ls() *flaggy.Subcommand {
	var service_descr string

	cmd := flaggy.NewSubcommand("ls")
	cmd.Description = "List service configuration variables"
	cmd.AddPositionalValue(&service_descr, "service", 1, true, "The service to act on")

	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		service, err := service.LoadServiceByName(service_descr)
		if err != nil {
			return err
		}

		for k, v := range service.Config {
			fmt.Printf("%s=%s\n", k, v)
		}

		return nil
	})
	return cmd
}

func cmd_service_config_get() *flaggy.Subcommand {
	var service_descr string
	var vars []string

	cmd := flaggy.NewSubcommand("get") // "SERVICE VAR...",
	cmd.Description = "Get service configuration variable"
	cmd.AddPositionalValue(&service_descr, "service", 1, true, "The service to act on")
	cmd.AddExtraValues(&vars, "VAR", "Variables to get")

	cmd.CommandUsed = Hook(func() error {
		service, err := service.LoadServiceByName(service_descr)
		if err != nil {
			return err
		}

		var failures []string

		for i := 0; i < len(vars); i++ {
			arg := vars[i]
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
	})
	return cmd
}

func cmd_service_config_set() *flaggy.Subcommand {
	var service_descr, file_flag string
	var no_reload_flag, no_block_flag bool
	var vars []string

	cmd := flaggy.NewSubcommand("set") // "SERVICE [VAR=VAL...]",
	cmd.Description = "Set service configuration variable"
	cmd.String(&file_flag, "f", "file", "Write in this file instead of the service file")
	cmd.Bool(&no_reload_flag, "n", "no-reload", "Do not reload service")
	cmd.Bool(&no_block_flag, "", "no-block", "Do not block while reloading")
	cmd.AddPositionalValue(&service_descr, "service", 1, true, "The service to act on")
	cmd.AddExtraValues(&vars, "VAR=VAL", "Variables to set")

	cmd.CommandUsed = Hook(func() error {
		serv, err := service.LoadServiceByName(service_descr)
		if err != nil {
			return err
		}

		filename := file_flag
		if filename == "" {
			filename = serv.ConfigSetFile
		}

		changed_args := map[string]string{}
		for i := 0; i < len(vars); i++ {
			splits := strings.SplitN(vars[i], "=", 2)
			if len(splits) < 2 {
				continue
			}

			key, value := splits[0], splits[1]
			if serv.Config[key].String() != value {
				changed_args[key] = value
			} else {
				fmt.Printf("Configuration %q is already at %q\n", key, value)
			}
		}

		if len(changed_args) == 0 {
			fmt.Printf("No configuration change detected. Do nothing.\n")
			return nil
		}

		//
		// Add config
		//

		fmt.Printf("Update config in: %s\n", filename)
		err = service_public.ServiceSetConfig(filename, changed_args)
		if err != nil {
			return err
		}

		//
		// Check configuration has been applied
		//

		serv, err = service.LoadServiceByName(service_descr)
		if err != nil {
			return err
		}

		var failures []string
		for k, v := range changed_args {
			cfg, ok := serv.Config[k]
			if !ok {
				failures = append(failures, fmt.Sprintf("%q in unset, should be %q", k, v))
			} else if cfg.String() != v {
				failures = append(failures, fmt.Sprintf("%q in %q, should be %q", k, cfg.String(), v))
			}
		}
		if len(failures) > 0 {
			return fmt.Errorf("Configuration update failed: %v", strings.Join(failures, ", "))
		}

		//
		// reload service
		//

		if !no_reload_flag {
			return service_public.Reload(service_descr, service_public.ReloadOpts{
				NoBlock: no_block_flag,
			})
		}

		return nil
	})
	return cmd
}

func cmd_service_config() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("config")
	cmd.Description = "Manage service configuration"
	cmd.AttachSubcommand(cmd_service_config_ls(), 1)
	cmd.AttachSubcommand(cmd_service_config_get(), 1)
	cmd.AttachSubcommand(cmd_service_config_set(), 1)
	cmd.RequireSubcommand = true
	return cmd
}

func cmd_service_env() *flaggy.Subcommand {
	var service_name string
	cmd := flaggy.NewSubcommand("env") // "SERVICE",
	cmd.Description = "Print service template environment variables"
	cmd.AddPositionalValue(&service_name, "service", 1, true, "The service to act on")
	cmd.CommandUsed = Hook(func() error {
		log.Default().SetOutput(io.Discard)

		service, err := service.LoadServiceByName(service_name)
		if err != nil {
			return err
		}

		for _, v := range service.Vars() {
			fmt.Printf("%s\n", v)
		}

		return nil
	})
	return cmd
}

func cmd_service() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("service")
	cmd.ShortName = "s"
	cmd.Description = "Service commands"
	cmd.AttachSubcommand(cmd_service_start(), 1)
	cmd.AttachSubcommand(cmd_service_stop(), 1)
	cmd.AttachSubcommand(cmd_service_enable(), 1)
	cmd.AttachSubcommand(cmd_service_disable(), 1)
	cmd.AttachSubcommand(cmd_service_reload(), 1)
	cmd.AttachSubcommand(cmd_service_rolling_restart(), 1)
	cmd.AttachSubcommand(cmd_service_restart(), 1)
	cmd.AttachSubcommand(cmd_service_deploy(), 1)
	cmd.AttachSubcommand(cmd_service_inspect(), 1)
	cmd.AttachSubcommand(cmd_service_ls(), 1)
	cmd.AttachSubcommand(cmd_service_show("show"), 1)
	cmd.AttachSubcommand(cmd_service_show("print"), 1)
	cmd.AttachSubcommand(cmd_service_status(), 1)
	cmd.AttachSubcommand(cmd_service_unit(), 1)
	cmd.AttachSubcommand(cmd_service_config(), 1)
	cmd.AttachSubcommand(cmd_service_env(), 1)
	cmd.RequireSubcommand = true
	return cmd
}
