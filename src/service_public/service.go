package service_public

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/service_internal"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/service"
)

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

func Enable(ctx context.Context, name string, now bool) error {
	definition_path, err := ServiceDirByName(name)
	if err != nil {
		return err
	}

	err = setDisableConfig(definition_path, false)
	if err != nil {
		return err
	}

	svc, err := LoadServiceByName(definition_path)
	if err != nil {
		return err
	}

	return service_internal.EnableService(context.Background(), svc, now)
}

func Disable(ctx context.Context, name string, now bool) error {
	definition_path, err := ServiceDirByName(name)
	if err != nil {
		return err
	}

	err = setDisableConfig(definition_path, true)
	if err != nil {
		return err
	}

	svc, err := LoadServiceByName(definition_path)
	if err != nil {
		return err
	}

	return service_internal.DisableService(context.Background(), svc, now)
}

type StartOpts struct {
	Foreground bool
	Background bool
}

func Start(ctx context.Context, definition_path string, opts StartOpts) error {
	if opts.Foreground && opts.Background {
		return fmt.Errorf("cannot specify both backgroudn and foreground operation")
	}

	svc, err := LoadServiceByName(definition_path)
	if err != nil {
		return err
	}

	background := opts.Background || svc.IsDynamicDeployment()
	if opts.Foreground {
		background = false
	}

	unit, err := ServiceUnitByName(definition_path)
	if err != nil {
		return err
	}

	if !background {
		err = service_internal.StartOrReload(definition_path, service_internal.StartOrReloadOpts{
			ExitWhenStarted: true,
			StartConfig:     true,
		})
		if err != nil {
			return err
		}
	}

	cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "start", unit)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return utils.RunWithJournal(ctx, cmd, unit)
}

type StopOpts struct {
	Foreground           bool
	Background           bool
	NoBlock              bool
	RemoveAllDeployments bool
	KillUnstoppable      bool
}

func Stop(ctx context.Context, definition_path string, opts StopOpts) error {
	unit, err := ServiceUnitByName(definition_path)
	if err != nil {
		return err
	}

	svc, err := LoadServiceByName(definition_path)
	if err != nil {
		return err
	}

	background := opts.Background || svc.IsDynamicDeployment()
	if opts.Foreground {
		background = false
	}

	if !background {
		err = service_internal.Stop(ctx, definition_path, service_internal.StopOpts{
			KillUnstoppable: opts.KillUnstoppable,
		})
		if err != nil {
			return err
		}
		defer utils.AddPrefix(log.Default(), "[%s] stop: ", definition_path)()
	}

	var args []string = []string{dirs.SystemdModeFlag(), "stop"}
	if opts.NoBlock {
		args = append(args, "--no-block")
	}
	args = append(args, unit)

	cmd := utils.LogCommand(ctx, "systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	var errs error
	if opts.NoBlock {
		errs = cmd.Run()
	} else {
		errs = utils.RunWithJournal(ctx, cmd, unit)
	}

	if opts.RemoveAllDeployments {
		err := service_internal.RemoveAllDeployments(ctx, definition_path)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return errs
}

type KillOpts struct {
	KillSignal string
}

func Kill(ctx context.Context, definition_path string, opts KillOpts) error {
	err := service_internal.Kill(ctx, definition_path, opts.KillSignal)
	if err != nil {
		return err
	}

	return nil
}

type RestartOpts struct {
	Foreground bool
	Background bool
	NoBlock    bool
}

func Restart(ctx context.Context, definition_path string, opts RestartOpts) error {
	if opts.Foreground && opts.Background {
		return fmt.Errorf("cannot specify both backgroudn and foreground operation")
	}

	svc, err := LoadServiceByName(definition_path)
	if err != nil {
		return err
	}

	background := opts.Background || svc.IsDynamicDeployment()
	if opts.Foreground {
		background = false
	}

	if !background && opts.NoBlock {
		return fmt.Errorf("Cannot reload service in foreground without blocking")
	}

	unit, err := ServiceUnitByName(definition_path)
	if err != nil {
		return err
	}

	if background {

		var args []string = []string{dirs.SystemdModeFlag(), "restart"}
		if opts.NoBlock {
			args = append(args, "--no-block")
		}
		args = append(args, unit)

		cmd := utils.LogCommand(ctx, "systemctl", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if opts.NoBlock {
			return cmd.Run()
		} else {
			return utils.RunWithJournal(ctx, cmd, unit)
		}

	} else {
		err := Stop(ctx, definition_path, StopOpts{
			RemoveAllDeployments: true,
		})
		if err != nil {
			return err
		}

		return Start(ctx, definition_path, StartOpts{
			Foreground: true,
		})
	}
}

type ReloadOpts struct {
	Foreground bool
	Background bool
	NoBlock    bool
}

func Reload(ctx context.Context, definition_path string, opts ReloadOpts) error {
	if opts.Foreground && opts.Background {
		return fmt.Errorf("cannot specify both backgroudn and foreground operation")
	}

	svc, err := LoadServiceByName(definition_path)
	if err != nil {
		return err
	}

	background := opts.Background || svc.IsDynamicDeployment()
	if opts.Foreground {
		background = false
	}

	if !background && opts.NoBlock {
		return fmt.Errorf("Cannot reload service in foreground without blocking")
	}

	if !background {
		return service_internal.StartOrReload(definition_path, service_internal.StartOrReloadOpts{
			Restart:   true,
			ReuseDepl: true,
		})
	}

	var active bool

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

	cmd := utils.LogCommand(ctx, "systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if opts.NoBlock {
		return cmd.Run()
	} else {
		return utils.RunWithJournal(ctx, cmd, unit)
	}
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
