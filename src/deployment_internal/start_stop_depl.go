package deployment_internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/mildred/conductor.go/src/caddy"
	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/service"

	. "github.com/mildred/conductor.go/src/deployment"
)

// Note for deployment hooks, there could be different ways to hook:
//
// - using system commands or script, executed into a specific scope via systemd-run
//
// - using podman exec on a container (except pre-start and post-stop), this is
// already scoped in the podman pod.
//
// - or as a HTTP query to the pod IP address with a retry mechanism

func Prepare(deployment_name_or_dir string) error {
	ctx := context.Background()

	dir, deployment_name, err := deployment.DeploymentDirByName(deployment_name_or_dir, true)
	if err != nil {
		return err
	}

	log.Printf("prepare: Prepare deployment %s\n", dir)

	//
	// Create deployment config from service and run the templates
	//

	depl, err := ReadDeployment(".", deployment_name)
	if err != nil {
		return err
	}

	err = depl.TemplateAll(ctx)
	if err != nil {
		return err
	}

	log.Printf("prepare: Saving deployment config %s\n", ConfigName)
	err = depl.Save(ConfigName)
	if err != nil {
		return err
	}

	//
	// Configure systemd deployment
	//
	//

	// sd, err := utils.NewSystemdClient(ctx)
	// if err != nil {
	// 	return err
	// }

	// var props []dbus.Property
	// props = append(props, dbus.Property{"LogExtraFields", godbus.MakeVariant(fmt.Sprintf("CONDUCTOR_APP=%s", depl.AppName))})
	// props = append(props, dbus.Property{"LogExtraFields", godbus.MakeVariant(fmt.Sprintf("CONDUCTOR_INSTANCE=%s", depl.InstanceName))})
	// err = sd.SetUnitPropertiesContext(ctx, DeploymentUnit(depl.DeploymentName), false, props...)
	// if err != nil {
	// 	return err
	// }

	// err = sd.ReloadContext(ctx)
	// if err != nil {
	// 	return err
	// }

	//
	// Run the pre-start hooks (via systemd-run specific scope)
	//

	log.Printf("prepare: executing pre-start hooks...")
	err = depl.RunHooks(ctx, "pre-start", depl.Vars(), 60*time.Second)
	if err != nil {
		return err
	}

	log.Printf("prepare: Preparation sequence completed\n")

	return nil
}

func Start(deployment_name string, function bool) error {
	ctx := context.Background()

	depl, err := deployment.ReadDeploymentByName(deployment_name, true)
	if err != nil {
		return err
	}

	log.Printf("start: Loaded deployment %s, service %s-%s\n", depl.DeploymentName, depl.AppName, depl.InstanceName)

	if depl.Pod != nil && !function {
		return StartPod(ctx, depl)
	} else if depl.Function != nil {
		return StartFunction(ctx, depl, function)
	} else {
		return fmt.Errorf("Cannot start incompatible deployment")
	}
}

func Stop(deployment_name string, function bool) error {
	ctx := context.Background()

	//
	// Notify the deployment is stopping
	//

	_, err := daemon.SdNotify(false, daemon.SdNotifyStopping)
	if err != nil {
		return err
	}

	//
	// Load deployment configuration
	//

	depl, err := deployment.ReadDeploymentByName(deployment_name, true)
	if err != nil {
		return err
	}

	log.Printf("stop: Loaded deployment %s, service %s-%s\n", depl.DeploymentName, depl.AppName, depl.InstanceName)

	if depl.Pod != nil && !function {
		return StopPod(ctx, depl)
	} else if depl.Function != nil {
		return StopFunction(ctx, depl, function)
	} else {
		return fmt.Errorf("Cannot stop deployment: not a pod")
	}
}

func Cleanup(deployment_name_or_dir string) error {
	ctx := context.Background()

	dir, deployment_name, err := deployment.DeploymentDirByName(deployment_name_or_dir, true)
	if err != nil {
		return err
	}

	log.Printf("cleanup: Cleaning up %s\n", dir)

	//
	// Run the post-stop hooks (via systemd-run specific scope), just in case
	//

	depl, err := deployment.ReadDeployment(dir, deployment_name)
	if err != nil {
		return err
	}

	log.Printf("cleanup: Loaded deployment %s, service %s-%s\n", depl.DeploymentName, depl.AppName, depl.InstanceName)

	log.Printf("cleanup: executing post-stop hooks...")
	err = depl.RunHooks(ctx, "post-stop", depl.Vars(), 60*time.Second)
	if err != nil {
		return err
	}

	//
	// Remove deployment files
	//

	log.Printf("cleanup: Files left behind in %q\n", dir)
	log.Printf("cleanup: Cleanup sequence completed (deployment removal is up to the service)\n")
	return nil
}

func CaddyRegister(register bool, deployment_name_or_dir string) error {
	ctx := context.Background()
	var prefix = "register"
	if !register {
		prefix = "deregister"
	}

	depl, err := deployment.ReadDeploymentByName(deployment_name_or_dir, true)
	if err != nil {
		return fmt.Errorf("while loading deployment %+v, %v", ConfigName, err)
	}

	log.Printf("%s: Loaded deployment %s, service %s-%s\n", prefix, depl.DeploymentName, depl.AppName, depl.InstanceName)

	configs, err := depl.ProxyConfig(ctx)
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
			if retry {
				start = "reload"
			}

			fmt.Fprintf(os.Stderr, "+ systemctl %s %s %q\n", dirs.SystemdModeFlag(), start, unit_name)
			cmd := exec.CommandContext(ctx, "systemctl", dirs.SystemdModeFlag(), start, unit_name)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
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

	log.Printf("%s: Completed", prefix)
	return nil
}
