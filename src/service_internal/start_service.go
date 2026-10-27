package service_internal

import (
	"context"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/mildred/conductor.go/src/caddy"
	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/deployment_util"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/tmpl"
	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/service"
)

// Note for services: a linked proxy config is set up when the service itself is
// enabled. The service depends on this proxy config

type StartOrReloadOpts struct {
	ExitWhenStarted    bool
	Restart            bool
	ReuseDepl          bool
	ReuseDeplIfStatic  bool
	NoLogPrefix        bool
	MaxDeploymentIndex int
	StopTimeout        time.Duration
	TermTimeout        time.Duration
}

func stopServicesOrLog(ctx context.Context, depl *deployment.Deployment, units []string) {
	log.Printf("Stop %s newest units after failure...", depl.DeploymentName)
	for _, unit := range units {
		cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "stop", unit)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := utils.RunWithJournal(ctx, cmd, unit)
		if err != nil {
			log.Printf("ERROR %v", err)
		}
	}
}

func StartOrReload(service_name string, opts StartOrReloadOpts) error {
	if !opts.NoLogPrefix && opts.Restart {
		defer utils.AddPrefix(log.Default(), "[%s] restart: ", service_name)()
	} else if !opts.NoLogPrefix {
		defer utils.AddPrefix(log.Default(), "[%s] start: ", service_name)()
	}

	if opts.MaxDeploymentIndex == 0 {
		opts.MaxDeploymentIndex = 10
	}

	var action_name string = "start"
	var err error
	var ctx = context.Background()
	var deferred func() = func() {}

	//
	// [restart] Notify systemd reload in progress
	//

	if opts.Restart {
		action_name = "restart"

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

	if service.IsStaticDeployment() && opts.ReuseDeplIfStatic {
		opts.ReuseDepl = true
	}

	//
	// Run pre-start-service hook
	//

	err = service.RunHooks(ctx, "pre-start-service", "", service.Vars(), 60*time.Second)
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

		if part_name == "" {
			log.Printf("Loaded service, configure socket for default part...\n")
		} else {
			log.Printf("Loaded service, configure socket for part %q...\n", part_name)
		}

		seed, err := deployment.SeedFromService(ctx, service, part_name)
		if err != nil {
			return err
		}

		depl, depl_status, err := deployment_util.StartNewOrExistingFromService(ctx, service, seed, deployment_util.StartNewOrExistingOpts{
			MaxIndex:  opts.MaxDeploymentIndex,
			ReuseDepl: opts.ReuseDepl,
		})
		if err != nil {
			return err
		}

		unit_name := depl.MainUnit()

		depl_names = append(depl_names, depl.DeploymentName)

		if seed.IsPod {

			ctx, cancel := context.WithCancel(context.Background())
			go utils.ExtendTimeout(ctx, 60*time.Second)

			err = func() error {
				defer cancel()

				if depl_status == "active" {
					log.Printf("Found started pod deployment %s", depl.DeploymentName)
				} else if depl_status == "activating" || depl_status == "inactive" {
					log.Printf("Found %s pod deployment %s, waiting to be started...", depl_status, depl.DeploymentName)
					cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "start", unit_name)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					err = utils.RunWithJournal(ctx, cmd, unit_name)
					started_services = append(started_services, unit_name)
					if err != nil {
						return err
					}
				} else {
					log.Printf("Starting new pod deployment %s...", depl.DeploymentName)
					cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "start", unit_name)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					err = utils.RunWithJournal(ctx, cmd, unit_name)
					started_services = append(started_services, unit_name)
					if err != nil {
						return err
					}
				}

				return nil
			}()
			if err != nil {
				stopServicesOrLog(ctx, depl, started_services)
				started_services = nil
				return err
			}

		} else if seed.IsFunction {

			log.Printf("Starting new CGI function deployment %s...", depl.DeploymentName)
			cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "start", deployment.CGIFunctionSocketUnit(depl.DeploymentName))
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = utils.RunWithJournal(ctx, cmd, deployment.CGIFunctionSocketUnit(depl.DeploymentName))
			started_services = append(started_services, deployment.CGIFunctionSocketUnit(depl.DeploymentName))
			if err != nil {
				stopServicesOrLog(ctx, depl, started_services)
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

	log.Printf("Removing obsolete deployments (except %v)...\n", depl_names)

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

		log.Printf("Removing deployment %s...\n", d.DeploymentName)

		ctx, cancel := context.WithCancel(context.Background())
		go utils.ExtendTimeout(ctx, 60*time.Second)

		err = func() error {
			defer cancel()
			return deployment_util.RemoveTimeout(ctx, d.DeploymentName, opts.StopTimeout, opts.TermTimeout)
		}()
		if err != nil {
			log.Printf("ERROR removing deployment %s (but continuing): %v", d.DeploymentName, err)
		}
	}

	//
	// Run post-start-service hook
	//

	err = service.RunHooks(ctx, "post-start-service", "", service.Vars(), 60*time.Second)
	if err != nil {
		return err
	}

	//
	// [restart] Notify systemd ready and stop there
	//

	if opts.Restart {
		log.Printf("restart: Restart sequence completed\n")
		deferred() // execute without defer statement in case it cause an issue with the PID and systemd cannot attribute the notify message
		if err == nil {
			log.Printf("restart: completed with no error\n")
		} else {
			log.Printf("restart: completed with an error: %v\n", err)
		}
		return err
	}

	//
	// [start] Notify systemd ready and monitor deployments
	//

	_, err = daemon.SdNotify(false, daemon.SdNotifyReady)
	if err != nil {
		return err
	}

	if opts.ExitWhenStarted || service.IsStaticDeployment() {
		log.Printf("start: Start sequence completed\n")
		return nil
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

		part_ids, err := service.PartIds(ctx)
		if err != nil {
			return err
		}

		var diagnostics []string
		all_parts_ok := true

		for part, part_id := range part_ids {
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
				if depl.PartId != part_id {
					diagnostics = append(diagnostics, fmt.Sprintf("part %q: deployment %s id %q (service %q) is invalid", part, depl.DeploymentName, depl.PartId, depl.ServiceId))
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
			for part_name, part_id := range part_ids {
				if part_name == "" {
					diagnostics = append([]string{fmt.Sprintf("service default part has id %q", part_id)}, diagnostics...)
				} else {
					diagnostics = append([]string{fmt.Sprintf("service part %q has id %q", part_name, part_id)}, diagnostics...)
				}
			}
			return fmt.Errorf("deployment has gone missing for service %q or service configuration changed during %s:\n  - service id: %q\n  - %s",
				service_name, action_name, service.Id, strings.Join(diagnostics, "\n  - "))
		}

		time.Sleep(30 * time.Second)
	}
}

