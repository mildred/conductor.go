package deployment_public

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

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
	ConfigStatus     bool
	FilterServiceDir string
	QuietServiceInfo bool
}

func PrintList(settings PrintListSettings) error {
	var ctx = context.Background()
	sd, err := dbus.NewWithContext(ctx)
	if err != nil {
		return err
	}

	units, err := sd.ListUnitsByPatternsContext(ctx, nil, []string{"conductor-deployment@*.service", "conductor-service@*.service", "conductor-deployment-config@*.service"})
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
	var list_depl_config_status []dbus.UnitStatus
	var extra_service_cols []string
	var extra_depl_cols []string

	for _, depl := range deployments {
		var unit, service_unit, config_unit dbus.UnitStatus
		for _, u := range units {
			if u.Name == DeploymentUnit(depl.DeploymentName) {
				unit = u
			} else if u.Name == DeploymentConfigUnit(depl.DeploymentName) {
				config_unit = u
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
		list_depl_config_status = append(list_depl_config_status, config_unit)

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
	if settings.ConfigStatus {
		row = append(row, "Reverse-Proxy")
	}
	for _, col := range extra_depl_cols {
		row = append(row, col)
	}

	tbl := table.New(row...)

	for i, depl := range list_depl {
		s := list_services[i]
		ss := list_service_status[i]
		ds := list_depl_status[i]
		dcs := list_depl_config_status[i]

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
		var ip_addr string
		if depl.Pod != nil {
			ip_addr = depl.Pod.IPAddress
		}
		row = append(row, depl.DeploymentName, ds.ActiveState, ds.SubState, ip_addr)
		if settings.Unit {
			row = append(row, ds.Name)
		}
		if settings.ConfigStatus {
			row = append(row, fmt.Sprintf("%s (%s)", dcs.ActiveState, dcs.SubState))
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

func Print(depl_name string) error {
	depl, err := ReadDeploymentByName(depl_name)
	if err != nil {
		return err
	}

	tbl := table.New("Name", depl.DeploymentName)
	tbl.AddRow("App", depl.AppName)
	tbl.AddRow("Instance", depl.InstanceName)
	tbl.AddRow("Part", depl.PartName)
	tbl.AddRow("Service Path", depl.ServiceDir)
	tbl.AddRow("Service Id", depl.ServiceId)

	tbl.Print()

	fmt.Println()

	var ctx = context.Background()
	sd, err := dbus.NewWithContext(ctx)
	if err != nil {
		return err
	}

	units, err := sd.ListUnitsByPatternsContext(ctx, nil, []string{
		service.ServiceUnit(depl.ServiceDir),
		service.ServiceConfigUnit(depl.ServiceDir),
		DeploymentUnit(depl.DeploymentName),
		DeploymentConfigUnit(depl.DeploymentName),
	})
	if err != nil {
		return err
	}

	tbl = table.New("", "Unit", "Loaded", "Active", "")
	for _, u := range units {
		var name string
		if u.Name == service.ServiceUnit(depl.ServiceDir) {
			name = "Service"
		} else if u.Name == service.ServiceConfigUnit(depl.ServiceDir) {
			name = "Reverse-Proxy Service Config"
		} else if u.Name == DeploymentUnit(depl.DeploymentName) {
			name = "Deployment"
		} else if u.Name == DeploymentConfigUnit(depl.DeploymentName) {
			name = "Reverse-Proxy Deployment Config"
		}
		tbl.AddRow(name, u.Name, u.LoadState, u.ActiveState, "("+u.SubState+")")
	}
	tbl.Print()

	return nil
}
