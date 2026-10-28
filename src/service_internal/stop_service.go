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

type StopOpts struct {
	KillUnstoppable    bool
	UnstoppableTimeout time.Duration
}

func Stop(ctx context.Context, service_name string, opts StopOpts) error {
	defer utils.AddPrefix(log.Default(), "[%s] stop: ", service_name)()

	if opts.UnstoppableTimeout == 0 {
		opts.UnstoppableTimeout = 5 * time.Second
	}

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

			if opts.KillUnstoppable {
				stopped := false

				func() {
					stopCtx, stopCancel := context.WithTimeout(ctx, opts.UnstoppableTimeout)
					defer stopCancel()
					err := deployment_public.Stop(stopCtx, d.DeploymentName)
					if err != nil {
						log.Printf("Stop deployment %s error: %v", d.DeploymentName, err)
					}
					stopped = stopCtx.Err() == nil
				}()

				if !stopped {
					func() {
						killCtx, killCancel := context.WithTimeout(ctx, opts.UnstoppableTimeout)
						defer killCancel()
						err := deployment_public.Kill(killCtx, d.DeploymentName, "TERM")
						if err != nil {
							log.Printf("Kill deployment %s with %s error: %v", d.DeploymentName, "TERM", err)
						}
						stopped = killCtx.Err() == nil
					}()
				}

				if !stopped {
					err := deployment_public.Kill(ctx, d.DeploymentName, "KILL")
					if err != nil {
						log.Printf("Kill deployment %s with %s error: %v", d.DeploymentName, "KILL", err)
					}
				}

			} else {
				err := deployment_public.Stop(ctx, d.DeploymentName)
				if err != nil {
					log.Printf("Stop deployment %s error: %v", d.DeploymentName, err)
				}
			}

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

func Kill(ctx context.Context, service_name string, signal string) error {
	if signal == "" {
		signal = "TERM"
	}

	defer utils.AddPrefix(log.Default(), "[%s] kill %s: ", service_name, signal)()

	service, err := LoadServiceByName(service_name)
	if err != nil {
		return err
	}

	deployments, err := deployment_util.List(deployment_util.ListOpts{
		FilterServiceDir: service.BasePath,
	})
	if err != nil {
		return err
	}

	for _, d := range deployments {
		log.Printf("Killing deployment %s...\n", d.DeploymentName)
		deployment_public.Kill(ctx, d.DeploymentName, signal)
	}

	return nil
}

func Cleanup(ctx context.Context, service_name string) error {
	errs := Stop(ctx, service_name, StopOpts{})

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
