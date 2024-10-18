package service_public

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path"
	"slices"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/gandarez/go-realpath"

	"github.com/mildred/conductor.go/src/service"
	. "github.com/mildred/conductor.go/src/service"
)

func Reload(inclusive bool) error {
	var ctx = context.Background()
	sd, err := dbus.NewWithContext(ctx)
	if err != nil {
		return err
	}

	//
	// Reload services in well-known dirs
	//

	var service_dirs []string
	var start_list []string
	var stop_list []string

	for _, dir := range ServiceDirs {
		entries, err := os.ReadDir(dir)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		for _, ent := range entries {
			service_dir := path.Join(dir, ent.Name())
			_, err = os.Stat(path.Join(service_dir, service.ConfigName))
			if err != nil && !os.IsNotExist(err) {
				return err
			} else if err != nil {
				// ignore error, this is not a valid service dir
				continue
			}

			service_dirs = append(service_dirs, service_dir)
			start_list = append(start_list, ServiceUnit(service_dir))
		}
	}

	if !inclusive {
		existing_units, err := sd.ListUnitsByPatternsContext(ctx, nil, []string{"conductor-service@*.service"})
		if err != nil {
			return err
		}

		for _, u := range existing_units {
			service := ServiceDirFromUnit(u.Name)
			if service == "" || slices.Contains(service_dirs, service) {
				continue
			}

			stop_list = append(stop_list, u.Name)
		}
	}

	for _, unit := range stop_list {
		log.Printf("+ systemctl disable --now %q", unit)
		err = exec.Command("systemctl", "disable", "--now", unit).Run()
		if err != nil {
			return err
		}
	}

	for _, unit := range start_list {
		log.Printf("+ systemctl enable --now %q", unit)
		err = exec.Command("systemctl", "enable", "--now", unit).Run()
		if err != nil {
			return err
		}
	}

	return nil
}

func Start(definition_path string) error {
	path, err := realpath.Realpath(definition_path)
	if err != nil {
		return err
	}

	return exec.Command("systemctl", "start", ServiceUnit(path)).Run()
}
