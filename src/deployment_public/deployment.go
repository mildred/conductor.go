package deployment_public

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/rodaine/table"

	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/deployment"
	. "github.com/mildred/conductor.go/src/deployment_util"
)

type PrintListSettings struct {
	Unit             bool
	ServiceUnit      bool
	FilterServiceDir string
	QuietServiceInfo bool
}

func PrintList(settings PrintListSettings) error {
	var ctx = context.Background()
	sd, err := dbus.NewWithContext(ctx)
	if err != nil {
		return err
	}

	units, err := sd.ListUnitsByPatternsContext(ctx, nil, []string{"conductor-deployment@*.service", "conductor-service@*.service"})
	if err != nil {
		return err
	}

	deployments, err := List(ListOpts{
		FilterServiceDir: settings.FilterServiceDir,
	})
	if err != nil {
		return err
	}

	var list_services []*service.Service
	var list_depl []*Deployment
	var list_service_status []dbus.UnitStatus
	var list_depl_status []dbus.UnitStatus
	var extra_service_cols []string
	var extra_depl_cols []string

	for _, depl := range deployments {
		var unit, service_unit dbus.UnitStatus
		for _, u := range units {
			if u.Name == DeploymentUnit(depl.DeploymentName) {
				unit = u
			} else if u.Name == service.ServiceUnit(depl.ServiceDir) {
				service_unit = u
			}
		}

		service, err := service.LoadServiceDir(depl.ServiceDir)
		if err != nil {
			return err
		}

		list_services = append(list_services, service)

		list_depl = append(list_depl, depl)
		list_service_status = append(list_service_status, service_unit)
		list_depl_status = append(list_depl_status, unit)

		if extra_service_cols == nil {
			extra_service_cols = depl.DisplayServiceConfig
		} else {
			utils.IntersectHoles(&extra_service_cols, depl.DisplayServiceConfig)
		}

		if extra_depl_cols == nil {
			extra_depl_cols = depl.DisplayDeploymentConfig
		} else {
			utils.IntersectHoles(&extra_depl_cols, depl.DisplayDeploymentConfig)
		}
	}

	extra_service_cols = utils.Compact(extra_service_cols...)
	extra_depl_cols = utils.Compact(extra_depl_cols...)

	row := []interface{}{}
	if !settings.QuietServiceInfo {
		row = append(row, "App", "Instance", "Enabled", "Active")
		if settings.ServiceUnit {
			row = append(row, "Service")
		}
		for _, col := range extra_service_cols {
			row = append(row, col)
		}
	}
	row = append(row, "Deployment", "Active", "State", "IP")
	if settings.Unit {
		row = append(row, "Unit")
	}
	for _, col := range extra_depl_cols {
		row = append(row, col)
	}

	tbl := table.New(row...)

	for i, depl := range list_depl {
		s := list_services[i]
		ss := list_service_status[i]
		ds := list_depl_status[i]

		row := []interface{}{}
		if !settings.QuietServiceInfo {
			row = append(row, s.AppName, s.InstanceName, ss.LoadState, ss.ActiveState)
			if settings.ServiceUnit {
				row = append(row, ss.Name)
			}
			for _, col := range extra_service_cols {
				row = append(row, s.Config[col])
			}
		}
		row = append(row, depl.DeploymentName, ds.ActiveState, ds.SubState, depl.PodIpAddress)
		if settings.Unit {
			row = append(row, ds.Name)
		}
		for _, col := range extra_depl_cols {
			row = append(row, depl.Config[col])
		}
		tbl.AddRow(row...)
	}
	if len(list_depl) > 0 {
		tbl.Print()
	}
	fmt.Printf("(%d deployments in %q)\n", len(list_depl), DeploymentRunDir)
	return nil
}

func PrintInspect(deployments ...string) error {
	if len(deployments) == 0 {
		return PrintInspect(".")
	}

	for _, dir := range deployments {
		depl, err := ReadDeployment(dir, "")
		if err != nil {
			return err
		}

		err = json.NewEncoder(os.Stdout).Encode(depl)
		if err != nil {
			return err
		}
	}
	return nil
}

func Stop(deployment_name string) error {
	fmt.Fprintf(os.Stderr, "+ systemctl stop %q\n", DeploymentUnit(deployment_name))
	cmd := exec.Command("systemctl", "stop", DeploymentUnit(deployment_name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Remove(deployment_name string) error {
	fmt.Fprintf(os.Stderr, "+ systemctl stop %q\n", DeploymentUnit(deployment_name))
	cmd := exec.Command("systemctl", "stop", DeploymentUnit(deployment_name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -rf %q\n", DeploymentDirByName(deployment_name))
	err = os.RemoveAll(DeploymentDirByName(deployment_name))
	if err != nil {
		return err
	}

	return nil
}

func Start(deployment_name string) error {
	fmt.Fprintf(os.Stderr, "+ systemctl start %q\n", DeploymentUnit(deployment_name))
	cmd := exec.Command("systemctl", "start", DeploymentUnit(deployment_name))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
