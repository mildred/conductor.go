package socket

import (
	"context"
	"log"
	"os"

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

	/*
		// ERROR: cannot find other parts unit name at start time
		var activate_service []string
		if sk.Activate.Part != nil {
			part := p.Depl.FindPart(ak.Activate.Part)
			deployment.DeploymentMainUnit()
		}
	*/

	socket_unit_name := deployment.SocketPartSocketUnit(name)
	service_unit_name := deployment.SocketPartServiceUnit(name)

	socket_conf := utils.JoinDirectives([]string{
		"[Unit]",
		"Description=Conductor socket for " + name,
		"Requires=" + service_unit_name,
		"",
		"[Socket]",
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
		"PartOf=" + socket_unit_name,
		"",
		"[Service]",
		"Sockets=" + socket_unit_name,
		"Type=oneshot",
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
