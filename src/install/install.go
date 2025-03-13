package install

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
)

func Update(version string, check bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	if check || version == "dev" {
		rel, found, err := selfupdate.DetectLatest("mildred/conductor.go")
		if err != nil {
			return err
		}
		if found {
			log.Println("Latest version is", rel.Version)
		} else {
			log.Println("Latest release not found")
		}
		return nil
	}

	v := semver.MustParse(version)

	latest, err := selfupdate.DefaultUpdater().UpdateSelf(v, "mildred/conductor.go")
	if err != nil {
		log.Println("Binary update failed:", err)
		return nil
	}

	tubectl_path := path.Join(path.Dir(exe), "tubectl")
	err = selfupdate.DefaultUpdater().UpdateTo(latest, tubectl_path)
	if err != nil {
		log.Println("Binary update for tubectl failed:", err)
		return nil
	}

	if check || version == "dev" {
		log.Println("Latest version is", latest.Version)
	} else if latest.Version.Equals(v) {
		// latest version is the same as current version. It means current binary is up to date.
		log.Println("Current binary is the latest version", version)
	} else {
		log.Println("Successfully updated to version", latest.Version)
		log.Println("Release note:\n", latest.ReleaseNotes)

	}
	return nil

}

func isInstalled() (bool, string, error) {
	self, err := os.Executable()
	if err != nil {
		return false, self, err
	}

	installed, err := exec.LookPath(path.Base(self))
	if err != nil {
		return false, self, nil
	}

	self_st, err := os.Stat(self)
	if err != nil {
		return false, self, err
	}

	installed_st, err := os.Stat(installed)
	if err != nil {
		return false, self, err
	}

	return os.SameFile(self_st, installed_st), self, nil
}

func Install(destdir string) error {
	if destdir == "" {
		installed, executable, err := isInstalled()
		if err != nil {
			return err
		}
		if !installed {
			destination := path.Join("/usr/local/bin", path.Base(executable))
			fmt.Fprintf(os.Stderr, "+ cp %q %q\n", executable, destination)

			dest, err := os.OpenFile(destination, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o777)
			if err != nil {
				return err
			}
			defer dest.Close()

			src, err := os.Open(executable)
			if err != nil {
				return err
			}
			defer src.Close()

			_, err = io.Copy(dest, src)
			if err != nil {
				return err
			}
		}
	}

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

	fmt.Fprintf(os.Stderr, "+ touch %q\n", destdir+ConductorPolicyServerSocketLocation)
	err = os.WriteFile(destdir+ConductorPolicyServerSocketLocation, []byte(ConductorPolicyServerSocket), 0644)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ touch %q\n", destdir+ConductorPolicyServerServiceLocation)
	err = os.WriteFile(destdir+ConductorPolicyServerServiceLocation, []byte(ConductorPolicyServerService), 0644)
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
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorCGIFunctionServiceLocation)
	err = os.Remove(destdir + ConductorCGIFunctionServiceLocation)
	if err != nil && !os.IsNotExist(err) {
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
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorServiceConfigServiceLocation)
	err = os.Remove(destdir + ConductorServiceConfigServiceLocation)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorDeploymentServiceLocation)
	err = os.Remove(destdir + ConductorDeploymentServiceLocation)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorDeploymentConfigServiceLocation)
	err = os.Remove(destdir + ConductorDeploymentConfigServiceLocation)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorFunctionSocketLocation)
	err = os.Remove(destdir + ConductorFunctionSocketLocation)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorCGIFunctionServiceLocation)
	err = os.Remove(destdir + ConductorCGIFunctionServiceLocation)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorPolicyServerSocketLocation)
	err = os.Remove(destdir + ConductorPolicyServerSocketLocation)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorPolicyServerServiceLocation)
	err = os.Remove(destdir + ConductorPolicyServerServiceLocation)
	if err != nil && !os.IsNotExist(err) {
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

	fmt.Fprintf(os.Stderr, "+ systemctl enable --now conductor-policy-server.socket\n")
	cmd = exec.Command("systemctl", "enable", "--now", "conductor-policy-server.socket")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
