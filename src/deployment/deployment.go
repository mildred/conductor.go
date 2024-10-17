package deployment

import (
	"os"
	"path"

	"github.com/rodaine/table"
)

func List() ([]*Deployment, error) {
	entries, err := os.ReadDir(DeploymentRunDir)
	if err != nil {
		return nil, err
	}

	var res []*Deployment
	for _, ent := range entries {
		depl, err := ReadDeployment(path.Join(DeploymentRunDir, ent.Name()), ent.Name())
		if err != nil {
			return nil, err
		}
		res = append(res, depl)
	}

	return res, nil
}

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
