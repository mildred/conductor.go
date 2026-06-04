package install

import (
	"embed"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"

	"github.com/mildred/conductor.go/src/dirs"
)

func Update(version string, desired_version string, check bool) error {
	var current semver.Version
	if version != "dev" {
		current = semver.MustParse(version)
	}

	argv0 := os.Args[0]
	executable, err := exec.LookPath(argv0)
	if err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(exe, ".exe") {
		// Ensure to add '.exe' to given path on Windows
		exe = exe + ".exe"
	}

	stat, err := os.Lstat(exe)
	if err != nil {
		return fmt.Errorf("Failed to stat '%s'. File may not exist: %s", exe, err)
	}
	if stat.Mode()&os.ModeSymlink != 0 {
		p, err := filepath.EvalSymlinks(exe)
		if err != nil {
			return fmt.Errorf("Failed to resolve symlink '%s' for executable: %s", exe, err)
		}
		exe = p
	}

	rel, found, err := selfupdate.DetectVersion("mildred/conductor.go", desired_version)
	if err != nil {
		return err
	} else if !found {
		log.Println("No release detected. Current version is considered up-to-date")
		return nil
	} else if current.Equals(rel.Version) {
		log.Println("Current version", current, "is the latest. Update is not needed")
		return nil
	}

	log.Println("Will update", exe, "to the latest version", rel.Version)

	if check || version == "dev" {
		if found {
			log.Println("Latest version is", rel.Version)
		} else {
			log.Println("Latest release not found")
		}
		return nil
	}

	err = selfupdate.DefaultUpdater().UpdateTo(rel, exe)
	if err != nil {
		log.Println("Binary update failed:", err)
		return nil
	}

	if check || version == "dev" {
		log.Println("Latest version is", rel.Version)
	} else if rel.Version.Equals(current) {
		// latest version is the same as current version. It means current binary is up to date.
		log.Println("Current binary is the latest version", version)
	} else {
		log.Println("Successfully updated to version", rel.Version)
		log.Println("Release note:\n", rel.ReleaseNotes)
	}

	tubectl_path := path.Join(path.Dir(exe), "tubectl")

	f, err := os.OpenFile(tubectl_path, os.O_RDONLY|os.O_CREATE, 0777)
	if err == nil {
		f.Close()
	}

	err = selfupdate.DefaultUpdater().UpdateTo(rel, tubectl_path)
	if err != nil {
		log.Println("Binary update for tubectl failed:", err)
		return nil
	}

	if path.Dir(exe) == "/usr/local/bin" || path.Dir(executable) == "/usr/local/bin" {
		cmd := exec.Command(exe, "system", "install")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Fprintf(os.Stderr, "+ %s system install\n", exe)
		err := cmd.Run()
		if err != nil {
			log.Printf("%s system install failed: %v", exe, err)
			return err
		}
	} else {
		log.Printf("Please run '%s system install' to finish installation of systemd units", executable)
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

func Install(destdir string, unlink, enable, now bool) error {
	if destdir == "" {
		installed, executable, err := isInstalled()
		if err != nil {
			return err
		}
		if !installed && dirs.AsRoot {
			destination := path.Join("/usr/local/bin", path.Base(executable))
			fmt.Fprintf(os.Stderr, "+ cp %q %q\n", executable, destination)

			if unlink {
				err = os.Remove(destination)
				if err != nil && os.IsNotExist(err) {
					err = nil
				} else if err != nil {
					return err
				}
			}
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

	fmt.Fprintf(os.Stderr, "+ touch %q\n", destdir+ConductorDeploymentSocketConfigServiceLocation)
	err = os.WriteFile(destdir+ConductorDeploymentSocketConfigServiceLocation, []byte(ConductorDeploymentSocketConfigService), 0644)
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

	fmt.Fprintf(os.Stderr, "+ rm -rf %q\n", destdir+ServicesDirLocation)
	err = os.RemoveAll(destdir + ServicesDirLocation)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	fmt.Fprintf(os.Stderr, ": extract %q\n", destdir+ServicesDirLocation)
	err = ExtractEmbedFs(ServicesDir, "services", destdir+ServicesDirLocation, true)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ systemctl %s daemon-reload\n", dirs.SystemdModeFlag())
	cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "daemon-reload")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	if enable {
		for _, service_name := range []string{"conductor_caddy"} {
			cmd := exec.Command("conductor", "service", "enable")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if now {
				cmd.Args = append(cmd.Args, "--now")
			}
			cmd.Args = append(cmd.Args, service_name)
			fmt.Fprintf(os.Stderr, "+ %s\n", strings.Join(cmd.Args, " "))
			err = cmd.Run()
			if err != nil {
				return err
			}
		}
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

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", destdir+ConductorDeploymentSocketConfigServiceLocation)
	err = os.Remove(destdir + ConductorDeploymentSocketConfigServiceLocation)
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

	fmt.Fprintf(os.Stderr, "+ rm -rf %q\n", destdir+ServicesDirLocation)
	err = os.RemoveAll(destdir + ServicesDirLocation)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ systemctl %s daemon-reload\n", dirs.SystemdModeFlag())
	cmd := exec.Command("systemctl", dirs.SystemdModeFlag(), "daemon-reload")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ systemctl %s enable --now conductor-policy-server.socket\n", dirs.SystemdModeFlag())
	cmd = exec.Command("systemctl", dirs.SystemdModeFlag(), "enable", "--now", "conductor-policy-server.socket")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func ExtractEmbedFs(embedfs embed.FS, embedfs_dir string, location string, log bool) error {
	if embedfs_dir == "" {
		embedfs_dir = "."
	}

	entries, err := embedfs.ReadDir(embedfs_dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return err
		}

		mode := info.Mode()&os.ModePerm | 0600

		if strings.HasSuffix(entry.Name(), ".sh") {
			mode = mode | 0111
		}

		location2 := path.Join(location, entry.Name())

		if entry.IsDir() {
			if log {
				fmt.Fprintf(os.Stderr, "+ install --mode=0%o -d %q\n", mode, location2)
			}

			err := os.MkdirAll(location2, mode)
			if err != nil {
				return err
			}

			err = ExtractEmbedFs(embedfs, path.Join(embedfs_dir, entry.Name()), location2, log)
			if err != nil {
				return err
			}
		} else {

			if log {
				fmt.Fprintf(os.Stderr, "+ install --mode=0%o -t %q %q\n", mode, location, entry.Name())
			}

			err = (func() error {
				f1, err := embedfs.Open(path.Join(embedfs_dir, entry.Name()))
				if err != nil {
					return err
				}

				defer f1.Close()

				f2, err := os.OpenFile(location2, os.O_WRONLY|os.O_CREATE, mode)
				if err != nil {
					return err
				}

				defer f2.Close()

				_, err = io.Copy(f2, f1)
				return err
			})()
			if err != nil {
				return err
			}
		}
	}

	return nil
}
