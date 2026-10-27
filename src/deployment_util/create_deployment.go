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
		defer utils.AddPrefix(log.Default(), "[default part] ")()
	} else {
		defer utils.AddPrefix(log.Default(), "[part %s] ", part)()
	}

	var started_deployments []*Deployment
	var starting_deployments []*Deployment
	var stopped_deployments []*Deployment
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

		log.Printf("Deployment %s (%s / %s) is considered for reuse", depl.DeploymentName, stat.ActiveState, stat.SubState)
	}

	//
	// If there is a deployment starting or started with the identical config,
	// use it and wait for it to be started, else start a new deployment
	//

	var name string

	if len(started_deployments) > 0 && opts.ReuseDepl {
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

		log.Printf("Create a new deployment %q", name)
	} else {
		if len(started_deployments) > 0 && !opts.ReuseDepl {
			log.Printf("Requesting a fresh deployment, create new...")
		} else {
			log.Printf("No matching deployment, create new...")
		}

		//
		// Find a deployment name
		//

		var i = 1
		for i <= opts.MaxIndex {
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

		log.Printf("Create a new deployment %s", name)
	}

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

func CreateDeploymentFromService(name string, svc *service.Service, seed *DeploymentSeed) (string, error) {
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

	var unit_name string
	var service_directives []string
	var socket_directives []string
	if seed.IsPod {
		pod := svc.Pods.FindPod(seed.PartName)
		unit_name = DeploymentMainUnit(svc, seed.PartName, name)
		service_directives = pod.ServiceDirectives
		if pod.SocketActivated {
			socket_directives = pod.ExpandedSocketDirectives(depl.Vars())
			if len(socket_directives) == 0 {
				return "", fmt.Errorf("Cannot create socket file %s: missing socket directives in pod configuration", DeploymentSocketUnit(name))
			}
		}
	} else if seed.IsFunction {
		fct := svc.Functions.FindFunction(seed.PartName)
		if fct.IsSingle() {
			unit_name = CGIFunctionServiceUnitSingle(name)
		} else {
			unit_name = CGIFunctionServiceUnit(name, "")
		}
		service_directives = fct.ServiceDirectives
		err = CreateCGIFunctionUnits(name, fct)
		if err != nil {
			return "", err
		}
	}

	if unit_name != "" {

		err = os.MkdirAll(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), unit_name+".d"), 0755)
		if err != nil {
			return "", err
		}

		var conf string = "[Service]\n"
		conf += fmt.Sprintf("LogExtraFields=CONDUCTOR_APP=%s\n", svc.AppName)
		conf += fmt.Sprintf("LogExtraFields=CONDUCTOR_INSTANCE=%s\n", svc.InstanceName)
		conf += fmt.Sprintf("LogExtraFields=CONDUCTOR_DEPLOYMENT=%s\n", name)
		err = os.WriteFile(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), unit_name+".d/50-journal.conf"), []byte(conf), 0644)
		if err != nil {
			return "", err
		}

		if len(service_directives) > 0 {
			var conf string = "[Service]\n"
			for _, directive := range service_directives {
				conf += strings.ReplaceAll(directive, "\n", "\\\n") + "\n"
			}
			err = os.WriteFile(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), unit_name+".d/90-extra-directives.conf"), []byte(conf), 0644)
			if err != nil {
				return "", err
			}
		}

		if len(socket_directives) > 0 {
			var conf string
			conf += "[Unit]\n"
			conf += "Requires=" + DeploymentSocketConfigUnit(name) + "\n\n"
			conf += "[Socket]\n"
			for _, directive := range socket_directives {
				directive = strings.ReplaceAll(directive, "\n", "\\\n")
				conf += directive + "\n"
			}
			err = os.WriteFile(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), DeploymentSocketUnit(name)), []byte(conf), 0644)
			if err != nil {
				return "", err
			}
		}

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

func CreateCGIFunctionUnits(name string, f *service.ServiceFunction) error {
	var unit_name, accept, service_config string
	if f.IsSingle() {
		unit_name = CGIFunctionServiceUnitSingle(name)
		accept = "no"
	} else {
		unit_name = CGIFunctionServiceUnit(name, "")
		accept = "yes"
		service_config = "Type=oneshot\n" +
			"StandardInput=socket\n" +
			"StandardOutput=socket\n" +
			"StandardError=journal\n"
	}

	var service = `[Unit]
Description=Conductor CGI Function ` + name + `
CollectMode=inactive-or-failed

[Service]
` + service_config + `
WorkingDirectory=` + DeploymentDirByNameOnly(name) + `
Environment=CONDUCTOR_DEPLOYMENT=` + name + `
Environment=CONDUCTOR_SYSTEMD_UNIT=%n

ExecStart=/bin/sh -xc 'PID=$$$$; exec conductor _ deployment start --function'
`

	var socket = `[Unit]
Description=Conductor CGI Function socket for ` + name + `
Requires=` + DeploymentServiceUnit(name) + `
Requires=` + DeploymentConfigUnit(name) + `

[Socket]
ListenStream=` + DeploymentSocketPath(name) + `
Accept=` + accept + `

[Install]
WantedBy=sockets.target
`

	var function_conf = `[Unit]
Requires=` + DeploymentConfigUnit(name) + `
Before=` + DeploymentConfigUnit(name) + `

[Service]
Type=oneshot
ExitType=main
RemainAfterExit=yes
Restart=on-failure
`

	log.Printf("Create %s", dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode()))
	err := os.MkdirAll(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode()), 0755)
	if err != nil {
		return err
	}

	log.Printf("Write %s", dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), CGIFunctionSocketUnit(name)))
	err = os.WriteFile(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), CGIFunctionSocketUnit(name)), []byte(socket), 0o644)
	if err != nil {
		return err
	}

	log.Printf("Write %s", dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), unit_name))
	err = os.WriteFile(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), unit_name), []byte(service), 0o644)
	if err != nil {
		return err
	}

	log.Printf("Create %s", dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), DeploymentServiceUnit(name)+".d"))
	err = os.MkdirAll(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), DeploymentServiceUnit(name)+".d"), 0755)
	if err != nil {
		return err
	}

	log.Printf("Write %s", dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), DeploymentServiceUnit(name)+".d", "function.conf"))
	err = os.WriteFile(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), DeploymentServiceUnit(name)+".d", "function.conf"), []byte(function_conf), 0o644)
	if err != nil {
		return err
	}

	return nil
}
