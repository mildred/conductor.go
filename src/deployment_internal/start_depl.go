package deployment_internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/parts"
	"github.com/mildred/conductor.go/src/service_util"
	"github.com/mildred/conductor.go/src/utils"

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

	dir, deployment_name, err := deployment.DeploymentDirByName(deployment_name_or_dir, true, false)
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
	// Bring addr up (if not already up by service)
	//

	err = service_util.BringAddrUp(ctx, depl.Service)
	if err != nil {
		return err
	}

	//
	// Run the pre-start hooks (via systemd-run specific scope)
	//

	vars, err := depl.Vars(ctx)
	if err != nil {
		return err
	}

	log.Printf("prepare: executing pre-start hooks...")
	err = depl.RunHooks(ctx, "pre-start", depl.PartName, vars, 60*time.Second)
	if err != nil {
		return err
	}

	log.Printf("prepare: Preparation sequence completed\n")

	return nil
}

func Start(deployment_name string, function bool) error {
	ctx := context.Background()

	depl, err := deployment.ReadDeploymentByName(deployment_name, true, false)
	if err != nil {
		return err
	}

	log.Printf("start: Loaded deployment %s, service %s-%s\n", depl.DeploymentName, depl.AppName, depl.InstanceName)

	if depl.PartType == parts.FunctionType && !function {
		// Nothing to start, this is started on demand
		return nil
	} else if function && depl.PartType != parts.FunctionType {
		return fmt.Errorf("cannot start function on %+v part", depl.PartType)
	}

	return depl.StartDeployment(ctx, depl)
}

func StartPost(deployment_name string, function bool) error {
	ctx := context.Background()

	depl, err := deployment.ReadDeploymentByName(deployment_name, true, false)
	if err != nil {
		return err
	}

	log.Printf("start: Loaded deployment %s, service %s-%s\n", depl.DeploymentName, depl.AppName, depl.InstanceName)

	if function && depl.PartType != parts.FunctionType {
		return fmt.Errorf("cannot start function on %+v part", depl.PartType)
	}

	return depl.StartPostDeployment(ctx, depl)
}

func StartDeploymentConfig(ctx context.Context, depl *Deployment) error {
	//
	// Add IP address to load balancer
	//
	// notify load balancer of ip address, hook into existing load balancer
	// configuration for the service and add the IP address
	//

	if !depl.HasProxyConfig() {
		log.Printf("No load-balancer to configure...\n")
		return nil
	}

	log.Printf("Adding deployment to load-balancer...\n")
	cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "start", DeploymentConfigUnit(depl.DeploymentName))

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := utils.RunWithJournal(ctx, cmd, DeploymentConfigUnit(depl.DeploymentName))
	if err != nil {
		err = fmt.Errorf("failed to add deployment to load-balancer, %v", err)
		SdNotifyOrLog(err.Error())
		return err
	}

	return nil
}
