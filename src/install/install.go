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

	fmt.Fprintf(os.Stderr, "+ touch %q\n", destdir+ConductorServiceConfigServiceLocation)
	err = os.WriteFile(destdir+ConductorServiceConfigServiceLocation, []byte(ConductorServiceConfigService), 0644)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ touch %q\n", destdir+ConductorDeploymentServiceLocation)
	err = os.WriteFile(destdir+ConductorDeploymentServiceLocation, []byte(ConductorDeploymentService), 0644)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ touch %q\n", destdir+ConductorDeploymentConfigServiceLocation)
	err = os.WriteFile(destdir+ConductorDeploymentConfigServiceLocation, []byte(ConductorDeploymentConfigService), 0644)
	if err != nil {
		return err
	}

	/*
		fmt.Fprintf(os.Stderr, "+ touch %q\n", destdir+ConductorFunctionSocketLocation)
		err = os.WriteFile(destdir+ConductorFunctionSocketLocation, []byte(ConductorFunctionSocket), 0644)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "+ touch %q\n", destdir+ConductorCGIFunctionServiceLocation)
		err = os.WriteFile(destdir+ConductorCGIFunctionServiceLocation, []byte(ConductorCGIFunctionService), 0644)
		if err != nil {
			return err
		}
	*/

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorFunctionSocketLocation)
	err = os.Remove(destdir + ConductorFunctionSocketLocation)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorCGIFunctionServiceLocation)
	err = os.Remove(destdir + ConductorCGIFunctionServiceLocation)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ systemctl daemon-reload\n")
	cmd := exec.Command("systemctl", "daemon-reload")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
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

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorServiceConfigServiceLocation)
	err = os.Remove(destdir + ConductorServiceConfigServiceLocation)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorDeploymentServiceLocation)
	err = os.Remove(destdir + ConductorDeploymentServiceLocation)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorDeploymentConfigServiceLocation)
	err = os.Remove(destdir + ConductorDeploymentConfigServiceLocation)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorFunctionSocketLocation)
	err = os.Remove(destdir + ConductorFunctionSocketLocation)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorCGIFunctionServiceLocation)
	err = os.Remove(destdir + ConductorCGIFunctionServiceLocation)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ systemctl daemon-reload\n")
	cmd := exec.Command("systemctl", "daemon-reload")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