var LookupPaths []string = dirs.MultiJoin("services", append(append([]string{dirs.SelfRuntimeDir}, dirs.SelfConfigDirs...), dirs.SelfDataDirs...)...)

func CaddyRegister(ctx context.Context, register bool, service_name string) error {
	if register {
		defer utils.ReplacePrefix(log.Default(), "[%s] register: ", service_name)()
	} else {
		defer utils.ReplacePrefix(log.Default(), "[%s] deregister: ", service_name)()
	}

	service, err := LoadServiceByName(service_name)
	if err != nil {
		return err
	}

	configs, err := service.ProxyConfig(ctx)
	if err != nil {
		return err
	} else if len(configs) == 0 {
		return nil
	}

	caddy, err := caddy.NewClient(service.CaddyLoadBalancer.ApiEndpoint, time.Duration(service.CaddyLoadBalancer.Timeout))
	if err != nil {
		return err
	}

	if register {
		log.Printf("Registering service...")
	} else {
		log.Printf("Deregistering service...")
	}

	err = caddy.Register(ctx, register, configs)
	if err != nil {
		return err
	}

	log.Printf("Completed")
	return nil
}

func Template(ctx context.Context, service_name string, template string) error {
	service, err := LoadServiceByName(service_name)
	if err != nil {
		return err
	}

	return tmpl.RunTemplateStdout(ctx, template, service.Vars())
}
