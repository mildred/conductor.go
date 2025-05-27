package deployment_public

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/rodaine/table"

	"github.com/mildred/conductor.go/src/caddy"
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
	sd, err := utils.NewSystemdClient(ctx)
	if err != nil {
		return err
	}

	deployments, err := List(ListOpts{
		FilterServiceDir: settings.FilterServiceDir,
	})
	if err != nil {
		return err
	}

	patterns := []string{"conductor-deployment@*.service", "conductor-service@*.service", "conductor-deployment-config@*.service"}

	for _, depl := range deployments {
		patterns = append(patterns,
			CGIFunctionServiceUnit(depl.DeploymentName, "*"),
			CGIFunctionSocketUnit(depl.DeploymentName))
	}

	units, err := sd.ListUnitsByPatternsContext(ctx, nil, patterns)
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
				row = append(row, DeploymentUnit(depl.DeploymentName))
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
			row = append(row, DeploymentConfigUnit(depl.DeploymentName))
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

	for _, depl_name := range deployments {
		depl, err := ReadDeploymentByName(depl_name, true)
		if err != nil {
			return err
		}

		err = depl.TemplateAll()
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

type PrintSettings struct {
	ShowProxyLocation bool
}

func Print(depl_name string, settings PrintSettings) error {
	depl, err := ReadDeploymentByName(depl_name, true)
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

	configs, err := depl.ProxyConfig()
	if err != nil {
		fmt.Printf("Error getting proxy config: %v\n", err)
		fmt.Println()
	} else if len(configs) == 0 {
		fmt.Println("No reverse-proxy configuration")
		fmt.Println()
	} else {
		caddy, err := caddy.NewClient(depl.CaddyLoadBalancer.ApiEndpoint)
		if err != nil {
			return err
		}

		if settings.ShowProxyLocation {
			tbl = table.New("Reverse-Proxy configuration", "Present", "Dial")
		} else {
			tbl = table.New("Reverse-Proxy configuration", "Present", "Dial")
		}
		for _, config := range configs {
			cfg, err := caddy.GetConfig(config)
			if err != nil {
				return fmt.Errorf("while reading caddy config %+v in %+v, %v", config.Id, config.MountPoint, err)
			}

			if cfg.Present {
				dial, err := cfg.Dial()
				if err != nil {
					return err
				}

				if settings.ShowProxyLocation {
					tbl.AddRow(cfg.Id, cfg.MountPoint, "yes", dial)
				} else {
					tbl.AddRow(cfg.Id, "yes", dial)
				}
			} else {
				if settings.ShowProxyLocation {
					tbl.AddRow(cfg.Id, cfg.MountPoint, "no", "")
				} else {
					tbl.AddRow(cfg.Id, "no", "")
				}
			}
		}
		tbl.Print()
		fmt.Println()
	}

	var ctx = context.Background()
	sd, err := utils.NewSystemdClient(ctx)
	if err != nil {
		return err
	}

	units := utils.UnitStatusSpecs{
		&utils.UnitStatusSpec{
			Name:    "Service",
			Pattern: service.ServiceUnit(depl.ServiceDir),
		},
		&utils.UnitStatusSpec{
			Name:    "Reverse-Proxy Service Config",
			Pattern: service.ServiceConfigUnit(depl.ServiceDir),
		},
		&utils.UnitStatusSpec{
			Name:    "Deployment",
			Pattern: DeploymentUnit(depl.DeploymentName),
		},
		&utils.UnitStatusSpec{
			Name:    "Reverse-Proxy Deployment Config",
			Pattern: DeploymentConfigUnit(depl.DeploymentName),
		},
	}
	if depl.Function != nil {
		units = append(units, &utils.UnitStatusSpec{
			Name:    "Function Socket",
			Pattern: CGIFunctionSocketUnit(depl.DeploymentName),
		})
		if depl.Function.IsSingle() {
			units = append(units, &utils.UnitStatusSpec{
				Name:    "Function Instance",
				Pattern: CGIFunctionServiceUnitSingle(depl.DeploymentName),
			})
		} else {
			units = append(units, &utils.UnitStatusSpec{
				Name:    "Function Instances",
				Pattern: CGIFunctionServiceUnit(depl.DeploymentName, "*"),
			})
		}
	}
	if _, err := utils.UnitsStatus(ctx, sd, units); err != nil {
		return err
	}

	units.ToTable().Print()

	return nil
}

func stringsFromJSON(jsons []json.RawMessage) []string {
	var result []string
	for _, json := range jsons {
		result = append(result, string(json))
	}
	return result
}
