package deployment_public

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/coreos/go-systemd/v22/unit"
	"github.com/rodaine/table"

	. "github.com/mildred/conductor.go/src/deployment"
	. "github.com/mildred/conductor.go/src/deployment_util"
)

func PrintList() error {
	deployments, err := List()
	if err != nil {
		return err
	}

	tbl := table.New("App", "Instance", "Deployment")
	for _, depl := range deployments {
		tbl.AddRow(depl.AppName, depl.InstanceName, depl.DeploymentName)
	}
	tbl.Print()
	return nil
}

func PrintInspect(deployments ...string) error {
	if len(deployments) == 0 {
		return PrintInspect(".")
	}

	for _, dir := range deployments {
		depl, err := ReadDeployment(dir, "")
		if err != nil {
			return err
		}

		err = json.NewEncoder(os.Stdout).Encode(depl)
		if err != nil {
			return err
		}
	}
	return nil
}

func Stop(deployment_name string) error {
	unit := fmt.Sprintf("conductor-deployment@%s.service", unit.UnitNameEscape(deployment_name))
	return exec.Command("systemctl", "stop", unit).Run()
}

func Start(deployment_name string) error {
	unit := fmt.Sprintf("conductor-deployment@%s.service", unit.UnitNameEscape(deployment_name))
	return exec.Command("systemctl", "start", unit).Run()
}
