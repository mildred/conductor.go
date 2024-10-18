package service_public

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"slices"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/gandarez/go-realpath"
	"github.com/rodaine/table"

	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/service"
)

func Reload(inclusive bool) error {
	var ctx = context.Background()
	sd, err := dbus.NewWithContext(ctx)
	if err != nil {
		return err
	}

	//
	// Reload services in well-known dirs
	//

	var service_dirs []string
	var start_list []string
	var stop_list []string

	for _, dir := range ServiceDirs {
		entries, err := os.ReadDir(dir)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		for _, ent := range entries {
			service_dir := path.Join(dir, ent.Name())
			_, err = os.Stat(path.Join(service_dir, ConfigName))
			if err != nil && !os.IsNotExist(err) {
				return err
			} else if err != nil {
				// ignore error, this is not a valid service dir
				continue
			}

			service_dirs = append(service_dirs, service_dir)
			start_list = append(start_list, ServiceUnit(service_dir))
		}
	}

	if !inclusive {
		existing_units, err := sd.ListUnitsByPatternsContext(ctx, nil, []string{"conductor-service@*.service"})
		if err != nil {
			return err
		}

		for _, u := range existing_units {
			service := ServiceDirFromUnit(u.Name)
			if service == "" || slices.Contains(service_dirs, service) {
				continue
			}

			stop_list = append(stop_list, u.Name)
		}
	}

	for _, unit := range stop_list {
		fmt.Fprintf(os.Stderr, "+ systemctl disable --now %q\n", unit)
		err = exec.Command("systemctl", "disable", "--now", unit).Run()
		if err != nil {
			return err
		}
	}

	for _, unit := range start_list {
		fmt.Fprintf(os.Stderr, "+ systemctl enable --now %q\n", unit)
		err = exec.Command("systemctl", "enable", "--now", unit).Run()
		if err != nil {
			return err
		}
	}

	return nil
}

func Start(definition_path string) error {
	path, err := realpath.Realpath(definition_path)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ systemctl start %q\n", ServiceUnit(path))
	return exec.Command("systemctl", "start", ServiceUnit(path)).Run()
}

func PrintList() error {
	var ctx = context.Background()
	sd, err := dbus.NewWithContext(ctx)
	if err != nil {
		return err
	}

	units, err := sd.ListUnitsByPatternsContext(ctx, nil, []string{"conductor-service@*.service"})
	if err != nil {
		return err
	}

	var list_services []*Service
	var list_status []dbus.UnitStatus
	var extra_cols []string

	for _, u := range units {
		service_dir := ServiceDirFromUnit(u.Name)
		if service_dir == "" {
			continue
		}

		service, err := LoadServiceDir(service_dir, true)
		if err != nil {
			return err
		}

		list_services = append(list_services, service)
		list_status = append(list_status, u)

		if extra_cols == nil {
			extra_cols = service.DisplayServiceConfig
		} else {
			utils.IntersectHoles(&extra_cols, service.DisplayServiceConfig)
		}
	}

	extra_cols = utils.Compact(extra_cols...)

	row := []interface{}{"App", "Instance", "Enabled", "Active", "State"}
	for _, col := range extra_cols {
		row = append(row, col)
	}
	tbl := table.New(row...)

	for i, service := range list_services {
		u := list_status[i]

		row = []interface{}{service.AppName, service.InstanceName, u.LoadState, u.ActiveState, u.SubState}
		for _, col := range extra_cols {
			row = append(row, service.Config[col])
		}
		tbl.AddRow(row...)
	}

	tbl.Print()
	return nil
}

func PrintInspect(fix_paths bool, services ...string) error {
	if len(services) == 0 {
		return PrintInspect(fix_paths, ".")
	}

	for _, dir := range services {
		service, err := LoadServiceDir(dir, fix_paths)
		if err != nil {
			return err
		}

		err = json.NewEncoder(os.Stdout).Encode(service)
		if err != nil {
			return err
		}
	}
	return nil
}
