package deployment_util

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/coreos/go-systemd/v22/unit"
	"github.com/taigrr/systemctl"
	"github.com/taigrr/systemctl/properties"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/deployment"
)

type StartNewOrExistingOpts struct {
	MaxIndex  int
	ReuseDepl bool
}

func StartNewOrExistingFromService(ctx context.Context, svc *service.Service, seed *DeploymentSeed, opts StartNewOrExistingOpts) (*Deployment, string, error) {
	// sd, err := utils.NewSystemdClient(ctx)
	// if err != nil {
	//   return nil, "", err
	// }

	part := seed.PartName
	if part == "" {
		defer utils.AddPrefix(log.Default(), "default part: ")()
	} else {
		defer utils.AddPrefix(log.Default(), "part %s: ", part)()
	}

	var started_deployments []*Deployment
	var starting_deployments []*Deployment
	var stopped_deployments []*Deployment
	var once_deployments []*Deployment
	// var deployment_units []string
	deployments, err := List(ListOpts{
		FilterServiceDir: svc.BasePath,
		FilterPartName:   &seed.PartName,
		FilterPartId:     seed.PartId,
	})
	if err != nil {
		return nil, "", err
	}

	// for _, depl := range deployments {
	// 	deployment_units = append(deployment_units, DeploymentUnit(depl.DeploymentName))
	// }

	//
	// For some reason this is blocking
	//
	// log.Printf("Gather deployment status... %v", deployment_units)
	// var statuses []dbus.UnitStatus
	// if len(deployment_units) > 0 {
	// 	statuses, err = sd.ListUnitsByPatternsContext(ctx, deployment_units)
	// 	if err != nil {
	// 		return nil, "", err
	// 	}
	// }
	// log.Printf("Gathered deployment statuses: %v", statuses)

	part_ids, err := svc.PartIds(ctx)
	if err != nil {
		return nil, "", err
	}

	for _, depl := range deployments {
		log.Printf("Considering deployment %v...", depl.DeploymentName)

		should_match := depl.AppName == svc.AppName && depl.InstanceName == svc.InstanceName
		if depl.ServiceDir != svc.BasePath {
			if should_match {
				log.Printf("Deployment %s do not match (service %q != %q)", depl.DeploymentName, depl.ServiceDir, svc.BasePath)
			}
			continue
		}

		part_id, _ := part_ids[depl.PartName]
		if depl.PartId != part_id {
			if should_match {
				log.Printf("Deployment %s do not match (id %q != %q)", depl.DeploymentName, depl.PartId, part_id)
			}
			continue
		}

		if depl.Once != seed.Once {
			if should_match {
				log.Printf("Deployment %s do not match (once %v != %v)", depl.DeploymentName, depl.Once, seed.Once)
			}
			continue
		}

		var stat dbus.UnitStatus

		// for _, st := range statuses {
		//   if st.Name == DeploymentUnit(depl.DeploymentName) {
		//     stat = st
		//     break
		//   }
		// }

		stat.ActiveState, err = systemctl.Show(ctx, depl.MainUnit(), properties.ActiveState, systemctl.Options{UserMode: !dirs.AsRoot})
		if err != nil {
			return nil, "", err
		}

		if depl.Once {
			once_deployments = append(once_deployments, depl)
		} else if stat.ActiveState == "failed" {
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

		log.Printf("Deployment %s (%s / %s) is considered for reuse", depl.DeploymentName, stat.ActiveState, stat.SubState)
	}

	//
	// If there is a deployment starting or started with the identical config,
	// use it and wait for it to be started, else start a new deployment
	//

	var name string

	if len(once_deployments) > 0 {
		log.Printf("Found deployment %q", once_deployments[0].DeploymentName)
		return once_deployments[0], "", nil
	} else if len(started_deployments) > 0 && opts.ReuseDepl {
		log.Printf("Found started deployment %q", started_deployments[0].DeploymentName)
		return started_deployments[0], "active", nil
	} else if len(starting_deployments) > 0 {
		log.Printf("Found starting deployment %q", starting_deployments[0].DeploymentName)
		return starting_deployments[0], "activating", nil
	} else if len(stopped_deployments) > 0 {
		log.Printf("Found stopped deployment %q", stopped_deployments[0].DeploymentName)
		log.Printf("Removing deployment...")

		name = stopped_deployments[0].DeploymentName
		err = RemoveTimeout(ctx, name, 0, 0)
		if err != nil {
			return nil, "", err
		}

		log.Printf("Create a new deployment %q (from stopped)", name)
	} else {
		if len(started_deployments) > 0 && !opts.ReuseDepl {
			log.Printf("Requesting a fresh deployment, create new...")
		} else {
			log.Printf("No matching deployment, create new...")
		}

		//
		// Find a deployment name
		//

		name, err = FindDeploymentName(svc, seed, opts.MaxIndex)
		if err != nil {
			return nil, "", err
		}

		if name == "" && len(started_deployments) > 0 {
			log.Printf("Could not find free deployment name, but found started deployment %q", started_deployments[0].DeploymentName)
			return started_deployments[0], "active", nil
		} else if name == "" {
			return nil, "", fmt.Errorf("Failed to find free deployment name")
		}

		log.Printf("Create a new deployment %s", name)
	}

	//
	// Symlink the service config over to the deployment directory
	//

	log.Printf("Create new deployment %+s from service %v", name, svc.BasePath)
	dir, err := CreateDeploymentFromService(ctx, name, svc, seed)
	if err != nil {
		return nil, "", err
	}

	depl, err := ReadDeployment(dir, name)
	if err != nil {
		return nil, "", err
	}

	return depl, "", nil
}

func FindDeploymentName(svc *service.Service, seed *deployment.DeploymentSeed, maxIndex int) (name string, err error) {
	var prefix string
	if svc.AppName == "" || svc.InstanceName == "" {
		if svc.Name == "" {
			prefix = unit.UnitNamePathEscape(svc.BasePath)
		} else {
			prefix = svc.Name
		}
	} else {
		prefix = fmt.Sprintf("%s-%s", svc.AppName, svc.InstanceName)
	}

	var i int
	if seed.Once {
		i = 0
	} else {
		i = 1
	}

	var candidates []string

	for i <= maxIndex {
		if i == 0 {
			name = fmt.Sprintf("%s%s", prefix, seed.Suffix())
		} else {
			name = fmt.Sprintf("%s%s-%d", prefix, seed.Suffix(), i)
		}

		if seed.Once {
			// TODO: handle the case when the deployment exists but is a different
			// service
			break
		} else {
			_, err := os.Stat(path.Join(DeploymentRunDir, name))
			if err != nil && !os.IsNotExist(err) {
				return "", err
			} else if err == nil {
				// the deployment exists, try next integer
				i = i + 1
				candidates = append(candidates, name)
				name = ""
				continue
			} else {
				break
			}
		}
	}

	if name == "" {
		log.Printf("Dis not found deployment name, candidates were: %s", name, strings.Join(candidates, ", "))
	} else if len(candidates) > 0 {
		log.Printf("Found new deployment name %s (candidates were: %s)", name, strings.Join(candidates, ", "))
	} else {
		log.Printf("Found new deployment name %s", name)
	}

	return name, nil
}

func CreateDeploymentFromService(ctx context.Context, name string, svc *service.Service, seed *DeploymentSeed) (string, error) {
	dir := DeploymentRunPath(name)

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

	depl := deployment.NewDeploymentFromService(svc, name, seed)

	reload, err := depl.CreateDeploymentUnits(ctx)
	if err != nil {
		return "", err
	}

	for _, service_unit := range depl.AllServiceUnits() {
		reload = true

		err = os.MkdirAll(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), service_unit+".d"), 0755)
		if err != nil {
			return "", err
		}

		var conf string = "[Service]\n"
		conf += fmt.Sprintf("LogExtraFields=CONDUCTOR_APP=%s\n", svc.AppName)
		conf += fmt.Sprintf("LogExtraFields=CONDUCTOR_INSTANCE=%s\n", svc.InstanceName)
		conf += fmt.Sprintf("LogExtraFields=CONDUCTOR_DEPLOYMENT=%s\n", name)
		err = os.WriteFile(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), service_unit+".d/50-journal.conf"), []byte(conf), 0644)
		if err != nil {
			return "", err
		}
	}

	if reload {
		cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "daemon-reload")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return "", fmt.Errorf("while running systemctl %s daemon-reload, %v", dirs.SystemdModeFlag(), err)
		}
	}

	return dir, nil
}
