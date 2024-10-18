package deployment_util

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/coreos/go-systemd/v22/dbus"

	"github.com/mildred/conductor.go/src/service"

	. "github.com/mildred/conductor.go/src/deployment"
)

func List() ([]*Deployment, error) {
	entries, err := os.ReadDir(DeploymentRunDir)
	if err != nil && !os.IsNotExist(err) {
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

func StartNewOrExistingFromService(ctx context.Context, svc *service.Service) (*Deployment, string, error) {
	sd, err := dbus.NewWithContext(ctx)
	if err != nil {
		return nil, "", err
	}

	var started_deployments []*Deployment
	var starting_deployments []*Deployment
	var deployment_units []string
	deployments, err := List()
	if err != nil {
		return nil, "", err
	}

	for _, depl := range deployments {
		if depl.ServiceDir != svc.BasePath || depl.ServiceId != svc.Id {
			continue
		}
		deployment_units = append(deployment_units, DeploymentUnit(depl.DeploymentName))
	}

	statuses, err := sd.ListUnitsByNamesContext(ctx, deployment_units)
	if err != nil {
		return nil, "", err
	}

	for _, depl := range deployments {
		if depl.ServiceDir != svc.BasePath || depl.ServiceId != svc.Id {
			continue
		}
		var stat dbus.UnitStatus
		for _, st := range statuses {
			if st.Name == DeploymentUnit(depl.DeploymentName) {
				stat = st
				break
			}
		}
		if stat.Name == "started" {
			started_deployments = append(started_deployments, depl)
		} else if stat.Name == "starting" {
			starting_deployments = append(starting_deployments, depl)
		}
	}

	//
	// If there is a deployment starting or started with the identical config,
	// use it and wait for it to be started, else start a new deployment
	//

	if len(started_deployments) > 0 {
		return started_deployments[0], "started", nil
	} else if len(starting_deployments) > 0 {
		return starting_deployments[0], "starting", nil
	} else {

		//
		// Find a deployment name
		//

		var name string
		var i = 1
		for {
			name = fmt.Sprintf("%s-%s-%d", svc.AppName, svc.InstanceName, i)
			_, sterr := os.Stat(path.Join(DeploymentRunDir, name))
			if err != nil && !os.IsNotExist(sterr) {
				return nil, "", err
			} else if err == nil {
				// the deployment exists, try next integer
				i = i + 1
				continue
			} else {
				break
			}
		}

		log.Printf("Create a new deployment %s from %s", name, svc.BasePath)

		//
		// Symlink the service config over to the deployment directory
		//

		dir, err := CreateDeploymentFromService(name, svc)
		if err != nil {
			return nil, "", err
		}

		depl, err := ReadDeployment(dir, name)
		if err != nil {
			return nil, "", err
		}

		return depl, "", nil
	}
}

func CreateDeploymentFromService(name string, svc *service.Service) (string, error) {
	dir := path.Join(DeploymentRunDir, name)

	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return "", err
	}

	err = os.Symlink(path.Join(svc.BasePath, service.ConfigName), path.Join(dir, service.ConfigName))
	if err != nil {
		return "", err
	}

	return dir, nil
}
