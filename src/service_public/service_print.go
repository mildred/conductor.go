package service_public

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rodaine/table"

	"github.com/mildred/conductor.go/src/caddy"
	"github.com/mildred/conductor.go/src/deployment_public"
	"github.com/mildred/conductor.go/src/service_util"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/service"
)

type PrintSettings struct {
	ShowProxyLocation bool
}

func PrintService(name string, settings PrintSettings) error {
	service, err := LoadServiceByName(name)
	if err != nil {
		return err
	}

	if service.Name != "" {
		name = service.Name
	}

	tbl := table.New("Name", name)
	tbl.AddRow("App", service.AppName)
	tbl.AddRow("Instance", service.InstanceName)
	tbl.AddRow("Path", service.BasePath)
	tbl.AddRow("Filename", service.FileName)
	tbl.AddRow("Id", service.Id)
	tbl.Print()

	var ctx = context.Background()
	sd, err := utils.NewSystemdClient(ctx)
	if err != nil {
		return err
	}

	tbl = table.New("", "")
	for _, col := range service.DisplayServiceConfig {
		val, err := service.GetDisplayColumn(col, &service_util.ServiceCommandRunner{service})
		if err != nil {
			return err
		}

		tbl.AddRow(col, val)
	}
	tbl.Print()
	fmt.Println()

	units := utils.UnitStatusSpecs{
		&utils.UnitStatusSpec{
			Name:    "Service",
			Pattern: ServiceUnit(service.BasePath),
		},
		&utils.UnitStatusSpec{
			Name:    "Reverse-Proxy Service Config",
			Pattern: ServiceConfigUnit(service.BasePath),
		},
	}
	if _, err := utils.UnitsStatus(ctx, sd, units); err != nil {
		return err
	}

	units.ToTable().Print()
	fmt.Println()

	configs, err := service.ProxyConfig()
	if err != nil {
		fmt.Printf("Error getting proxy config: %v\n", err)
		fmt.Println()
	} else if len(configs) == 0 {
		fmt.Println("No reverse-proxy configuration")
		fmt.Println()
	} else {
		caddy, err := caddy.NewClient(service.CaddyLoadBalancer.ApiEndpoint)
		if err != nil {
			return err
		}

		if settings.ShowProxyLocation {
			tbl = table.New("Reverse-Proxy configuration", "Location", "Present", "Configuration (matchers)", "Upstreams")
		} else {
			tbl = table.New("Reverse-Proxy configuration", "Present", "Configuration (matchers)", "Upstreams")
		}
		for _, config := range configs {
			cfg, err := caddy.GetConfig(config)
			if err != nil {
				return fmt.Errorf("while reading caddy config %+v in %+v, %v", config.Id, config.MountPoint, err)
			}

			if cfg.Present {
				matches, err := cfg.MatchConfig()
				if err != nil {
					// ignore err
				}

				upstreams, err := cfg.UpstreamDials()
				if err != nil {
					// ignore err
				}

				if settings.ShowProxyLocation {
					tbl.AddRow(cfg.Id, cfg.MountPoint, "yes", strings.Join(stringsFromJSON(matches), "\n"), strings.Join(upstreams, "\n"))
				} else if cfg.Id == "" {
					tbl.AddRow(cfg.MountPoint, "yes", string(cfg.Config))
				} else {
					tbl.AddRow(cfg.Id, "yes", strings.Join(stringsFromJSON(matches), "\n"), strings.Join(upstreams, "\n"))
				}
			} else {
				if settings.ShowProxyLocation {
					tbl.AddRow(cfg.Id, cfg.MountPoint, "no", "")
				} else if cfg.Id == "" {
					tbl.AddRow(cfg.MountPoint, "?", string(cfg.Item.Config))
				} else {
					tbl.AddRow(cfg.Id, "no", "")
				}
			}
		}
		tbl.Print()
		fmt.Println()
	}

	deployment_public.PrintList(deployment_public.PrintListSettings{
		Unit:             true,
		FilterServiceDir: service.BasePath,
		ConfigStatus:     true,
		QuietServiceInfo: true,
	})

	return nil
}

func stringsFromJSON(jsons []json.RawMessage) []string {
	var result []string
	for _, json := range jsons {
		result = append(result, string(json))
	}
	return result
}
