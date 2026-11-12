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

func ReloadServices(inclusive bool, verbose bool) error {
	var ctx = context.Background()
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

			if disable || !condition {
				stop_list = append(stop_list, serv)
				if verbose && disable {
					log.Printf("service is disabled explicitely")
				} else if verbose && !condition {
					log.Printf("service is disabled because conditions do not match")
				}
			} else {
				start_list = append(start_list, serv)
				setServiceRequirements(ctx, serv)
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
		err = service_internal.EnableService(ctx, svc, true)
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

func setServiceRequirements(ctx context.Context, svc *service.Service) error {
	var err error
	var requirements_conf = "[Unit]\n"
	var num_requirements int = 0

	for _, req := range svc.Requires {
		var path string
		if req.Path != "" {
			path = req.Path
		}
		if req.Name != "" {
			path, err = service.ServiceDirByName(req.Name)
			if err != nil {
				return err
			}
		}
		if path != "" {
			requirements_conf += "Requires=" + service.ServiceUnit(path) + "\n"
			if req.After {
				requirements_conf += "After=" + service.ServiceUnit(path) + "\n"
			}
			num_requirements += 1
		}
	}

	cfgdir := dirs.Join(dirs.ConfigHome, "systemd", dirs.SystemdMode(), service.ServiceUnit(svc.BasePath)+".d")

	if num_requirements == 0 {
		return unsetServiceRequirements(ctx, svc)
	} else {
		// log.Printf("Create %s", cfgdir)
		err = os.MkdirAll(cfgdir, 0755)
		if err != nil {
			return err
		}

		log.Printf("Write %s", dirs.Join(cfgdir, "conductor-requirements.conf"))
		err = os.WriteFile(dirs.Join(cfgdir, "conductor-requirements.conf"), []byte(requirements_conf), 0o644)
		if err != nil {
			return err
		}

		return nil
	}
}

func unsetServiceRequirements(ctx context.Context, svc *service.Service) error {
	return os.RemoveAll(dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), service.ServiceUnit(svc.BasePath)+".d", "conductor-requirements.conf"))
}
