package service_internal

import (
	"context"
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

func Stop(service_name string) error {
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
	// Stop MAINPID monitoring
	//

	if mainpid := os.Getenv("MAINPID"); mainpid != "" {
		log.Printf("stop: Sending SIGTERM to pid=%s\n", mainpid)
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

	log.Printf("stop: Stopping all deployments...\n")

	deployments, err := deployment_util.List(deployment_util.ListOpts{
		FilterServiceDir: service.BasePath,
	})
	if err != nil {
		return err
	}

	for _, d := range deployments {
		log.Printf("stop: Stopping deployment %s...\n", d.DeploymentName)

		ctx, cancel := context.WithCancel(context.Background())
		go utils.ExtendTimeout(ctx, 60*time.Second)

		func() {
			defer cancel()
			deployment_public.Stop(d.DeploymentName)
		}()
	}

	log.Printf("stop: Stop sequence completed\n")
	return nil
}

func Cleanup(service_name string) error {
	//
	// Remove temp files if there is any
	//

	// log.Printf("cleanup: Cleaning up...\n")

	log.Printf("cleanup: Cleanup sequence completed\n")
	return nil
}
