package deployment_util

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"

	"github.com/coreos/go-systemd/v22/dbus"

	"github.com/mildred/conductor.go/src/service"

	. "github.com/mildred/conductor.go/src/deployment"
)

func StartNewOrExistingFromService(ctx context.Context, svc *service.Service, seed *DeploymentSeed, max_deployment_index int, wants_fresh bool) (*Deployment, string, error) {
	sd, err := dbus.NewWithContext(ctx)
	if err != nil {
		return nil, "", err
	}

	var started_deployments []*Deployment
	var starting_deployments []*Deployment
	var stopped_deployments []*Deployment
	var deployment_units []string
	deployments, err := List(ListOpts{
		FilterServiceDir: svc.BasePath,
		FilterServiceId:  svc.Id,
	})
	if err != nil {
		return nil, "", err
	}

	for _, depl := range deployments {
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

	if len(started_deployments) > 0 && !wants_fresh {
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
			name = fmt.Sprintf("%s-%s-%s%d", svc.AppName, svc.InstanceName, seed.Prefix(), i)
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

		if name == "" && len(started_deployments) > 0 {
			log.Printf("Could not find free deployment name, but found started deployment %q", started_deployments[0].DeploymentName)
			return started_deployments[0], "active", nil
		} else if name == "" {
			return nil, "", fmt.Errorf("Failed to find free deployment name")
		}

		log.Printf("Create a new deployment %s from %s", name, svc.BasePath)

		//
		// Symlink the service config over to the deployment directory
		//

		dir, err := CreateDeploymentFromService(name, svc, seed)
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

func CreateDeploymentFromService(name string, svc *service.Service, seed *DeploymentSeed) (string, error) {
	dir := path.Join(DeploymentRunDir, name)

	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return "", err
	}

	err = os.Symlink(path.Join(svc.BasePath, service.ConfigName), path.Join(dir, service.ConfigName))
	if err != nil {
		return "", err
	}

	seed_data, err := json.Marshal(seed)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(path.Join(dir, SeedName), seed_data, 0644)
	if err != nil {
		return "", err
	}

	// var env string
	// env += fmt.Sprintf("CONDUCTOR_APP=%s\n", svc.AppName)
	// env += fmt.Sprintf("CONDUCTOR_INSTANCE=%s\n", svc.InstanceName)
	// env += fmt.Sprintf("CONDUCTOR_SERVICE_DIR=%s\n", svc.BasePath)
	// env += fmt.Sprintf("CONDUCTOR_DEPLOYMENT=%s\n", name)
	// err = os.WriteFile(path.Join(dir, "conductor-deployment.env"), []byte(env), 0644)
	// if err != nil {
	// 	return "", err
	// }

	err = os.MkdirAll("/run/systemd/system/"+DeploymentUnit(name)+".d", 0755)
	if err != nil {
		return "", err
	}

	var conf string = "[Service]\n"
	conf += fmt.Sprintf("LogExtraFields=CONDUCTOR_APP=%s\n", svc.AppName)
	conf += fmt.Sprintf("LogExtraFields=CONDUCTOR_INSTANCE=%s\n", svc.InstanceName)
	conf += fmt.Sprintf("LogExtraFields=CONDUCTOR_DEPLOYMENT=%s\n", name)
	err = os.WriteFile("/run/systemd/system/"+DeploymentUnit(name)+".d/50-journal.conf", []byte(conf), 0644)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("systemctl", "daemon-reload")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("while running systemctl daemon-reload, %v", err)
	}

	return dir, nil
}
