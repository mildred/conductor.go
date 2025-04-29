package dirs

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/yookoala/realpath"
)

var home string = os.Getenv("HOME")
var AsRoot bool = os.Geteuid() == 0

var ConfigHome = init_dir("/etc", "XDG_CONFIG_HOME", ".config")
var CacheHome = init_dir("/var/cache", "XDG_CACHE_HOME", ".cache")
var DataHome = init_dir("/usr/share", "XDG_DATA_HOME", ".local/share")
var StateHome = init_dir("/var/lib", "XDG_STATE_HOME", ".local/state")
var RuntimeDir = init_dir("/run", "XDG_RUNTIME_DIR", fmt.Sprintf("/run/user/%d", os.Getuid()))

var DataDirs = init_dirs([]string{"/usr/local/share", "/usr/share"}, "XDG_DATA_DIRS", []string{"/usr/local/share", "/usr/share"}, DataHome)
var ConfigDirs = init_dirs([]string{"/etc"}, "XDG_CONFIG_DIRS", []string{"/etc/xdg"}, ConfigHome)

var SelfName = "conductor"
var SelfConfigHome = path.Join(ConfigHome, SelfName)
var SelfCacheHome = path.Join(CacheHome, SelfName)
var SelfDataHome = path.Join(DataHome, SelfName)
var SelfStateHome = path.Join(StateHome, SelfName)
var SelfRuntimeDir = path.Join(RuntimeDir, SelfName)
var SelfDataDirs = MultiJoin(SelfName, DataDirs...)
var SelfConfigDirs = MultiJoin(SelfName, ConfigDirs...)

func SystemdMode() string {
	if AsRoot {
		return "system"
	} else {
		return "user"
	}
}

func SystemdModeFlag() string {
	if AsRoot {
		return "--system"
	} else {
		return "--user"
	}
}

func init_dir(root_dir, xdg_varname, xdg_dir string) string {
	if AsRoot {
		return root_dir
	} else {
		env := os.Getenv(xdg_varname)
		if env == "" {
			env = path.Join(home, xdg_dir)
		}
		return env
	}
}

func init_dirs(root_dirs []string, xdg_varname string, xdg_dirs []string, xdg_home_dir string) []string {
	if AsRoot {
		return root_dirs
	} else {
		env := os.Getenv(xdg_varname)
		if env != "" {
			var res []string
			for _, dir := range strings.Split(env, ":") {
				if dir != "" {
					res = append(res, dir)
				}
			}
			return append(res, xdg_home_dir)
		}
		return append(xdg_dirs, xdg_home_dir)
	}
}

func MultiJoin(ext string, dirs ...string) []string {
	var res []string
	for _, dir := range dirs {
		res = append(res, path.Join(dir, ext))
	}
	return res
}

func Join(elem ...string) string {
	return path.Join(elem...)
}

func DirConfigRealpath(dir, config_name string) (string, error) {
	service_file, err := realpath.Realpath(filepath.Join(dir, config_name))
	if err != nil {
		return "", err
	}
	_, err = os.Stat(service_file)
	if err != nil {
		return "", err
	}
	return filepath.Dir(service_file), nil
}
