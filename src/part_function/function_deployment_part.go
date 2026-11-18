package function

import (
	"context"
	"log"
	"os"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/parts"
)

func init() {
	deployment.DeploymentPartImplementations[SocketType] = func(depl *deployment.Deployment) deployment.DeploymentPartImplementation {
		return &FunctionDeploymentPart{deployment.DefaultDeploymentImplementation{depl}}
	}
}

type FunctionDeploymentPart struct {
	deployment.DefaultDeploymentImplementation
}

func (p *FunctionDeploymentPart) AllServiceUnits() []string {
	fct := p.Depl.FindFunction(p.Depl.PartName)

	if fct.IsSingle() {
		return []string{deployment.CGIFunctionServiceUnitSingle(p.Depl.DeploymentName)}
	} else {
		return []string{deployment.CGIFunctionServiceUnit(p.Depl.DeploymentName, "")}
	}
}

func (p *FunctionDeploymentPart) CreateDeploymentUnits(ctx context.Context) (reload bool, err error) {
	reload = true
	svc := p.Depl.Service
	name := p.Depl.DeploymentName

	fct := svc.FindFunction(p.Depl.PartName)
	service_directives := fct.ServiceDirectives

	err = CreateCGIFunctionUnits(name, fct)
	if err != nil {
		return reload, err
	}

	if len(service_directives) > 0 {
		for _, unit_name := range p.AllServiceUnits() {
			err = os.MkdirAll(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), unit_name+".d"), 0755)
			if err != nil {
				return reload, err
			}

			var conf string = "[Service]\n"
			conf += utils.JoinDirectives(service_directives)
			err = os.WriteFile(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), unit_name+".d/90-extra-directives.conf"), []byte(conf), 0644)
			if err != nil {
				return reload, err
			}
		}
	}

	return reload, nil
}

func CreateCGIFunctionUnits(name string, f *service.ServiceFunction) error {
	var unit_name, accept, service_config string
	if f.IsSingle() {
		unit_name = deployment.CGIFunctionServiceUnitSingle(name)
		accept = "no"
	} else {
		unit_name = deployment.CGIFunctionServiceUnit(name, "")
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
WorkingDirectory=` + deployment.DeploymentDirByNameOnly(name) + `
Environment=CONDUCTOR_DEPLOYMENT=` + name + `
Environment=CONDUCTOR_SYSTEMD_UNIT=%n

ExecStart=/bin/sh -xc 'PID=$$$$; exec conductor _ deployment start --function'
`

	var socket = `[Unit]
Description=Conductor CGI Function socket for ` + name + `
Requires=` + deployment.DeploymentServiceUnit(name) + `
Requires=` + deployment.DeploymentConfigUnit(name) + `

[Socket]
ListenStream=` + deployment.DeploymentSocketPath(name) + `
Accept=` + accept + `

[Install]
WantedBy=sockets.target
`

	var function_conf = `[Unit]
Requires=` + deployment.DeploymentConfigUnit(name) + `
Before=` + deployment.DeploymentConfigUnit(name) + `

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

	log.Printf("Write %s", dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), deployment.CGIFunctionSocketUnit(name)))
	err = os.WriteFile(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), deployment.CGIFunctionSocketUnit(name)), []byte(socket), 0o644)
	if err != nil {
		return err
	}

	log.Printf("Write %s", dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), unit_name))
	err = os.WriteFile(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), unit_name), []byte(service), 0o644)
	if err != nil {
		return err
	}

	log.Printf("Create %s", dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), deployment.DeploymentServiceUnit(name)+".d"))
	err = os.MkdirAll(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), deployment.DeploymentServiceUnit(name)+".d"), 0755)
	if err != nil {
		return err
	}

	log.Printf("Write %s", dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), deployment.DeploymentServiceUnit(name)+".d", "function.conf"))
	err = os.WriteFile(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), deployment.DeploymentServiceUnit(name)+".d", "function.conf"), []byte(function_conf), 0o644)
	if err != nil {
		return err
	}

	return nil
}

func (p *FunctionDeploymentPart) AllUnitsStopOrder(context.Context) ([]deployment.UnitOrder, error) {
	return []deployment.UnitOrder{
		deployment.UnitOrder{10, deployment.CGIFunctionSocketUnit(p.Depl.DeploymentName)},
		deployment.UnitOrder{50, deployment.DeploymentConfigUnit(p.Depl.DeploymentName)},
	}, nil
}
