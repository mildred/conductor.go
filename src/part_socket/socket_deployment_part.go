package socket

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/parts"
)

func init() {
	deployment.DeploymentPartImplementations[SocketType] = func(depl *deployment.Deployment) deployment.DeploymentPartImplementation {
		return &SocketDeploymentPart{deployment.DefaultDeploymentImplementation{depl}}
	}
}

type SocketDeploymentPart struct {
	deployment.DefaultDeploymentImplementation
}

func (d *SocketDeploymentPart) AllServiceUnits() []string {
	return []string{
		deployment.SocketPartServiceUnit(d.Depl.DeploymentName),
	}
}

func (p *SocketDeploymentPart) CreateDeploymentUnits(ctx context.Context) (reload bool, err error) {
	reload = true
	svc := p.Depl.Service
	name := p.Depl.DeploymentName
	sk := svc.Parts.FindSocket(p.Depl.PartName)

	vars, err := p.Depl.Vars(ctx)
	if err != nil {
		return reload, err
	}

	log.Printf("Create %s", dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode()))
	err = os.MkdirAll(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode()), 0755)
	if err != nil {
		return reload, err
	}

	var activate_service string
	if sk.Activate.Part != nil {
		part := p.Depl.OtherParts[*sk.Activate.Part]
		if part == nil {
			var other_parts_names []string
			for n, _ := range p.Depl.OtherParts {
				other_parts_names = append(other_parts_names, fmt.Sprintf("%+s", n))
			}
			if *sk.Activate.Part == "" {
				return reload, fmt.Errorf("could not find activate main part (available parts are: %s)", strings.Join(other_parts_names, ", "))
			} else {
				return reload, fmt.Errorf("could not find activate part %+v (available parts are: %s)", *sk.Activate.Part, strings.Join(other_parts_names, ", "))
			}
		}
		unit_name := deployment.DeploymentMainUnit(p.Depl.Service, part.PartName, part.DeploymentName)
		if !strings.HasSuffix(unit_name, ".service") {
			return reload, fmt.Errorf("activate %v part %+v main unit %+v is not a service, the part might not have the correct type or might already be socket activated", part.PartType, part.PartName, unit_name)
		}
		activate_service = unit_name
	}

	socket_unit_name := deployment.SocketPartSocketUnit(name)
	service_unit_name := deployment.SocketPartServiceUnit(name)

	socket_conf := utils.JoinDirectives([]string{
		"[Unit]",
		"Description=Conductor socket for " + name,
		// Use Wants rather than Requires to avoid the signaling service stop to
		// trigger a socket stop, especially since the signalling unit is a oneshot
		// and does not remain after exit
		"Wants=" + service_unit_name,
		"Before=" + service_unit_name,
		"",
		"[Socket]",
		"Service=" + activate_service,
	}, utils.ExpandAllVars(sk.Directives, vars), []string{
		"",
		"[Install]",
		"WantedBy=sockets.target",
	})

	log.Printf("Write %s", dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), socket_unit_name))
	err = os.WriteFile(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), socket_unit_name), []byte(socket_conf), 0o644)
	if err != nil {
		return reload, err
	}

	service_conf := utils.JoinDirectives([]string{
		"[Unit]",
		"Description=Conductor signaling service for socket " + name,
		// "PartOf=" + socket_unit_name,
		"",
		"[Service]",
		"Sockets=" + socket_unit_name,
		"Type=oneshot",
		// do not remain after exit to allow the signalling unit to restart when the
		// socket restarts. It should not remain active while the socket restarts
		"RemainAfterExit=no",
		"WorkingDirectory=" + p.Depl.DeploymentDir,
		"",
		"Environment=CONDUCTOR_LOG_COMMAND=1",
		"Environment=CONDUCTOR_DEPLOYMENT=" + p.Depl.DeploymentName,
		"Environment=CONDUCTOR_SYSTEMD_UNIT=%n",
		"",
		"ExecStartPre=conductor  _ deployment prepare",
		"ExecStart=conductor     _ deployment start",
		"ExecStartPost=conductor _ deployment start-post",
		"ExecStop=conductor      _ deployment stop",
		"ExecStopPost=conductor  _ deployment cleanup",
		"",
	}, sk.ServiceDirectives)

	log.Printf("Write %s", dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), service_unit_name))
	err = os.WriteFile(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), service_unit_name), []byte(service_conf), 0o644)
	if err != nil {
		return reload, err
	}

	return reload, nil
}

func (p *SocketDeploymentPart) AllUnitsStopOrder(context.Context) ([]deployment.UnitOrder, error) {
	return []deployment.UnitOrder{
		deployment.UnitOrder{50, deployment.SocketPartSocketUnit(p.Depl.DeploymentName)},
		deployment.UnitOrder{50, deployment.SocketPartServiceUnit(p.Depl.DeploymentName)},
	}, nil
}
