package daemon

import (
	"context"
	"os"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/parts"
)

func init() {
	deployment.DeploymentPartImplementations[DaemonType] = func(depl *deployment.Deployment) deployment.DeploymentPartImplementation {
		return &DaemonDeploymentPart{deployment.DefaultDeploymentImplementation{depl}}
	}
}

type DaemonDeploymentPart struct {
	deployment.DefaultDeploymentImplementation
}

func (d *DaemonDeploymentPart) AllServiceUnits() []string {
	return []string{
		deployment.DeploymentServiceUnit(d.Depl.DeploymentName),
	}
}

func (p *DaemonDeploymentPart) CreateDeploymentUnits(ctx context.Context) (reload bool, err error) {
	svc := p.Depl.Service
	name := p.Depl.DeploymentName

	unit_name := deployment.DeploymentServiceUnit(name)
	daemon := svc.FindDaemon(p.Depl.PartName)
	service_directives := daemon.ServiceDirectives

	if unit_name != "" {
		reload = true

		err = os.MkdirAll(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), unit_name+".d"), 0755)
		if err != nil {
			return reload, err
		}

		if len(service_directives) > 0 {
			var conf string = "[Service]\n"
			conf += utils.JoinDirectives(service_directives)
			err = os.WriteFile(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), unit_name+".d/90-extra-directives.conf"), []byte(conf), 0644)
			if err != nil {
				return reload, err
			}
		}
	}

	/*
		// ERROR: cannot find the unit name at start time
		for _, part := range p.Depl.Parts {
			sk, ok := part.(*service.ServiceSocket)
			if !ok {
				continue
			}

			if sk.Activate.Part == nil || *sk.Activate.Part != p.Depl.PartName {
				continue
			}

			err = os.MkdirAll(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), unit_name+".d"), 0755)
			if err != nil {
				return reload, err
			}

			if len(service_directives) > 0 {
				var conf string = "[Service]\n"
				conf += utils.JoinDirectives(service_directives)
				err = os.WriteFile(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), unit_name+".d/90-extra-directives.conf"), []byte(conf), 0644)
				if err != nil {
					return reload, err
				}
			}
		}
	*/

	return reload, nil
}

func (p *DaemonDeploymentPart) AllUnitsStopOrder(context.Context) ([]deployment.UnitOrder, error) {
	return []deployment.UnitOrder{
		deployment.UnitOrder{10, deployment.DeploymentServiceUnit(p.Depl.DeploymentName)},
		deployment.UnitOrder{50, deployment.DeploymentConfigUnit(p.Depl.DeploymentName)},
	}, nil
}
