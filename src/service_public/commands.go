package service_public

import (
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/rodaine/table"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/deployment_util"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/service"
	. "github.com/mildred/conductor.go/src/service_util"
)

func PrintListCommands(service *Service) error {
	tbl := table.New("NAME", "DESCRIPTION", "AVAILABLE").WithPrintHeaders(true)

	status, err := service.UnitStatus(context.Background())
	if err != nil {
		return err
	}

	for _, name := range utils.SortedStringKeys(service.Commands) {
		cmd := service.Commands[name]

		if cmd.Service || cmd.ServiceAnyDeployment {
			available := "yes"
			if !cmd.Service && cmd.ServiceAnyDeployment && status.ActiveState != "active" {
				available = "no"
			}
			tbl.AddRow(strings.Join(append([]string{name}, cmd.HelpArgs...), " "), cmd.Description, available)

			for _, help := range cmd.GetTabbedHelpFlags().Lines() {
				tbl.AddRow("", "    "+help, "")
			}
		}
	}

	tbl.Print()

	return nil
}

func RunServiceCommand(service *Service, direct bool, strictVersion bool, env []string, cmd_name string, args ...string) error {
	command := service.Commands[cmd_name]

	if command == nil {
		return fmt.Errorf("Command %q does not exists", cmd_name)
	}

	if !command.Service && !command.ServiceAnyDeployment {
		return fmt.Errorf("Command %q does not run on services", cmd_name)
	}

	if !command.Service && command.ServiceAnyDeployment {
		// Run a deployment instead

		var deployments []*deployment.Deployment
		var statuses []dbus.UnitStatus

		var filterServiceIds = []string{service.Id}
		if !strictVersion {
			filterServiceIds = append(filterServiceIds, "")
		}

		for _, filterServiceId := range filterServiceIds {
			var err error
			deployments, err = deployment_util.List(deployment_util.ListOpts{
				FilterServiceId:  filterServiceId,
				FilterServiceDir: service.BasePath,
			})
			if err != nil {
				return err
			}

			statuses, err = deployment_util.ListUnitStatus(context.Background(), deployments, false)
			if err != nil {
				return err
			}

			for i, depl := range deployments {
				st := statuses[i]
				if st.ActiveState == "active" {
					return RunCommand(command, direct, deployment.DeploymentDirByNameOnly(depl.DeploymentName), append(depl.Vars(), env...), cmd_name, args...)
				}
			}
		}

		var inactive_deployments []string
		for i, depl := range deployments {
			st := statuses[i]
			if st.ActiveState == "active" {
				continue
			}
			inactive_deployments = append(inactive_deployments, fmt.Sprintf("%s is %s", depl.DeploymentName, st.ActiveState))
		}

		if len(deployments) == 0 {
			return fmt.Errorf("Could not find an active deployment to run the command: there is no deployment found for service %s with id %s.", service.BasePath, service.Id)
		} else {
			return fmt.Errorf("Could not find an active deployment to run the command: %s", strings.Join(inactive_deployments, ", "))
		}
	}

	err := RunCommand(command, direct, service.BasePath, append(service.Vars(), env...), cmd_name, args...)
	return err
}
