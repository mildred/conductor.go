package install

import (
	"fmt"
	"os"
	"os/exec"
	"path"
)

func Install(destdir string) error {
	fmt.Fprintf(os.Stderr, "+ mkdir -p %q\n", path.Dir(destdir+ConductorCGIFunctionServiceLocation))
	err := os.MkdirAll(path.Dir(destdir+ConductorCGIFunctionServiceLocation), 0755)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ touch %q\n", destdir+ConductorServiceServiceLocation)
	err = os.WriteFile(destdir+ConductorServiceServiceLocation, []byte(ConductorServiceService), 0644)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ touch %q\n", destdir+ConductorDeploymentServiceLocation)
	err = os.WriteFile(destdir+ConductorDeploymentServiceLocation, []byte(ConductorDeploymentService), 0644)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ touch %q\n", destdir+ConductorCGIFunctionServiceLocation)
	err = os.WriteFile(destdir+ConductorCGIFunctionServiceLocation, []byte(ConductorCGIFunctionService), 0644)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ systemctl daemon-reload\n")
	err = exec.Command("systemctl", "daemon-reload").Run()
	if err != nil {
		return err
	}

	return nil
}

func Uninstall(destdir string) error {
	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorServiceServiceLocation)
	err := os.Remove(destdir + ConductorServiceServiceLocation)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorDeploymentServiceLocation)
	err = os.Remove(destdir + ConductorDeploymentServiceLocation)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorCGIFunctionServiceLocation)
	err = os.Remove(destdir + ConductorCGIFunctionServiceLocation)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ systemctl daemon-reload\n")
	err = exec.Command("systemctl", "daemon-reload").Run()
	if err != nil {
		return err
	}

	return nil
}
