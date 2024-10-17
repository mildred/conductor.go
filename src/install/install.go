package install

import (
	"os"
	"os/exec"
	"path"
)

func Install(destdir string) error {
	err := os.MkdirAll(path.Dir(destdir+ConductorCGIFunctionServiceLocation), 0755)
	if err != nil {
		return err
	}

	err = os.WriteFile(destdir+ConductorServiceServiceLocation, []byte(ConductorServiceService), 0644)
	if err != nil {
		return err
	}

	err = os.WriteFile(destdir+ConductorDeploymentServiceLocation, []byte(ConductorDeploymentService), 0644)
	if err != nil {
		return err
	}

	err = os.WriteFile(destdir+ConductorCGIFunctionServiceLocation, []byte(ConductorCGIFunctionService), 0644)
	if err != nil {
		return err
	}

	err = exec.Command("systemctl", "daemon-reload").Run()
	if err != nil {
		return err
	}

	return nil
}

func Uninstall(destdir string) error {
	err := os.Remove(destdir + ConductorServiceServiceLocation)
	if err != nil {
		return err
	}

	err = os.Remove(destdir + ConductorDeploymentServiceLocation)
	if err != nil {
		return err
	}

	err = os.Remove(destdir + ConductorCGIFunctionServiceLocation)
	if err != nil {
		return err
	}

	err = exec.Command("systemctl", "daemon-reload").Run()
	if err != nil {
		return err
	}

	return nil
}
