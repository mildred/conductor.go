package service_public

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"

	"github.com/mildred/conductor.go/src/deployment_util"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/service"
)

func ReloadServices(inclusive bool, verbose bool) error {
	var ctx = context.Background()
	sd, err := utils.NewSystemdClient(ctx)
	if err != nil {
		return err
	}

	//
	// Reload services in well-known dirs
	//

	var seen_service_dirs []string
	var start_list []string
	var stop_list []string

	for _, dir := range ServiceDirs {
		entries, err := os.ReadDir(dir)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		log_prefix := log.Prefix()

		for _, ent := range entries {
			service_dir := path.Join(dir, ent.Name())
			log.SetPrefix(fmt.Sprintf("%s%s: ", log_prefix, service_dir))

			_, err = os.Stat(path.Join(service_dir, ConfigName))
			if err != nil && !os.IsNotExist(err) {
				log.Printf("ignore service, error while querying service file: %v\n", err)
				continue
			} else if err != nil {
				// ignore error, this is not a valid service dir
				continue
			}

			service_dir, err = ServiceRealpath(service_dir)
			if err != nil {
				return err
			}

			serv, err := LoadServiceDir(service_dir)
			if err != nil {
				log.Printf("ignore service, cannot load configuration: %v\n", err)
				continue
			}

			seen_service_dirs = append(seen_service_dirs, service_dir)

			if verbose {
				log.Printf("evaluate conditions...")
			}
			condition, disable, err := serv.EvaluateCondition(verbose)
			if err != nil {
				log.Printf("ignore service, cannot evaluate conditions: %v\n", err)
				continue
			}

			if disable || !condition {
				stop_list = append(stop_list, ServiceUnit(service_dir))
				if verbose && disable {
					log.Printf("service is disabled explicitely")
				} else if verbose && !condition {
					log.Printf("service is disabled because conditions do not match")
				}
			} else {
				start_list = append(start_list, ServiceUnit(service_dir))
				if verbose {
					log.Printf("service is enabled")
				}
			}
		}

		log.SetPrefix(log_prefix)
	}

	if !inclusive {
		existing_units, err := sd.ListUnitsByPatternsContext(ctx, nil, []string{"conductor-service@*.service"})
		if err != nil {
			return err
		}

		for _, u := range existing_units {
			service := ServiceDirFromUnit(u.Name)
			if service == "" || slices.Contains(seen_service_dirs, service) {
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

func setDisableConfig(definition_path string, disable bool) error {
	serv, err := LoadServiceDir(definition_path)
	if err != nil {
		return err
	}

	service, err := readConfigSetFile(serv.ConfigSetFile)
	if err != nil {
		return err
	}

	service["disable"] = disable

	err = writeConfigSetFile(serv.ConfigSetFile, service)
	if err != nil {
		return err
	}

	return nil
}

func Enable(definition_path string, now bool) error {
	err := setDisableConfig(definition_path, false)
	if err != nil {
		return err
	}

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
	err := setDisableConfig(definition_path, true)
	if err != nil {
		return err
	}

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

type StopOpts struct {
	NoBlock              bool
	RemoveAllDeployments bool
}

func Stop(definition_path string, opts StopOpts) error {
	ctx := context.Background()
	unit, err := ServiceUnitByName(definition_path)
	if err != nil {
		return err
	}

	service_dir, err := ServiceDirByName(definition_path)
	if err != nil {
		return err
	}

	var args []string = []string{dirs.SystemdModeFlag(), "stop"}
	if opts.NoBlock {
		args = append(args, "--no-block")
	}
	args = append(args, unit)

	fmt.Fprintf(os.Stderr, "+ systemctl %s\n", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	errs := cmd.Run()

	if opts.RemoveAllDeployments {
		deployments, err := deployment_util.List(deployment_util.ListOpts{
			FilterServiceDir: service_dir,
		})
		if err != nil {
			return err
		}

		for _, d := range deployments {
			fmt.Fprintf(os.Stderr, "+ conductor deployment rm %s\n", d.DeploymentName)
			err := deployment_util.RemoveTimeout(ctx, d.DeploymentName, 0, 0)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ERROR removing deployment %s: %v\n", d.DeploymentName, err)
			} else {
				fmt.Fprintf(os.Stderr, "SUCCESS removing deployment %s\n", d.DeploymentName)
			}
			errs = errors.Join(errs, err)
		}
	}

	return errs
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

func readConfigSetFile(filename string) (map[string]interface{}, error) {
	var service = map[string]interface{}{}

	f, err := os.Open(filename)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if err == nil {
		defer f.Close()

		err = json.NewDecoder(f).Decode(&service)
		if err != nil {
			return nil, err
		}

		return service, nil
	} else {
		return service, nil
	}
}

func writeConfigSetFile(filename string, service map[string]interface{}) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm-0o111)
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

func ServiceSetConfig(filename string, config map[string]string) error {
	//
	// Read service file if it exists
	//

	service, err := readConfigSetFile(filename)
	if err != nil {
		return err
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

	return writeConfigSetFile(filename, service)
}
