package service_public

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/PaesslerAG/jsonpath"
	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/rodaine/table"

	"github.com/mildred/conductor.go/src/service_util"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/service"
)

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
	All               bool
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
				found, err = sel.Match(service.Config[k].String())
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
	sd, err := utils.NewSystemdClient(ctx)
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
	var extra_cols []DisplayColumn

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
			utils.IntersectHolesFunc(&extra_cols, service.DisplayServiceConfig, DisplayColumnEqual)
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
				utils.IntersectHolesFunc(&extra_cols, service.DisplayServiceConfig, DisplayColumnEqual)
			}
		}
	}

	extra_cols = utils.CompactFunc(DisplayColumnIsEmpty, extra_cols...)

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
		row = append(row, col.Name)
	}

	tbl := table.New(row...)
	// rows = append(rows, row)

	var jsons []json.RawMessage
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	var num_excluded = 0

	for _, i := range list_ordered {
		service := list_services[i]
		u := list_status[i]

		var filtered_out = false
		if !settings.All {
			condition, _, err := service.EvaluateCondition(false)
			if err != nil {
				return err
			}
			filtered_out = !condition && u.LoadState == "" && u.ActiveState == ""
		}

		msg, err := Inspect(ctx, service, &InspectState{
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

			if !filtered_out && len(settings.FilterJSONPaths) > 0 {
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
				num_excluded += 1
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
					val, err := service.GetDisplayColumn(col, &service_util.ServiceCommandRunner{Service: service})
					if err != nil {
						return err
					}

					row = append(row, val)
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
		fmt.Printf("(%d services)\n", len(list_services)-num_excluded)
	}

	return nil
}
