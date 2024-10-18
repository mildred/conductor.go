package deployment

import (
	"os"
	"path"
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
