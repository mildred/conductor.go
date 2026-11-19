package daemon

import (
	"context"
	"fmt"
	"os"
	"strings"

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

		if len(service_directives) > 0 || len(daemon.Sockets) > 0 {
			var require_directives []string
			var sockets_directives []string
			for _, skname := range daemon.Sockets {

				sk := p.Depl.OtherParts[skname]
				if sk == nil {
					var other_parts_names []string
					for n, _ := range p.Depl.OtherParts {
						other_parts_names = append(other_parts_names, fmt.Sprintf("%+s", n))
					}
					if skname == "" {
						return reload, fmt.Errorf("could not find socket main part (available parts are: %s)", strings.Join(other_parts_names, ", "))
					} else {
						return reload, fmt.Errorf("could not find activate part %+v (available parts are: %s)", skname, strings.Join(other_parts_names, ", "))
					}
				}

				unit_name := deployment.DeploymentMainUnit(p.Depl.Service, sk.PartName, sk.DeploymentName)
				if !strings.HasSuffix(unit_name, ".socket") {
					if sk.PartName == "" {
						return reload, fmt.Errorf("activate %v main part main unit %+v is not a socket, the part might not have the correct type or might not be socket activated", sk.PartType, unit_name)
					} else {
						return reload, fmt.Errorf("activate %v part %+s main unit %+v is not a socket, the part might not have the correct type or might not be socket activated", sk.PartType, sk.PartName, unit_name)
					}
				}

				sockets_directives = append(sockets_directives, "Sockets="+unit_name)
				// use Wants rather than Requires to avoid the socket waiting on the
				// service to stop when restarting. Wants brings the socket for startup
				// but does not blocks stopping
				require_directives = append(require_directives, "Wants="+unit_name, "After="+unit_name)
			}

			conf := utils.JoinDirectives([]string{
				"[Unit]",
			}, require_directives, []string{
				"",
				"[Service]",
			}, sockets_directives, daemon.ServiceDirectives)

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
