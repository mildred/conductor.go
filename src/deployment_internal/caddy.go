package deployment_internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mildred/conductor.go/src/caddy"
	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/deployment"
)

func CaddyRegister(register bool, deployment_name_or_dir, config_name string) error {
	ctx := context.Background()
	var prefix, hook_name string
	if register {
		prefix = "register"
		hook_name = "register"
	} else {
		prefix = "deregister"
		hook_name = "deregister"
	}

	depl, err := deployment.ReadDeploymentByName(deployment_name_or_dir, true, false)
	if err != nil {
		return fmt.Errorf("while loading deployment %+v, %v", ConfigName, err)
	}

	log.Printf("%s: Loaded deployment %s, service %s-%s\n", prefix, depl.DeploymentName, depl.AppName, depl.InstanceName)

	vars, err := depl.Vars(ctx)
	if err != nil {
		return err
	}

	err = depl.RunHooks(ctx, "pre-"+hook_name, "", vars, 60*time.Second)
	if err != nil {
		return err
	}

	configs, err := depl.ProxyConfig(ctx, config_name)
	if err != nil {
		return fmt.Errorf("getting the proxy config, %v", err)
	} else if len(configs) == 0 {
		return nil
	}

	caddy, err := caddy.NewClient(depl.CaddyLoadBalancer.ApiEndpoint, time.Duration(depl.CaddyLoadBalancer.Timeout))
	if err != nil {
		return fmt.Errorf("while connecting to Caddy, %v", err)
	}

	depl_desc := fmt.Sprintf("deployment %q", depl.DeploymentName)
	if depl.Pod != nil {
		depl_desc += fmt.Sprintf(" pod IP %s", depl.Pod.IPAddress)
	}

	retry := false
	for {
		if register {
			unit_name := fmt.Sprintf(service.ServiceConfigUnit(depl.ServiceDir))
			log.Printf("%s: Ensure the service config %s is registered", prefix, unit_name)

			start := "start"
			// TODO: start in foreground by default and let systemd acknoledge the
			// started state

			if retry {
				start = "reload"
				// TODO: reload in foreground by default instead of forwarding to
				// systemd
			}

			cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), start, unit_name)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = utils.RunWithJournal(ctx, cmd, unit_name)
			if err != nil {
				err = fmt.Errorf("while %sing the systemd unit %+v, %v", start, unit_name, err)
				SdNotifyOrLog(err.Error())
				if retry {
					return err
				} else {
					log.Printf("ERROR, will retry: %v", err)
					retry = true
					continue
				}
			}

			log.Printf("register: Registering %s", depl_desc)
		} else {
			log.Printf("deregister: Deregistering %s", depl_desc)
		}

		err = caddy.Register(ctx, register, configs)
		if err != nil {
			err = fmt.Errorf("while registering Caddy config, %v", err)
			SdNotifyOrLog(err.Error())
			if retry {
				return err
			} else {
				log.Printf("ERROR, will retry: %v", err)
				retry = true
				continue
			}
		}
		break
	}

	vars, err = depl.Vars(ctx)
	if err != nil {
		return err
	}

	err = depl.RunHooks(ctx, "pre-"+hook_name, "", vars, 60*time.Second)
	if err != nil {
		return err
	}

	log.Printf("%s: Completed", prefix)
	return nil
}
