package service_public

import (
	"fmt"
	"os/exec"

	"github.com/coreos/go-systemd/v22/unit"
	"github.com/gandarez/go-realpath"
)

func Reload() error {
	// TODO: reload services in well-known dirs
	return fmt.Errorf("Not yet implemented")
}

func Start(definition_path string) error {
	path, err := realpath.Realpath(definition_path)
	if err != nil {
		return err
	}

	unit := fmt.Sprintf("conductor-service@%s.service", unit.UnitNamePathEscape(path))
	return exec.Command("systemctl", "start", unit).Run()
}
