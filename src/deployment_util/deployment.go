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

func StartNewOrExistingFromService(ctx context.Context, svc *service.Service, max_deployment_index int) (*Deployment, string, error) {
	sd, err := dbus.NewWithContext(ctx)
	if err != nil {
		return nil, "", err
	}

	var started_deployments []*Deployment
	var starting_deployments []*Deployment
	var stopped_deployments []*Deployment
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
		should_match := depl.AppName == svc.AppName && depl.InstanceName == svc.InstanceName
		if depl.ServiceDir != svc.BasePath {
			if should_match {
				log.Printf("Deployment %s do not match (service %q != %q)", depl.DeploymentName, depl.ServiceDir, svc.BasePath)
			}
			continue
		}
		if depl.ServiceId != svc.Id {
			if should_match {
				log.Printf("Deployment %s do not match (id %q != %q)", depl.DeploymentName, depl.ServiceId, svc.Id)
			}
			continue
		}
		var stat dbus.UnitStatus
		for _, st := range statuses {
			if st.Name == DeploymentUnit(depl.DeploymentName) {
				stat = st
				break
			}
		}
		if stat.ActiveState == "failed" {
			log.Printf("Deployment %s do not match (state is %s / %s)", depl.DeploymentName, stat.ActiveState, stat.SubState)
			continue
		} else if stat.ActiveState == "active" {
			started_deployments = append(started_deployments, depl)
		} else if stat.ActiveState == "activating" {
			starting_deployments = append(starting_deployments, depl)
		} else if stat.ActiveState == "inactive" {
			stopped_deployments = append(stopped_deployments, depl)
		} else {
			// TODO: consider for reuse
			log.Printf("Deployment %s do not match (state is %s / %s)", depl.DeploymentName, stat.ActiveState, stat.SubState)
			continue
		}

		log.Printf("Deployment %s (%s / %s) is considered to reuse", depl.DeploymentName, stat.ActiveState, stat.SubState)
	}

	//
	// If there is a deployment starting or started with the identical config,
	// use it and wait for it to be started, else start a new deployment
	//

	if len(started_deployments) > 0 {
		log.Printf("found started deployment %q", started_deployments[0].DeploymentName)
		return started_deployments[0], "active", nil
	} else if len(starting_deployments) > 0 {
		log.Printf("found starting deployment %q", starting_deployments[0].DeploymentName)
		return starting_deployments[0], "activating", nil
	} else if len(stopped_deployments) > 0 {
		log.Printf("found stopped deployment %q", stopped_deployments[0].DeploymentName)
		return stopped_deployments[0], "inactive", nil
	} else {

		//
		// Find a deployment name
		//

		var name string
		var i = 1
		for i <= max_deployment_index {
			name = fmt.Sprintf("%s-%s-%d", svc.AppName, svc.InstanceName, i)
			log.Printf("Trying new deployment name %s", name)
			_, err := os.Stat(path.Join(DeploymentRunDir, name))
			if err != nil && !os.IsNotExist(err) {
				return nil, "", err
			} else if err == nil {
				// the deployment exists, try next integer
				i = i + 1
				name = ""
				continue
			} else {
				break
			}
		}

		if name == "" {
			return nil, "", fmt.Errorf("Failed to find free deployment name")
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
