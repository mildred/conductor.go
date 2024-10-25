package deployment_internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"

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

	if depl.Pod != nil {
		return StartPod(ctx, depl)
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

	depl, err := LoadDeployment(ConfigName)
	if err != nil {
		return err
	}

	if depl.Pod != nil {
		return CaddyRegisterPod(ctx, depl, register, dir)
	} else {
		return fmt.Errorf("Cannot register deployment: not a pod")
	}
}
