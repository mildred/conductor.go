package service_internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/mildred/conductor.go/src/caddy"
	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/deployment_public"
	"github.com/mildred/conductor.go/src/deployment_util"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/tmpl"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/service"
)

// Note for services: a linked proxy config is set up when the service itself is
// enabled. The service depends on this proxy config

type StartOrReloadOpts struct {
	Restart            bool
	WantsFresh         bool
	MaxDeploymentIndex int
	StopTimeout        time.Duration
	TermTimeout        time.Duration
}

func stopServicesOrLog(prefix string, depl *deployment.Deployment, units []string) {
	log.Printf("%s: Stop %s newest units after failure...", prefix, depl.DeploymentName)
	for _, unit := range units {
		fmt.Fprintf(os.Stderr, "+ systemctl %s stop %q\n", dirs.SystemdModeFlag(), unit)
		cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "stop", unit)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			log.Printf("%s: ERROR %v", prefix, err)
		}
	}
}

func StartOrReload(service_name string, opts StartOrReloadOpts) error {
	if opts.MaxDeploymentIndex == 0 {
		opts.MaxDeploymentIndex = 10
	}

	var prefix string = "start"
	var err error
	var ctx = context.Background()
	var deferred func() = func() {}

	//
	// [restart] Notify systemd reload in progress
	//

	if opts.Restart {
		prefix = "restart"

		var notified bool
		notified, err = daemon.SdNotify(false, daemon.SdNotifyStopping)
		if err != nil {
			return err
		}

		deferred = func() {
			if notified {
				log.Printf("restart: Notifying ready...")
				_, er := daemon.SdNotify(false, daemon.SdNotifyReady)
				if er != nil && err != nil {
					err = fmt.Errorf("%v; additionally, there was an error notifying the end of the reloading process: %v", err, er)
				} else if er != nil {
					log.Printf("restart: Error notifying ready: %v", er)
					err = er
				} else {
					log.Printf("restart: Notified ready, wait for systemd to catch up")
					time.Sleep(5 * time.Second)
					log.Printf("restart: dying")
				}
			}
			notified = false
		}
	}

	defer deferred()

	//
	// Fetch service config
	//

	service, err := LoadServiceByName(service_name)
	if err != nil {
		return err
	}

	//
	// Find or create a suitable deployment
	//

	var started_services []string

	parts, err := service.Parts()
	if err != nil {
		return err
	}

	var depl_names []string

	for _, part_name := range parts {

		log.Printf("%s: Loaded service, configure socket for part %q...\n", prefix, part_name)

		seed, err := deployment.SeedFromService(service, part_name)
		if err != nil {
			return err
		}

		depl, depl_status, err := deployment_util.StartNewOrExistingFromService(ctx, service, seed, deployment_util.StartNewOrExistingOpts{
			MaxIndex:  opts.MaxDeploymentIndex,
			WantFresh: opts.WantsFresh,
		})
		if err != nil {
			return err
		}

		depl_names = append(depl_names, depl.DeploymentName)

		if seed.IsPod {

			ctx, cancel := context.WithCancel(context.Background())
			go utils.ExtendTimeout(ctx, 60*time.Second)

			err = func() error {
				defer cancel()

				if depl_status == "active" {
					log.Printf("%s: Found started pod deployment %s", prefix, depl.DeploymentName)
				} else if depl_status == "activating" || depl_status == "inactive" {
					log.Printf("%s: Found %s pod deployment %s, waiting to be started...", prefix, depl_status, depl.DeploymentName)
					fmt.Fprintf(os.Stderr, "+ systemctl %s start %q\n", dirs.SystemdModeFlag(), deployment.DeploymentUnit(depl.DeploymentName))
					cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "start", deployment.DeploymentUnit(depl.DeploymentName))
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					err = cmd.Run()
					started_services = append(started_services, deployment.DeploymentUnit(depl.DeploymentName))
					if err != nil {
						return err
					}
				} else {
					log.Printf("%s: Starting new pod deployment %s...", prefix, depl.DeploymentName)
					fmt.Fprintf(os.Stderr, "+ systemctl %s start %q\n", dirs.SystemdModeFlag(), deployment.DeploymentUnit(depl.DeploymentName))
					cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "start", deployment.DeploymentUnit(depl.DeploymentName))
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					err = cmd.Run()
					started_services = append(started_services, deployment.DeploymentUnit(depl.DeploymentName))
					if err != nil {
						return err
					}
				}

				return nil
			}()
			if err != nil {
				stopServicesOrLog(prefix, depl, started_services)
				started_services = nil
				return err
			}

		} else if seed.IsFunction {

			log.Printf("%s: Starting new CGI function deployment %s...", prefix, depl.DeploymentName)
			fmt.Fprintf(os.Stderr, "+ systemctl %s start %q\n", dirs.SystemdModeFlag(), deployment.CGIFunctionSocketUnit(depl.DeploymentName))
			cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "start", deployment.CGIFunctionSocketUnit(depl.DeploymentName))
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			started_services = append(started_services, deployment.CGIFunctionSocketUnit(depl.DeploymentName))
			if err != nil {
				stopServicesOrLog(prefix, depl, started_services)
				started_services = nil
				return err
			}

		}

	}

	//
	// TODO [ if restarting ] if failing to start, exit uncleanly
	// TODO [ if starting ] if failing, stop all deployments and exit uncleanly
	//

	//
	// Stop all deployments that are of older config version
	//

	log.Printf("%s: Removing obsolete deployments (except %v)...\n", prefix, depl_names)

	deployments, err := deployment_util.List(deployment_util.ListOpts{
		FilterServiceDir: service.BasePath,
	})
	if err != nil {
		return err
	}

	for _, d := range deployments {
		if slices.Contains(depl_names, d.DeploymentName) {
			continue
		}

		log.Printf("%s: Removing deployment %s...\n", prefix, d.DeploymentName)

		ctx, cancel := context.WithCancel(context.Background())
		go utils.ExtendTimeout(ctx, 60*time.Second)

		err = func() error {
			defer cancel()
			return deployment_util.RemoveTimeout(ctx, d.DeploymentName, opts.StopTimeout, opts.TermTimeout)
		}()
		if err != nil {
			log.Printf("%s: ERROR removing deployment %s (but continuing): %v", prefix, d.DeploymentName, err)
		}
	}

	//
	// [restart] Notify systemd ready and stop there
	//

	if opts.Restart {
		log.Printf("restart: Restart sequence completed\n")
		deferred() // execute without defer statement in case it cause an issue with the PID and systemd cannot attribute the notify message
		return err
	}

	//
	// [start] Notify systemd ready and monitor deployments
	//

	_, err = daemon.SdNotify(false, daemon.SdNotifyReady)
	if err != nil {
		return err
	}
	log.Printf("start: Start sequence completed, start to monitor deployments\n")

	//
	// Keep running in the background, and monitor the deployments
	// (exit with an error if a deployment is missing)
	//

	for {
		// Reload service in case it changes its id
		service, err = LoadServiceByName(service_name)
		if err != nil {
			return err
		}

		parts, err := service.Parts()
		if err != nil {
			return err
		}

		var diagnostics []string
		all_parts_ok := true

		for _, part := range parts {
			deployments, err := deployment_util.List(deployment_util.ListOpts{
				FilterServiceDir: service.BasePath,
				FilterPartName:   &part,
			})
			if err != nil {
				return err
			}

			if len(deployments) == 0 {
				diagnostics = append(diagnostics, fmt.Sprintf("part %q: no deployment found", part))
				all_parts_ok = false
				continue
			}

			part_found := false
			for _, depl := range deployments {
				if depl.ServiceId != service.Id {
					diagnostics = append(diagnostics, fmt.Sprintf("part %q: deployment %s id %q is invalid", part, depl.DeploymentName, depl.ServiceId))
				} else {
					diagnostics = append(diagnostics, fmt.Sprintf("part %q: deployment %s matches", part, depl.DeploymentName))
					part_found = true
				}
			}
			if !part_found {
				all_parts_ok = false
			}
		}

		if !all_parts_ok {
			return fmt.Errorf("deployment has gone missing for service %q (id: %q):\n%s", service_name, service.Id, strings.Join(diagnostics, "\n"))
		}

		time.Sleep(30 * time.Second)
	}
}

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

var LookupPaths []string = dirs.MultiJoin("services", append(append([]string{dirs.SelfRuntimeDir}, dirs.SelfConfigDirs...), dirs.SelfDataDirs...)...)

func CaddyRegister(register bool, service_name string) error {
	var prefix = "register"
	if !register {
		prefix = "deregister"
	}

	service, err := LoadServiceByName(service_name)
	if err != nil {
		return err
	}

	if service.ProxyConfigTemplate == "" {
		return nil
	}

	var configs []caddy.ConfigItem

	caddy, err := caddy.NewClient(service.CaddyLoadBalancer.ApiEndpoint)
	if err != nil {
		return err
	}

	config, err := tmpl.RunTemplate(service.ProxyConfigTemplate, service.Vars())
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(config), &configs)
	if err != nil {
		return err
	}

	if register {
		log.Printf("register: Registering service...")
	} else {
		log.Printf("deregister: Deregistering service...")
	}

	err = caddy.Register(register, configs)
	if err != nil {
		return err
	}

	log.Printf("%s: Completed", prefix)
	return nil
}

func Template(service_name string, template string) error {
	service, err := LoadServiceByName(service_name)
	if err != nil {
		return err
	}

	return tmpl.RunTemplateStdout(template, service.Vars())
}
