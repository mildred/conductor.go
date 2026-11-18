package service_public

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"slices"

	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/service_internal"
	"github.com/mildred/conductor.go/src/utils"
)

type ReloadServiceOpts struct {
	Inclusive          bool
	Verbose            bool
	ReloadServiceNames []string
}

func ReloadServices(ctx context.Context, opts ReloadServiceOpts) error {
	inclusive := opts.Inclusive
	verbose := opts.Verbose

	sd, err := utils.NewSystemdClient(ctx)
	if err != nil {
		return err
	}

	log_prefix := log.Prefix()
	defer func() {
		log.SetPrefix(log_prefix)
	}()

	//
	// Reload services in well-known dirs
	//

	var seen_service_dirs []string
	var start_list []*service.Service
	var stop_list []*service.Service
	var dead_list []*service.Service
	var manual_stop_list []string

	for _, dir := range service.ServiceDirs {
		entries, err := os.ReadDir(dir)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		// log.Printf("Looking up services in %s, found %d entries", dir, len(entries))

		for _, ent := range entries {
			service_dir := path.Join(dir, ent.Name())
			log.SetPrefix(fmt.Sprintf("%sreload %s: ", log_prefix, service_dir))

			_, err = os.Stat(path.Join(service_dir, service.ConfigName))
			if err != nil && !os.IsNotExist(err) {
				log.Printf("ignore service, error while querying service file: %v\n", err)
				continue
			} else if err != nil {
				// ignore error, this is not a valid service dir
				log.Printf("ignoring, %v", err)
				continue
			}

			service_dir, err = service.ServiceRealpath(service_dir)
			if err != nil {
				return err
			}

			serv, err := service.LoadServiceDir(service_dir)
			if err != nil {
				log.Printf("ignore service, cannot load configuration: %v\n", err)
				continue
			}

			seen_service_dirs = append(seen_service_dirs, service_dir)

			if verbose {
				log.Printf("evaluate conditions...")
			}
			condition, disable, err := serv.EvaluateCondition(verbose)
			if err != nil {
				log.Printf("ignore service, cannot evaluate conditions: %v\n", err)
				continue
			}

			if disable {
				stop_list = append(stop_list, serv)
				if verbose {
					log.Printf("service is disabled explicitely")
				}
			} else if !condition {
				dead_list = append(dead_list, serv)
				if verbose {
					log.Printf("service is disabled because conditions do not match")
				}
			} else {
				start_list = append(start_list, serv)
				service_internal.SetServiceRequirements(ctx, serv, false)
				if err != nil {
					log.Printf("ignore service, cannot set requirements: %v\n", err)
					continue
				}
				if verbose {
					log.Printf("service is enabled")
				}
			}
		}

		log.SetPrefix(fmt.Sprintf("%sreload: ", log_prefix))
	}

	var new_start_list []*service.Service = start_list
	start_list = nil
	for len(new_start_list) > 0 {
		start_list = append(start_list, new_start_list...)
		cur_start_list := new_start_list
		new_start_list = nil

		for _, srv := range cur_start_list {
			for _, req := range srv.Requires {
				path := req.Path
				if path == "" {
					path, err = service.ServiceDirByName(req.Name)
					if err != nil {
						return err
					}
				}
				i := slices.IndexFunc(stop_list, func(ss *service.Service) bool { return ss.BasePath == path })
				if i >= 0 {
					new_start_list = append(new_start_list, stop_list[i])
					stop_list = append(stop_list[:i], stop_list[i+1:]...)
				}
			}
		}
	}

	if !inclusive {
		existing_units, err := sd.ListUnitsByPatternsContext(ctx, nil, []string{"conductor-service@*.service"})
		if err != nil {
			return err
		}

		for _, u := range existing_units {
			service := service.ServiceDirFromUnit(u.Name)
			if service == "" || slices.Contains(seen_service_dirs, service) {
				continue
			}

			manual_stop_list = append(manual_stop_list, u.Name)
		}
	}

	var errs error

	log.SetPrefix(log_prefix)

	cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "daemon-reload")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("while running systemctl %s daemon-reload, %v", dirs.SystemdModeFlag(), err)
	}

	for _, unit := range manual_stop_list {
		log.Println()
		cmd := utils.LogCommand(ctx, "systemctl", dirs.SystemdModeFlag(), "disable", "--now", unit)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = utils.RunWithJournal(ctx, cmd, unit)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("- Cannot disable %s, %v.", unit, err))
		}
	}

	for _, svc := range stop_list {
		log.Println()
		log.SetPrefix(fmt.Sprintf("%sreload %s: ", log_prefix, svc.Name))
		err = service_internal.DisableService(ctx, svc, true)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("- Cannot disable %s, %v.", svc.BasePath, err))
		}
		log.SetPrefix(log_prefix)
	}

	for _, svc := range start_list {
		log.Println()
		log.SetPrefix(fmt.Sprintf("%sreload %s: ", log_prefix, svc.Name))
		reload := slices.Contains(opts.ReloadServiceNames, svc.Name)
		enable_now := !reload
		err = service_internal.EnableService(ctx, svc, enable_now, reload)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("- Cannot enable %s, %v.", svc.BasePath, err))
		}
		log.SetPrefix(log_prefix)
	}

	if errs != nil {
		log.Println()
		errs = errors.Join(fmt.Errorf("Some services could not reload properly:"), errs)
	}

	return errs
}
