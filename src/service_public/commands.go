package service_public

import (
	"context"
	"fmt"
	"strings"

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

func RunServiceCommand(service *Service, direct bool, env []string, cmd_name string, args ...string) error {
	command := service.Commands[cmd_name]

	if command == nil {
		return fmt.Errorf("Command %q does not exists", cmd_name)
	}

	if !command.Service && !command.ServiceAnyDeployment {
		return fmt.Errorf("Command %q does not run on services", cmd_name)
	}

	if !command.Service && command.ServiceAnyDeployment {
		// Run a deployment instead
		deployments, err := deployment_util.List(deployment_util.ListOpts{
			FilterServiceId:  service.Id,
			FilterServiceDir: service.BasePath,
		})
		if err != nil {
			return err
		}

		statuses, err := deployment_util.ListUnitStatus(context.Background(), deployments, false)
		if err != nil {
			return err
		}

		for i, depl := range deployments {
			st := statuses[i]
			if st.ActiveState == "active" {
				return RunCommand(command, direct, deployment.DeploymentDirByName(depl.DeploymentName), append(depl.Vars(), env...), cmd_name, args...)
			}
		}

		return fmt.Errorf("Could not find active deployment to run the command")
	}

	err := RunCommand(command, direct, service.BasePath, append(service.Vars(), env...), cmd_name, args...)
	return err
}
