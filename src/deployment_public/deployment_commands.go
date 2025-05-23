package deployment_public

import (
	"fmt"
	"strings"

	"github.com/rodaine/table"

	"github.com/mildred/conductor.go/src/service_util"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/deployment"
)

func PrintListCommands(depl *Deployment) error {
	tbl := table.New("NAME", "DESCRIPTION").WithPrintHeaders(true)

	for _, name := range utils.SortedStringKeys(depl.Commands) {
		cmd := depl.Commands[name]

		if cmd.Deployment {
			tbl.AddRow(strings.Join(append([]string{name}, cmd.HelpArgs...), " "), cmd.Description)

			for _, help := range cmd.GetTabbedHelpFlags().Lines() {
				tbl.AddRow("", "    "+help)
			}
		}
	}

	tbl.Print()

	return nil
}

func RunDeploymentCommand(depl *Deployment, direct bool, env []string, cmd_name string, args ...string) error {
	command := depl.Commands[cmd_name]

	if command != nil && !command.Deployment {
		return fmt.Errorf("Command %q does not run on deployments", cmd_name)
	}

	return service_util.RunCommand(command, direct, DeploymentDirByName(depl.DeploymentName, true), append(depl.Vars(), env...), cmd_name, args...)
}
