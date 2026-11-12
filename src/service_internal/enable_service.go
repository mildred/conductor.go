package service_internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/install"
	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/utils"
)

func EnableService(ctx context.Context, svc *service.Service, now bool) error {
	err := svc.RunHooks(ctx, "pre-enable-service", "", svc.Vars(), 60*time.Second)
	if err != nil {
		return err
	}

	err = setLoadBalancerService(ctx, svc)
	if err != nil {
		return err
	}

	err = svc.RunHooks(ctx, "post-enable-service", "", svc.Vars(), 60*time.Second)
	if err != nil {
		return err
	}

	background := svc.IsDynamicDeployment()

	if now && !background {
		err := StartOrReload(svc.BasePath, StartOrReloadOpts{
			StartConfig:     true,
			ExitWhenStarted: true,
		})
		if err != nil {
			return err
		}
	}

	unit, err := service.ServiceUnitByName(svc.BasePath)
	if err != nil {
		return err
	}

	var cmd *exec.Cmd
	if now {
		cmd = utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "enable", "--now", unit)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return utils.RunWithJournal(ctx, cmd, unit)
	} else {
		cmd = utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "enable", unit)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
}

func DisableService(ctx context.Context, svc *service.Service, now bool) error {
	unit, err := service.ServiceUnitByName(svc.BasePath)
	if err != nil {
		return err
	}

	err = svc.RunHooks(ctx, "pre-disable-service", "", svc.Vars(), 60*time.Second)
	if err != nil {
		return err
	}

	var cmd *exec.Cmd
	if now {
		cmd = utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "disable", "--now", unit)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = utils.RunWithJournal(ctx, cmd, unit)
		if err != nil {
			return err
		}
	} else {
		cmd = utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "disable", unit)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	err = svc.RunHooks(ctx, "post-disable-service", "", svc.Vars(), 60*time.Second)
	if err != nil {
		return err
	}

	return nil
}

func setLoadBalancerService(ctx context.Context, svc *service.Service) error {
	sd, err := utils.NewSystemdClient(ctx)
	if err != nil {
		return err
	}

	config_unit := service.ServiceConfigUnit(svc.BasePath)
	config_path := dirs.Join(dirs.ConfigHome, "systemd", dirs.SystemdMode(), config_unit)

	if svc.LoadBalancerService == nil {
		// Remove drop-in
		fmt.Fprintf(os.Stderr, "+ rm -f %q\n", config_path)
		err := os.RemoveAll(config_path)
		if err != nil {
			return err
		}

		return sd.ReloadContext(ctx)

	} else {

		load_balancer_service := *svc.LoadBalancerService

		if !strings.HasSuffix(load_balancer_service, ".service") && load_balancer_service != "" {
			load_balancer_service, err = service.ServiceUnitByName(load_balancer_service)
			if err != nil {
				return err
			}
		}

		conf := strings.ReplaceAll(install.ConductorServiceConfigService, "caddy.service", load_balancer_service)

		actual_conf, _ := os.ReadFile(config_path)
		if string(actual_conf) == conf {
			return nil
		}

		err := os.MkdirAll(path.Dir(config_path), 0755)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "+ install --mode=0644 -t %q %q\n", path.Dir(config_path), path.Base(config_path))

		err = os.WriteFile(config_path, []byte(conf), 0644)
		if err != nil {
			return err
		}

		return sd.ReloadContext(ctx)
	}
}
