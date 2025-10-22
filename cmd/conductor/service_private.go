package main

import (
	"context"
	"fmt"

	"github.com/integrii/flaggy"

	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/service_internal"
)

func cmd_private_service_id() *flaggy.Subcommand {
	var service_name string
	var extra string
	var source bool
	var raw bool

	cmd := flaggy.NewSubcommand("id")
	cmd.Description = "Get the service ID"
	cmd.String(&extra, "extra", "e", "Extra data to compute the ID")
	cmd.Bool(&source, "source", "s", "Get the gata that contributes to the ID instead")
	cmd.Bool(&raw, "raw", "r", "Do not add extra end of line")
	cmd.AddPositionalValue(&service_name, "service", 1, true, "The service to act on")

	cmd.CommandUsed = Hook(func() error {
		s, err := service.LoadServiceByName(service_name)
		if err != nil {
			return err
		}

		var data string
		if source {
			var d []byte
			d, err = s.ComputeIdData(extra)
			data = string(d)
		} else {
			data, err = s.ComputeId(extra)
		}
		if err != nil {
			return err
		}

		if raw {
			fmt.Print(data)
		} else {
			fmt.Println(data)
		}
		return nil
	})
	return cmd
}

func cmd_private_service_template() *flaggy.Subcommand {
	var service string
	var template string

	cmd := flaggy.NewSubcommand("template")
	cmd.Description = "Run a template in the context of a service"
	cmd.AddPositionalValue(&service, "service", 1, true, "The service to act on")
	cmd.AddPositionalValue(&template, "template", 2, true, "The template file to run")

	cmd.CommandUsed = Hook(func() error {
		return service_internal.Template(context.Background(), service, template)
	})
	return cmd
}

func cmd_private_service_start() *flaggy.Subcommand {
	var service string
	var max_deployment_index = 10

	cmd := flaggy.NewSubcommand("start")
	cmd.Description = "Start a service"
	cmd.AddPositionalValue(&service, "service", 1, true, "The service to act on")
	cmd.Int(&max_deployment_index, "", "max-deployment-index", "Service will fail to deploy if it cannot find a deployment number below this")
	cmd.AdditionalHelpPrepend = "\n" +
		"Start a service by using an already starting deployment that has the\n" +
		"correct configuration or by starting a new deployment. Once the\n" +
		"deployment is started, old deployments for the service are stopped."

	cmd.CommandUsed = Hook(func() error {
		return service_internal.StartOrReload(service, service_internal.StartOrReloadOpts{
			Restart:            false,
			MaxDeploymentIndex: max_deployment_index,
		})

	})
	return cmd
}

func cmd_private_service_reload() *flaggy.Subcommand {
	var service string
	var fresh bool
	var max_deployment_index int = 10

	cmd := flaggy.NewSubcommand("reload")
	cmd.Description = "Reload a service"
	cmd.AddPositionalValue(&service, "service", 1, true, "The service to act on")
	cmd.Bool(&fresh, "", "fresh", "Do not reuse a started deployment is possible")
	cmd.Int(&max_deployment_index, "", "max-deployment-index", "Service will fail to deploy if it cannot find a deployment number below this")
	cmd.CommandUsed = Hook(func() error {
		return service_internal.StartOrReload(service, service_internal.StartOrReloadOpts{
			Restart:            true,
			WantsFresh:         fresh,
			MaxDeploymentIndex: max_deployment_index,
		})

	})
	return cmd
}

func cmd_private_service_stop() *flaggy.Subcommand {
	var service string

	cmd := flaggy.NewSubcommand("stop")
	cmd.Description = "Stop a service"
	cmd.AddPositionalValue(&service, "service", 1, true, "The service to act on")
	cmd.CommandUsed = Hook(func() error {
		return service_internal.Stop(service)
	})
	return cmd
}

func cmd_private_service_cleanup() *flaggy.Subcommand {
	var service string

	cmd := flaggy.NewSubcommand("cleanup")
	cmd.Description = "Clean up service after it has stopped"
	cmd.AddPositionalValue(&service, "service", 1, true, "The service to act on")
	cmd.CommandUsed = Hook(func() error {
		return service_internal.Cleanup(service)
	})
	return cmd
}

func cmd_private_service_register() *flaggy.Subcommand {
	var service string

	cmd := flaggy.NewSubcommand("register")
	cmd.Description = "Register service to load balancer"
	cmd.AddPositionalValue(&service, "service", 1, true, "The service to act on")
	cmd.CommandUsed = Hook(func() error {
		return service_internal.CaddyRegister(context.Background(), true, service)
	})
	return cmd
}

func cmd_private_service_deregister() *flaggy.Subcommand {
	var service string

	cmd := flaggy.NewSubcommand("deregister")
	cmd.Description = "Deregister service from load balancer"
	cmd.AddPositionalValue(&service, "service", 1, true, "The service to act on")
	cmd.CommandUsed = Hook(func() error {
		return service_internal.CaddyRegister(context.Background(), false, service)
	})
	return cmd
}

func cmd_private_service() *flaggy.Subcommand {
	cmd := flaggy.NewSubcommand("service")
	cmd.Description = "Manage conductor services"
	cmd.AttachSubcommand(cmd_private_service_start(), 1)
	cmd.AttachSubcommand(cmd_private_service_reload(), 1)
	cmd.AttachSubcommand(cmd_private_service_stop(), 1)
	cmd.AttachSubcommand(cmd_private_service_cleanup(), 1)
	cmd.AttachSubcommand(cmd_private_service_register(), 1)
	cmd.AttachSubcommand(cmd_private_service_deregister(), 1)
	cmd.AttachSubcommand(cmd_private_service_template(), 1)
	cmd.AttachSubcommand(cmd_private_service_id(), 1)
	cmd.RequireSubcommand = true
	return cmd
}
