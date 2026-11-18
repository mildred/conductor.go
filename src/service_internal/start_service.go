package service_internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/taigrr/systemctl/properties"

	"github.com/mildred/conductor.go/src/caddy"
	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/deployment_util"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/part_pod"
	"github.com/mildred/conductor.go/src/pidfile"
	"github.com/mildred/conductor.go/src/service_util"
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
	StartConfig        bool
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

func StartOrReloadByName(ctx context.Context, service_name string, opts StartOrReloadOpts) error {
	service, err := LoadServiceByName(service_name)
	if err != nil {
		return err
	}

	return StartOrReload(ctx, service, service_name, opts)
}

func StartOrReload(ctx context.Context, service *Service, service_name string, opts StartOrReloadOpts) error {
	nice_service_name, _ := service.GetServiceNameOrPath()
	if opts.Restart {
		defer utils.AddPrefix(log.Default(), "[restart %s] ", nice_service_name)()
	} else {
		defer utils.AddPrefix(log.Default(), "[restart %s] ", nice_service_name)()
	}

	if opts.MaxDeploymentIndex == 0 {
		opts.MaxDeploymentIndex = 10
	}

	var action_name string = "start"
	var err error
	var deferred func() = func() {}

	sd, err := utils.NewSystemdClient(ctx)
	if err != nil {
		return err
	}

	service_dir := service.BasePath

	pidfile, err := pidfile.CreateUpdateOrRetryPIDFile(ctx, ServiceLockFilename(service_dir), time.Second)
	if err != nil {
		return err
	}
	defer pidfile.Close()

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
	// Ensure conductor-service-config@.service is reloaded
	//

	if service.HasProxyConfig() {
		config_unit := ServiceConfigUnit(service.BasePath)

		if opts.StartConfig {
			cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "start", config_unit)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = utils.RunWithJournal(ctx, cmd, config_unit)
			if err != nil {
				return err
			}
		}

		prop, _ := sd.GetUnitPropertyContext(ctx, config_unit, string(properties.StateChangeTimestamp))
		if prop != nil {
			if timestamp, ok := prop.Value.Value().(uint64); ok {
				active_since := time.Unix(int64(timestamp/1e6), int64(timestamp%1e6))
				now := time.Now()
				if now.Sub(active_since).Abs() > 10*time.Second {
					log.Printf("Service configuration last change at %v, reloading...", active_since)
					cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "try-reload-or-restart", config_unit)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					err = utils.RunWithJournal(ctx, cmd, config_unit)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	//
	// Run pre-start-service hook
	//

	err = service.RunHooks(ctx, "pre-start-service", "", nil, 60*time.Second)
	if err != nil {
		return err
	}

	//
	// Bring address up
	//

	err = service_util.BringAddrUp(ctx, service)
	if err != nil {
		return err
	}

	//
	// Find or create a suitable deployment
	//

	var started_services []string

	parts := service.PartNames()

	var depl_names []string
	var seeds []*deployment.DeploymentSeed
	var deployments []*deployment.Deployment
	var deployment_statuses []string
	var depl_other_parts = map[string]*deployment.OtherPart{}

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

		depl, depl_status, err := deployment_util.PrepareNewOrExistingFromService(ctx, service, seed, deployment_util.StartNewOrExistingOpts{
			MaxIndex:  opts.MaxDeploymentIndex,
			ReuseDepl: opts.ReuseDepl || opts.ReuseDeplIfStatic && service.IsStaticDeployment(),
		})
		if err != nil {
			return err
		}

		depl_names = append(depl_names, depl.DeploymentName)
		deployments = append(deployments, depl)
		deployment_statuses = append(deployment_statuses, depl_status)
		seeds = append(seeds, seed)
		depl_other_parts[depl.PartName] = deployment.NewOtherPartFromDeployment(depl)
	}

	//
	// Save all cross-data between deployments from a same service
	//

	for i, depl := range deployments {
		seed := seeds[i]
		seed.OtherParts = depl_other_parts

		if depl.Service != nil {
			depl.OtherParts = depl_other_parts
			e := depl.SaveDefaultLocation()
			if e != nil {
				err = errors.Join(err, e)
			}
		}
	}
	if err != nil {
		return err
	}

	//
	// Create seed only deployments
	//

	for i, depl := range deployments {
		seed := seeds[i]
		if depl.Service != nil {
			continue
		}

		depl, err := deployment_util.CreateDeploymentFromService(ctx, depl.DeploymentName, service, seed)
		if err != nil {
			return err
		}

		deployments[i] = depl
	}

	//
	// Start all deployments
	//

	for i, _ := range parts {
		seed := seeds[i]
		depl := deployments[i]
		depl_status := deployment_statuses[i]

		if depl_status == "active" {
			log.Printf("Found started %s deployment %s", depl.PartType, depl.DeploymentName)
		} else if depl_status == "activating" || depl_status == "inactive" {
			log.Printf("Found %s %s deployment %s, waiting to be started...", depl_status, depl.PartType, depl.DeploymentName)
		} else {
			log.Printf("Starting new %s deployment %s...", depl.PartType, depl.DeploymentName)
		}

		e := func() error {
			defer utils.AddPrefix(log.Default(), fmt.Sprintf("[activate %s] ", depl.DeploymentName))()

			if depl.PartType == "" {
				return fmt.Errorf("deployment is missing its type")
			}

			err = depl.TemplateAll(ctx)
			if err != nil {
				return fmt.Errorf("failed to template deployment, %v", err)
			}

			vars, err := depl.Vars(ctx)
			if err != nil {
				return err
			}

			err = depl.RunHooks(ctx, "pre-create", depl.PartName, vars, 60*time.Second)
			if err != nil {
				return err
			}

			if seed.IsPod {
				//
				// Create the pod or fail
				//

				log.Printf("Create the deployment pod\n")
				err := pod.KubePlayPod(depl, pod.PodCreate, depl.DeploymentDir)
				if err != nil {
					err = fmt.Errorf("failed to create deployment pod, %v", err)
					return err
				}
			}

			err = depl.RunHooks(ctx, "pre-activate", depl.PartName, vars, 60*time.Second)
			if err != nil {
				return err
			}

			unit_name := depl.MainUnit()

			if seed.IsPod || seed.IsDaemon {

				ctx, cancel := context.WithCancel(context.Background())
				go utils.ExtendTimeout(ctx, 60*time.Second)

				err = func() error {
					defer cancel()

					action := "start"
					if opts.Restart {
						action = "reload-or-restart"
					}

					cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), action, unit_name)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					err = utils.RunWithJournal(ctx, cmd, unit_name)
					started_services = append(started_services, unit_name)
					if err != nil {
						return err
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

			} else if seed.IsSocket {

				log.Printf("Starting new socket deployment %s...", depl.DeploymentName)
				cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "reload-or-restart", depl.MainUnit())
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				err = utils.RunWithJournal(ctx, cmd, depl.MainUnit())
				if err != nil {
					stopServicesOrLog(ctx, depl, started_services)
					started_services = nil
					return err
				}

			}

			err = depl.RunHooks(ctx, "post-activate", depl.PartName, vars, 60*time.Second)
			if err != nil {
				return err
			}

			return nil
		}()
		if e != nil {
			err = errors.Join(err, e)
		}

	}
	if err != nil {
		return err
	}

	//
	// TODO [ if restarting ] if failing to start, exit uncleanly
	// TODO [ if starting ] if failing, stop all deployments and exit uncleanly
	//

	//
	// Stop all deployments that are of older config version
	//

	log.Printf("Removing obsolete deployments (except %v)...\n", depl_names)

	all_system_deployments, err := deployment.List(deployment.ListOpts{
		FilterServiceDir: service.BasePath,
	})
	if err != nil {
		return err
	}

	for _, d := range all_system_deployments {
		if slices.Contains(depl_names, d.DeploymentName) {
			continue
		}

		log.Printf("Removing deployment %s...\n", d.DeploymentName)

		err := func() error {
			defer utils.AddPrefix(log.Default(), fmt.Sprintf("[remove %s] ", d.DeploymentName))()

			vars, err := d.Vars(ctx)
			if err != nil {
				return err
			}

			err = d.RunHooks(ctx, "pre-remove", d.PartName, vars, 60*time.Second)
			if err != nil {
				log.Printf("ERROR executing pre-remote hooks for %s (but continuing): %v", d.DeploymentName, err)
				return nil
			}

			ctx, cancel := context.WithCancel(context.Background())
			go utils.ExtendTimeout(ctx, 60*time.Second)

			err = func() error {
				defer cancel()
				return deployment_util.RemoveTimeout(ctx, d.DeploymentName, opts.StopTimeout, opts.TermTimeout)
			}()
			if err != nil {
				log.Printf("ERROR removing deployment %s (but continuing): %v", d.DeploymentName, err)
				return nil
			}

			err = d.RunHooks(ctx, "post-remove", d.PartName, vars, 60*time.Second)
			if err != nil {
				log.Printf("ERROR executing post-remote hooks for %s (but continuing): %v", d.DeploymentName, err)
				return nil
			}

			return nil
		}()
		if err != nil {
			return err
		}

	}

	//
	// Run post-start-service hook
	//

	err = service.RunHooks(ctx, "post-start-service", "", nil, 60*time.Second)
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

	pidfile.Close()

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
			deployments, err := deployment.List(deployment.ListOpts{
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
	var hook_name string
	if register {
		defer utils.ReplacePrefix(log.Default(), "[%s] register: ", service_name)()
		hook_name = "register-service"
	} else {
		defer utils.ReplacePrefix(log.Default(), "[%s] deregister: ", service_name)()
		hook_name = "deregister-service"
	}

	service, err := LoadServiceByName(service_name)
	if err != nil {
		return err
	}

	err = service.RunHooks(ctx, "pre-"+hook_name, "", nil, 60*time.Second)
	if err != nil {
		return err
	}

	configs, err := service.ProxyConfig(ctx, false)
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

	err = service.RunHooks(ctx, "post-"+hook_name, "", nil, 60*time.Second)
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

	svars, err := service.Vars(ctx)
	if err != nil {
		return err
	}

	return tmpl.RunTemplateStdout(ctx, template, svars)
}
