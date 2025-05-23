package service_public

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"

	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/service"
)

func ReloadServices(inclusive bool) error {
	var ctx = context.Background()
	sd, err := utils.NewSystemdClient(ctx)
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
		fmt.Fprintf(os.Stderr, "+ systemctl %s disable --now %q\n", dirs.SystemdModeFlag(), unit)
		cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "disable", "--now", unit)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	for _, unit := range start_list {
		fmt.Fprintf(os.Stderr, "+ systemctl %s enable --now %q\n", dirs.SystemdModeFlag(), unit)
		cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "enable", "--now", unit)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	return nil
}

func Enable(definition_path string, now bool) error {
	unit, err := ServiceUnitByName(definition_path)
	if err != nil {
		return err
	}

	var cmd *exec.Cmd
	if now {
		fmt.Fprintf(os.Stderr, "+ systemctl %s enable --now %q\n", dirs.SystemdModeFlag(), unit)
		cmd = exec.Command("systemctl", dirs.SystemdModeFlag(), "enable", "--now", unit)
	} else {
		fmt.Fprintf(os.Stderr, "+ systemctl %s enable %q\n", dirs.SystemdModeFlag(), unit)
		cmd = exec.Command("systemctl", dirs.SystemdModeFlag(), "enable", unit)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Disable(definition_path string, now bool) error {
	unit, err := ServiceUnitByName(definition_path)
	if err != nil {
		return err
	}

	var cmd *exec.Cmd
	if now {
		fmt.Fprintf(os.Stderr, "+ systemctl %s disable --now %q\n", dirs.SystemdModeFlag(), unit)
		cmd = exec.Command("systemctl", dirs.SystemdModeFlag(), "disable", "--now", unit)
	} else {
		fmt.Fprintf(os.Stderr, "+ systemctl %s disable %q\n", dirs.SystemdModeFlag(), unit)
		cmd = exec.Command("systemctl", dirs.SystemdModeFlag(), "disable", unit)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Start(definition_path string) error {
	unit, err := ServiceUnitByName(definition_path)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ systemctl %s start %q\n", dirs.SystemdModeFlag(), unit)
	cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "start", unit)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Stop(definition_path string) error {
	unit, err := ServiceUnitByName(definition_path)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ systemctl %s stop %q\n", dirs.SystemdModeFlag(), unit)
	cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "stop", unit)
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

	var args []string = []string{dirs.SystemdModeFlag(), "restart"}
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
	sd, err := utils.NewSystemdClient(ctx)
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

	var args []string = []string{dirs.SystemdModeFlag()}
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
