package deployment_internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/mildred/conductor.go/src/caddy"
	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/tmpl"

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

func Prepare() error {
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	deployment := path.Base(cwd)

	log.Printf("prepare: Prepare deployment %s\n", cwd)

	//
	// Create deployment config from service and run the templates
	//

	depl, err := ReadDeployment(".", deployment)
	if err != nil {
		return err
	}

	err = depl.TemplateAll()
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

	// sd, err := dbus.NewWithContext(ctx)
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
	err = depl.RunHooks(ctx, "pre-start", 60*time.Second)
	if err != nil {
		return err
	}

	log.Printf("prepare: Preparation sequence completed\n")

	return nil
}

func Start() error {
	ctx := context.Background()
	depl, err := LoadDeployment(ConfigName)
	if err != nil {
		return err
	}

	log.Printf("start: Loaded deployment %s, service %s-%s\n", depl.DeploymentName, depl.AppName, depl.InstanceName)

	if depl.Pod != nil {
		return StartPod(ctx, depl)
	} else if depl.Function != nil {
		return StartFunction(ctx, depl)
	} else {
		return fmt.Errorf("Cannot start deployment: not a pod")
	}
}

func Stop() error {
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

	depl, err := LoadDeployment(ConfigName)
	if err != nil {
		return err
	}

	log.Printf("stop: Loaded deployment %s, service %s-%s\n", depl.DeploymentName, depl.AppName, depl.InstanceName)

	if depl.Pod != nil {
		return StopPod(ctx, depl)
	} else {
		return fmt.Errorf("Cannot stop deployment: not a pod")
	}
}

func Cleanup() error {
	ctx := context.Background()

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	log.Printf("cleanup: Cleaning up %s\n", cwd)

	//
	// Run the post-stop hooks (via systemd-run specific scope), just in case
	//

	depl, err := LoadDeployment(ConfigName)
	if err != nil {
		return err
	}

	log.Printf("cleanup: Loaded deployment %s, service %s-%s\n", depl.DeploymentName, depl.AppName, depl.InstanceName)

	log.Printf("cleanup: executing post-stop hooks...")
	err = depl.RunHooks(ctx, "post-stop", 60*time.Second)
	if err != nil {
		return err
	}

	//
	// Remove deployment files
	//

	log.Printf("cleanup: Files left behind in %q\n", cwd)
	log.Printf("cleanup: Cleanup sequence completed (deployment removal is up to the service)\n")
	return nil
}

func CaddyRegister(register bool, dir string) error {
	ctx := context.Background()
	var prefix = "register"
	if !register {
		prefix = "deregister"
	}

	depl, err := LoadDeployment(ConfigName)
	if err != nil {
		return err
	}

	log.Printf("%s: Loaded deployment %s, service %s-%s\n", prefix, depl.DeploymentName, depl.AppName, depl.InstanceName)

	if depl.ProxyConfigTemplate == "" {
		return nil
	}

	var configs []caddy.ConfigItem

	caddy, err := caddy.NewClient(depl.CaddyLoadBalancer.ApiEndpoint)
	if err != nil {
		return err
	}

	config, err := tmpl.RunTemplate(depl.ProxyConfigTemplate, depl.Vars())
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(config), &configs)
	if err != nil {
		return err
	}

	depl_desc := fmt.Sprintf("deployment %q", depl.DeploymentName)
	if depl.Pod != nil {
		depl_desc += fmt.Sprintf(" pod IP %s", depl.Pod.IPAddress)
	}

	if register {
		unit_name := fmt.Sprintf(service.ServiceConfigUnit(depl.ServiceDir))
		log.Printf("%s: Ensure the service config %s is registered", prefix, unit_name)

		fmt.Fprintf(os.Stderr, "+ systemctl start %q\n", unit_name)
		cmd := exec.CommandContext(ctx, "systemctl", "start", unit_name)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}

		log.Printf("register: Registering %s", depl_desc)
	} else {
		log.Printf("deregister: Deregistering %s", depl_desc)
	}

	err = caddy.Register(register, configs)
	if err != nil {
		return err
	}

	log.Printf("%s: Completed", prefix)
	return nil
}
