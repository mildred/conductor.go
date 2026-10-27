package service_internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/mildred/conductor.go/src/deployment_public"
	"github.com/mildred/conductor.go/src/deployment_util"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/service"
)

func Stop(ctx context.Context, service_name string) error {
	defer utils.AddPrefix(log.Default(), "[%s] stop: ", service_name)()

	//
	// Notify stop in progress
	//

	_, err := daemon.SdNotify(false, daemon.SdNotifyStopping)
	if err != nil {
		return err
	}

	//
	// Fetch service config
	//

	service, err := LoadServiceByName(service_name)
	if err != nil {
		return err
	}

	//
	// Run pre-stop-service hook
	//

	err = service.RunHooks(ctx, "pre-stop-service", "", service.Vars(), 60*time.Second)
	if err != nil {
		return err
	}

	//
	// Stop MAINPID monitoring
	//

	if mainpid := os.Getenv("MAINPID"); mainpid != "" {
		log.Printf("Sending SIGTERM to pid=%s\n", mainpid)
		main_pid, err := strconv.ParseInt(mainpid, 10, 0)
		if err != nil {
			return fmt.Errorf("MAINPID=%s is not a PID number, %v", mainpid, err)
		}
		proc, err := os.FindProcess(int(main_pid))
		if err != nil {
			return err
		}
		err = proc.Signal(syscall.SIGTERM)
		if err != nil {
			return err
		}
	}

	//
	// Stop all deployments
	//

	log.Printf("Stopping all deployments...\n")

	deployments, err := deployment_util.List(deployment_util.ListOpts{
		FilterServiceDir: service.BasePath,
	})
	if err != nil {
		return err
	}

	for _, d := range deployments {
		log.Printf("Stopping deployment %s...\n", d.DeploymentName)

		ctx1, cancel := context.WithCancel(ctx)
		go utils.ExtendTimeout(ctx1, 60*time.Second)

		func() {
			defer cancel()
			deployment_public.Stop(ctx, d.DeploymentName)
		}()
	}

	//
	// Run post-stop-service hook
	//

	err = service.RunHooks(ctx, "post-stop-service", "", service.Vars(), 60*time.Second)
	if err != nil {
		return err
	}

	log.Printf("Stop sequence completed\n")
	return nil
}

func Cleanup(ctx context.Context, service_name string) error {
	errs := Stop(ctx, service_name)

	//
	// Remove temp files if there is any
	//

	log.Printf("cleanup: Cleaning up...\n")

	err := RemoveAllDeployments(ctx, service_name)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	log.Printf("cleanup: Cleanup sequence completed\n")
	return errs
}

func RemoveAllDeployments(ctx context.Context, service_name string) error {
	service_dir, err := ServiceDirByName(service_name)
	if err != nil {
		return err
	}

	deployments, err := deployment_util.List(deployment_util.ListOpts{
		FilterServiceDir: service_dir,
	})
	if err != nil {
		return err
	}

	var errs error

	for _, d := range deployments {
		fmt.Fprintf(os.Stderr, "+ conductor deployment rm %s\n", d.DeploymentName)
		err := deployment_util.RemoveTimeout(ctx, d.DeploymentName, 5*time.Second, 5*time.Second)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR removing deployment %s: %v\n", d.DeploymentName, err)
		}
		errs = errors.Join(errs, err)
	}
	return nil
}
