package pod

import (
	"context"
	"fmt"
	"os"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/parts"
)

func init() {
	deployment.DeploymentPartImplementations[PodType] = func(depl *deployment.Deployment) deployment.DeploymentPartImplementation {
		return &PodDeploymentPart{deployment.DefaultDeploymentImplementation{depl}}
	}
}

type PodDeploymentPart struct {
	deployment.DefaultDeploymentImplementation
}

func (p *PodDeploymentPart) AllServiceUnits() []string {
	return []string{deployment.DeploymentServiceUnit(p.Depl.DeploymentName)}
}

func (p *PodDeploymentPart) CreateDeploymentUnits(ctx context.Context) (reload bool, err error) {
	svc := p.Depl.Service
	name := p.Depl.DeploymentName

	pod := svc.FindPod(p.Depl.PartName)

	if len(pod.ServiceDirectives) > 0 {
		for _, unit_name := range p.AllServiceUnits() {
			reload = true

			err = os.MkdirAll(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), unit_name+".d"), 0755)
			if err != nil {
				return reload, err
			}

			var conf string = "[Service]\n"
			conf += utils.JoinDirectives(pod.ServiceDirectives)
			err = os.WriteFile(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), unit_name+".d/90-extra-directives.conf"), []byte(conf), 0644)
			if err != nil {
				return reload, err
			}
		}
	}

	if pod.SocketActivated {
		vars, err := p.Depl.Vars(ctx)
		if err != nil {
			return reload, err
		}

		socket_directives := pod.ExpandedSocketDirectives(vars)
		if len(socket_directives) == 0 {
			return reload, fmt.Errorf("Cannot create socket file %s: missing socket directives in pod configuration", deployment.DeploymentSocketUnit(name))
		}

		reload = true

		var conf string
		conf += "[Unit]\n"
		conf += "Requires=" + deployment.DeploymentSocketConfigUnit(name) + "\n\n"
		conf += "[Socket]\n"
		conf += utils.JoinDirectives(socket_directives)
		err = os.WriteFile(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), deployment.DeploymentSocketUnit(name)), []byte(conf), 0644)
		if err != nil {
			return reload, err
		}
	}

	return reload, nil
}

func (p *PodDeploymentPart) AllUnitsStopOrder(context.Context) ([]deployment.UnitOrder, error) {
	return []deployment.UnitOrder{
		deployment.UnitOrder{10, deployment.DeploymentSocketUnit(p.Depl.DeploymentName)},
		deployment.UnitOrder{10, deployment.DeploymentServiceUnit(p.Depl.DeploymentName)},
		deployment.UnitOrder{50, deployment.DeploymentConfigUnit(p.Depl.DeploymentName)},
		deployment.UnitOrder{50, deployment.DeploymentSocketConfigUnit(p.Depl.DeploymentName)},
	}, nil
}
