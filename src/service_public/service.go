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
	"github.com/rodaine/table"

	"github.com/mildred/conductor.go/src/deployment_public"
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

			service_dir, err = ServiceRealpath(service_dir)
			if err != nil {
				return err
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
		cmd := exec.Command("systemctl", "disable", "--now", unit)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	for _, unit := range start_list {
		fmt.Fprintf(os.Stderr, "+ systemctl enable --now %q\n", unit)
		cmd := exec.Command("systemctl", "enable", "--now", unit)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	return nil
}

func Start(definition_path string) error {
	unit, err := ServiceUnitByName(definition_path)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ systemctl start %q\n", unit)
	cmd := exec.Command("systemctl", "start", unit)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Stop(definition_path string) error {
	unit, err := ServiceUnitByName(definition_path)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ systemctl stop %q\n", unit)
	cmd := exec.Command("systemctl", "stop", unit)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Restart(definition_path string) error {
	unit, err := ServiceUnitByName(definition_path)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ systemctl restart %q\n", unit)
	cmd := exec.Command("systemctl", "restart", unit)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type PrintListSettings struct {
	Unit bool
}

func PrintList(settings PrintListSettings) error {
	var ctx = context.Background()
	sd, err := dbus.NewWithContext(ctx)
	if err != nil {
		return err
	}

	units, err := sd.ListUnitsByPatternsContext(ctx, nil, []string{"conductor-service@*.service"})
	if err != nil {
		return err
	}

	var list_service_dirs []string
	var list_services []*Service
	var list_status []dbus.UnitStatus
	var extra_cols []string

	for _, u := range units {
		service_dir := ServiceDirFromUnit(u.Name)
		if service_dir == "" {
			continue
		}

		service, err := LoadServiceDir(service_dir)
		if err != nil {
			return err
		}

		list_service_dirs = append(list_service_dirs, service_dir)
		list_services = append(list_services, service)
		list_status = append(list_status, u)

		if extra_cols == nil {
			extra_cols = service.DisplayServiceConfig
		} else {
			utils.IntersectHoles(&extra_cols, service.DisplayServiceConfig)
		}
	}

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

			service_dir, err = ServiceRealpath(service_dir)
			if err != nil {
				return err
			}

			if slices.Contains(list_service_dirs, service_dir) {
				continue
			}

			service, err := LoadServiceDir(service_dir)
			if err != nil {
				return err
			}

			list_service_dirs = append(list_service_dirs, service_dir)
			list_services = append(list_services, service)
			list_status = append(list_status, dbus.UnitStatus{})

			if extra_cols == nil {
				extra_cols = service.DisplayServiceConfig
			} else {
				utils.IntersectHoles(&extra_cols, service.DisplayServiceConfig)
			}
		}
	}

	extra_cols = utils.Compact(extra_cols...)

	row := []interface{}{"Name", "App", "Instance", "Enabled", "Active", "State"}
	if settings.Unit {
		row = append(row, "Unit")
	}
	for _, col := range extra_cols {
		row = append(row, col)
	}

	tbl := table.New(row...)

	for i, service := range list_services {
		u := list_status[i]

		name := service.Name
		if name == "" {
			name = service.BasePath
		}

		row = []interface{}{name, service.AppName, service.InstanceName, u.LoadState, u.ActiveState, u.SubState}
		if settings.Unit {
			row = append(row, u.Name)
		}
		for _, col := range extra_cols {
			row = append(row, service.Config[col])
		}
		tbl.AddRow(row...)
	}

	if len(list_services) > 0 {
		tbl.Print()
	}
	fmt.Printf("(%d services)\n", len(list_services))

	return nil
}

func PrintService(name string) error {
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

	var ctx = context.Background()
	sd, err := dbus.NewWithContext(ctx)
	if err != nil {
		return err
	}

	units, err := sd.ListUnitsByPatternsContext(ctx, nil, []string{ServiceUnit(service.BasePath), ServiceConfigUnit(service.BasePath)})
	if err != nil {
		return err
	}

	for _, u := range units {
		if u.Name == ServiceUnit(service.BasePath) {
			tbl.AddRow("Service", u.Name)
			tbl.AddRow("Service Enabled", u.LoadState)
			tbl.AddRow("Service Started", fmt.Sprintf("%s (%s)", u.ActiveState, u.SubState))
		} else if u.Name == ServiceConfigUnit(service.BasePath) {
			tbl.AddRow("Reverse-Proxy config", u.Name)
			tbl.AddRow("Reverse-Proxy config Enabled", u.LoadState)
			tbl.AddRow("Reverse-Proxy config Started", fmt.Sprintf("%s (%s)", u.ActiveState, u.SubState))
		}
	}

	for _, col := range service.DisplayServiceConfig {
		tbl.AddRow(col, service.Config[col])
	}

	tbl.Print()

	fmt.Println()

	deployment_public.PrintList(deployment_public.PrintListSettings{
		Unit:             true,
		FilterServiceDir: service.BasePath,
		QuietServiceInfo: true,
	})

	return nil
}

func PrintInspect(services ...string) error {
	if len(services) == 0 {
		return PrintInspect(".")
	}

	for _, name := range services {
		service, err := LoadServiceByName(name)
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

func ServiceSetConfig(filename string, config map[string]string) error {
	var service = map[string]interface{}{}

	//
	// Read service file if it exists
	//

	f, err := os.Open(filename)
	if err != nil && !os.IsNotExist(err) {
		return err
	} else if err == nil {
		err = func() error {
			defer f.Close()

			err := json.NewDecoder(f).Decode(&service)
			if err != nil {
				return err
			}

			return nil
		}()
		if err != nil {
			return err
		}
	}

	//
	// Add to Config
	//

	service_config_if, ok := service["config"]
	if !ok {
		service_config_if = map[string]interface{}{}
		service["config"] = service_config_if
	}

	service_config, ok := service_config_if.(map[string]interface{})
	if !ok {
		return fmt.Errorf("JSON key %q does not contain an object", "config")
	}

	for k, v := range config {
		service_config[k] = v
	}

	f, err = os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, os.ModePerm-0o111)
	if err != nil {
		return err
	}

	defer f.Close()

	err = json.NewEncoder(f).Encode(service)
	if err != nil {
		return err
	}

	return nil
}
