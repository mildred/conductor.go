package service_public

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/PaesslerAG/jsonpath"
	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/rodaine/table"

	"github.com/mildred/conductor.go/src/deployment_public"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/service"
)

func ReloadServices(inclusive bool) error {
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

type RestartOpts struct {
	NoBlock bool
}

func Restart(definition_path string, opts RestartOpts) error {

	unit, err := ServiceUnitByName(definition_path)
	if err != nil {
		return err
	}

	var args []string = []string{"restart"}
	if opts.NoBlock {
		args = append(args, "--no-block")

	}
	args = append(args, unit)

	fmt.Fprintf(os.Stderr, "+ systemctl %s\n", strings.Join(args, " "))
	cmd := exec.Command("systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type ReloadOpts struct {
	NoBlock bool
}

func Reload(definition_path string, opts ReloadOpts) error {
	var active bool

	var ctx = context.Background()
	sd, err := dbus.NewWithContext(ctx)
	if err != nil {
		return err
	}

	unit, err := ServiceUnitByName(definition_path)
	if err != nil {
		return err
	}

	units, err := sd.ListUnitsByPatternsContext(ctx, nil, []string{unit})
	if err != nil {
		return err
	}

	for _, u := range units {
		if u.Name != unit {
			continue
		}

		active = u.ActiveState == "active"
	}

	var args []string
	if active {
		args = append(args, "reload")
	} else {
		args = append(args, "reload-or-restart")
	}
	if opts.NoBlock {
		args = append(args, "--no-block")

	}
	args = append(args, unit)

	fmt.Fprintf(os.Stderr, "+ systemctl %s\n", strings.Join(args, " "))
	cmd := exec.Command("systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type Selector struct {
	Selector string
	Value    string
	Negate   bool
}

func (sel *Selector) Match(val string) (bool, error) {
	res, err := sel.match_val(val)
	if sel.Negate {
		res = !res
	}
	return res, err
}

func (sel *Selector) match_val(val string) (bool, error) {
	if sel.Selector == "=" {
		return val == sel.Value, nil
	} else if sel.Selector == "*=" {
		re, err := regexp.Compile(sel.Value)
		if err != nil {
			return false, err
		}
		return re.Match([]byte(val)), nil
	} else if sel.Selector == "^=" {
		return strings.HasPrefix(val, sel.Value), nil
	} else if sel.Selector == "$=" {
		return strings.HasSuffix(val, sel.Value), nil
	} else if sel.Selector == "~=" {
		return slices.Contains(strings.Split(val, " "), sel.Value), nil
	} else if sel.Selector == "~json=" {
		var slice []interface{}
		err := json.Unmarshal([]byte(val), &slice)
		if err != nil {
			return false, nil
		}
		for _, v := range slice {
			if str, ok := v.(string); ok && str == sel.Value {
				return true, nil
			}
		}
		return false, nil
	} else if sel.Selector == "~jsonpath=" || sel.Selector == "jsonpath" {
		var value interface{}
		err := json.Unmarshal([]byte(val), &value)
		if err != nil {
			return false, nil
		}
		jpvalue, err := jsonpath.Get(sel.Value, value)
		if err != nil {
			return false, err
		}
		bvalue, ok := jpvalue.(bool)
		if !ok {
			return false, fmt.Errorf("JSONPath %q did not return a boolean but %+v", sel.Value, jpvalue)
		}
		return bvalue, nil
	} else {
		return false, fmt.Errorf("Unknown selector %s", sel.Selector)
	}
}

type PrintListSettings struct {
	Unit              bool
	FilterApplication string
	FilterConfig      map[string][]Selector
	FilterJSONPaths   []string
	JSON              bool
	JSONs             bool
	CSV               bool
	CSVSeparator      string
	JSONPath          string
	ResumeBefore      *Selector
	ResumeAfter       *Selector
	StopBefore        *Selector
	StopAfter         *Selector
}

func PrintListFilter(service *Service, settings PrintListSettings) (bool, error) {
	var err error
	if settings.FilterApplication != "" && settings.FilterApplication != service.AppName {
		return false, nil
	}

	if settings.FilterConfig != nil {
		for k, v := range settings.FilterConfig {
			found := false
			for _, sel := range v {
				found, err = sel.Match(service.Config[k])
				if err != nil {
					return false, err
				}
				if found {
					break
				}
			}
			if !found {
				return false, nil
			}
		}
	}

	return true, nil
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

		filter, err := PrintListFilter(service, settings)
		if err != nil {
			return err
		} else if !filter {
			continue
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

			filter, err := PrintListFilter(service, settings)
			if err != nil {
				return err
			} else if !filter {
				continue
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

	var list_ordered []int
	for i := range list_services {
		list_ordered = append(list_ordered, i)
	}

	sort.SliceStable(list_ordered, func(i, j int) bool {
		ii, jj := list_ordered[i], list_ordered[j]
		x, y := list_services[ii], list_services[jj]
		return (x.AppName < y.AppName) ||
			(x.AppName == y.AppName && x.InstanceName < y.InstanceName)
	})

	resume := settings.ResumeBefore == nil && settings.ResumeAfter == nil

	var rows [][]interface{}
	row := []interface{}{"Name", "App", "Instance", "Enabled", "Active", "State"}
	if settings.Unit {
		row = append(row, "Unit")
	}
	for _, col := range extra_cols {
		row = append(row, col)
	}

	tbl := table.New(row...)
	// rows = append(rows, row)

	var jsons []json.RawMessage
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	for _, i := range list_ordered {
		service := list_services[i]
		u := list_status[i]

		msg, err := Inspect(service, &InspectState{
			UnitStatus: u,
		})
		if err != nil {
			return err
		}

		var jsonvalue interface{}
		if settings.JSONPath != "" || len(settings.FilterJSONPaths) > 0 {
			err := json.Unmarshal(msg, &jsonvalue)
			if err != nil {
				return err
			}
		}

		if !resume && settings.ResumeBefore != nil {
			resume, err = settings.ResumeBefore.Match(string(msg))
			if err != nil {
				return err
			}
		}

		if resume {
			if settings.StopBefore != nil {
				stop, err := settings.StopBefore.Match(string(msg))
				if err != nil {
					return err
				} else if stop {
					break
				}
			}

			filtered_out := false
			if len(settings.FilterJSONPaths) > 0 {
				filtered_out = true
				for _, p := range settings.FilterJSONPaths {
					jpvalue, err := jsonpath.Get(p, jsonvalue)
					if err != nil {
						return fmt.Errorf("while executing JSONPath %q for filtering, %v", p, err)
					}
					bvalue, ok := jpvalue.(bool)
					if !ok {
						return fmt.Errorf("JSONPath %q did not return a boolean but %+v", p, jpvalue)
					}
					if bvalue {
						filtered_out = false
						break
					}
				}
			}

			if filtered_out {
				// do nothing
			} else if settings.JSONPath != "" {
				jpvalue, err := jsonpath.Get(settings.JSONPath, jsonvalue)
				if err != nil {
					return err
				}
				fmt.Printf("%v\n", jpvalue)
			} else if settings.JSONs {
				err = enc.Encode(msg)
				if err != nil {
					return err
				}
			} else if settings.JSON {
				jsons = append(jsons, msg)
			} else {
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
				rows = append(rows, row)
			}

			if settings.StopAfter != nil {
				stop, err := settings.StopAfter.Match(string(msg))
				if err != nil {
					return err
				} else if stop {
					break
				}
			}
		}

		if !resume && settings.ResumeAfter != nil {
			resume, err = settings.ResumeAfter.Match(string(msg))
			if err != nil {
				return err
			}
		}
	}

	if settings.JSON {
		err = enc.Encode(jsons)
		if err != nil {
			return err
		}
	} else if settings.CSV {
		sep := settings.CSVSeparator
		if sep == "" {
			sep = ","
		}
		for _, r := range rows {
			row := []string{}
			for _, c := range r {
				row = append(row, c.(string))
			}
			fmt.Println(strings.Join(row, sep))
		}
	} else if !settings.JSONs && settings.JSONPath == "" {
		if len(list_services) > 0 {
			tbl.Print()
		}
		fmt.Printf("(%d services)\n", len(list_services))
	}

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
		ConfigStatus:     true,
		QuietServiceInfo: true,
	})

	return nil
}

type InspectState struct {
	UnitStatus dbus.UnitStatus `json:"unit_status"`
}

func Inspect(service *Service, state *InspectState) (json.RawMessage, error) {
	exported := struct {
		*Service
		Inherit       []*InheritFile `json:"inherit"`
		State         *InspectState  `json:"_state,omitempty"`
		BasePath      string         `json:"_base_path"`
		FileName      string         `json:"_file_name"`
		ConfigSetFile string         `json:"_config_set_file"`
		Name          string         `json:"_name"`
		Id            string         `json:"_id"`
	}{
		Service:       service,
		State:         state,
		Inherit:       service.Inherit.Inherit,
		BasePath:      service.BasePath,
		FileName:      service.FileName,
		ConfigSetFile: service.ConfigSetFile,
		Name:          service.Name,
		Id:            service.Id,
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	err := enc.Encode(exported)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
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

		msg, err := Inspect(service, nil)
		if err != nil {
			return err
		}

		fmt.Println(string(msg))
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

	f, err = os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm-0o111)
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
