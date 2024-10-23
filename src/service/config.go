package service

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/coreos/go-systemd/v22/unit"
	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/yookoala/realpath"
	"golang.org/x/crypto/sha3"

	"github.com/mildred/conductor.go/src/dirs"
)

type Hook struct {
	Id         string   `json:"id"`
	When       string   `json:"when"`
	Exec       []string `json:"exec"`
	TimeoutSec int64    `json:"timeout_sec"`
}

type CaddyMapping struct {
	Port int `json:"port"`
}

type CaddyConfig struct {
	ApiEndpoint string         `json:"api_endpoint"`
	Mapping     []CaddyMapping `json:"mapping"`
}

type Service struct {
	BasePath                string            `json:"-"`
	FileName                string            `json:"-"`
	ConfigSetFile           string            `json:"-"`
	Name                    string            `json:"-"`
	Id                      string            `json:"-"`
	Inherit                 *InheritedFile    `json:"-"`
	AppName                 string            `json:"app_name",omitempty`              // my-app
	InstanceName            string            `json:"instance_name",omitempty`         // staging
	Config                  map[string]string `json:"config",omitempty`                // key-value pairs for config and templating, CHANNEL=staging
	PodTemplate             string            `json:"pod_template",omitempty`          // Template file for pod
	ConfigMapTemplate       string            `json:"config_map_template",omitempty`   // ConfigMap template file
	ProxyConfigTemplate     string            `json:"proxy_config_template",omitempty` // Template file for the load-balancer config
	Hooks                   []*Hook           `json:"hooks",omitempty`
	CaddyLoadBalancer       CaddyConfig       `json:"caddy_load_balancer"`
	DisplayServiceConfig    []string          `json:"display_service_config"`
	DisplayDeploymentConfig []string          `json:"display_deployment_config"`
}

const ConfigName = "conductor-service.json"

var ServiceDirs = dirs.MultiJoin("services", append([]string{dirs.SelfRuntimeDir}, append(dirs.SelfConfigDirs, dirs.SelfDataDirs...)...)...)

func ServiceFileByName(name string) (string, error) {
	if name == "." || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "./") {
		name = strings.TrimSuffix(name, "/"+ConfigName)
		service_file, err := realpath.Realpath(filepath.Join(name, ConfigName))
		if err != nil {
			return "", err
		}
		_, err = os.Stat(service_file)
		if err != nil {
			return "", err
		}
		return service_file, nil
	} else if strings.Contains(name, "/") {
		return "", fmt.Errorf("Invalid service name with '/' within, if you mean a part, it must start with '/' or './'")
	} else {
		for _, dir := range ServiceDirs {
			service_file := filepath.Join(dir, name, ConfigName)
			_, err := os.Stat(service_file)
			if err != nil && !os.IsNotExist(err) {
				return "", err
			} else if err != nil {
				// ignore error, this is not a valid service dir
				continue
			}

			// We found the service_dir
			service_file, err = realpath.Realpath(service_file)
			if err != nil {
				return "", err
			}
			return service_file, nil
		}
		return "", fmt.Errorf("Service %q is not found", name)
	}
}

func ServiceDirByName(name string) (string, error) {
	file, err := ServiceFileByName(name)
	if err != nil {
		return "", err
	}

	return filepath.Dir(file), err
}

func ServiceUnitByName(name string) (string, error) {
	file, err := ServiceDirByName(name)
	if err != nil {
		return "", err
	}

	return ServiceUnit(file), err
}

func ServiceRealpath(service_dir string) (string, error) {
	service_file, err := realpath.Realpath(filepath.Join(service_dir, ConfigName))
	if err != nil {
		return "", err
	}
	_, err = os.Stat(service_file)
	if err != nil {
		return "", err
	}
	return filepath.Dir(service_file), nil
}

// func ValidateServiceNameFromDir(service_dir, name_hint string) (string, error) {
// 	stat, err := os.Stat(filepath.Join(service_dir, ConfigName))
// 	if err != nil {
// 		return "", err
// 	}
//
// 	for _, services_dir := range ServiceDirs {
// 		dir := filepath.Join(services_dir, name_hint)
// 		st, err := os.Stat(filepath.Join(dir, ConfigName))
// 		if err != nil && !os.IsNotExist(err) {
// 			return "", err
// 		} else if err != nil {
// 			// ignore error, this is not a valid service dir
// 			continue
// 		}
//
// 		if os.SameFile(stat, st) {
// 			return name_hint, nil
// 		} else {
// 			return "", nil
// 		}
// 	}
//
// 	return "", nil
// }

func ServiceNameFromFile(service_file string) (string, error) {
	stat, err := os.Stat(service_file)
	if err != nil {
		return "", err
	}

	names := map[string]bool{}

	for _, services_dir := range ServiceDirs {
		entries, err := os.ReadDir(services_dir)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}

		for _, ent := range entries {
			if names[ent.Name()] {
				// Name is shadowed
				continue
			}
			names[ent.Name()] = true

			dir := filepath.Join(services_dir, ent.Name())
			st, err := os.Stat(filepath.Join(dir, ConfigName))
			if err != nil && !os.IsNotExist(err) {
				return "", err
			} else if err != nil {
				// ignore error, this is not a valid service dir
				continue
			} else {
				if os.SameFile(stat, st) {
					return ent.Name(), nil
				}
			}
		}
	}

	return "", nil
}

func ServiceUnit(path string) string {
	return fmt.Sprintf("conductor-service@%s.service", unit.UnitNamePathEscape(path))
}

func ServiceConfigUnit(path string) string {
	return fmt.Sprintf("conductor-service-config@%s.service", unit.UnitNamePathEscape(path))
}

func ServiceDirFromUnit(u string) string {
	s := strings.TrimSuffix(u, ".service")
	splits := strings.SplitN(s, "@", 2)
	if len(splits) < 2 {
		return ""
	} else {
		return unit.UnitNamePathUnescape(splits[1])
	}
}

func LoadServiceByName(name string) (*Service, error) {
	service_file, err := ServiceFileByName(name)
	if err != nil {
		return nil, err
	}

	return LoadServiceFile(service_file)
}

func LoadServiceDir(dir string) (*Service, error) {
	return LoadServiceFile(filepath.Join(dir, ConfigName))
}

func LoadServiceFile(path string) (*Service, error) {
	path, err := realpath.Realpath(path)
	if err != nil {
		return nil, err
	}

	service, err := loadService(path, true, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("while loading service %q, %v", path, err)
	}

	err = service.FillDefaults()
	if err != nil {
		return nil, err
	}

	err = service.ComputeId()
	if err != nil {
		return nil, err
	}

	name, err := ServiceNameFromFile(path)
	if err != nil {
		return nil, err
	}
	service.Name = name

	return service, nil
}

func loadService(path string, fix_paths bool, base *Service, inh *InheritFile) (*Service, error) {
	dir := filepath.Dir(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	inherit, err := DecodeInherit(data, dir)
	if err != nil {
		return nil, fmt.Errorf("while reading %q, %v", path, err)
	}

	var service *Service = base

	if service == nil {
		service = &Service{}
		service.Inherit = inherit
		service.Config = map[string]string{}
		service.Hooks = []*Hook{}
	} else if inh != nil {
		inh.Inherit = inherit
	}

	if len(inherit.Inherit) > 0 {
		has_config_set_file := service.ConfigSetFile != ""

		for _, inherit := range inherit.Inherit {
			if inherit.IgnoreError {
				_, err = os.Stat(inherit.Path)
				if err != nil && os.IsNotExist(err) {
					log.Printf("service: %s could inherit from %s (not found)", dir, inherit.Path)
					continue
				} else if err != nil {
					return nil, err
				}
			}

			log.Printf("service: %s inherit from %s", dir, inherit.Path)
			service, err = loadService(inherit.Path, true, service, inherit)
			if err != nil {
				return nil, err
			}

			if !has_config_set_file && inherit.SetConfig {
				service.ConfigSetFile = inherit.Path
			}
		}
	}

	if service == nil {
		service = &Service{}
	}

	last_hooks := service.Hooks
	service.Hooks = []*Hook{}

	err = json.Unmarshal(data, service)
	if err != nil {
		return nil, err
	}

	service.Hooks = merge_hooks(last_hooks, service.Hooks)

	service.BasePath = dir

	service.FileName = path

	if fix_paths {
		if err := fix_path(dir, &service.PodTemplate, false); err != nil {
			return nil, err
		}

		if err := fix_path(dir, &service.ConfigMapTemplate, false); err != nil {
			return nil, err
		}

		if err := fix_path(dir, &service.ProxyConfigTemplate, false); err != nil {
			return nil, err
		}

		for _, hook := range service.Hooks {
			if len(hook.Exec) == 0 {
				continue
			}
			if err := fix_path(dir, &hook.Exec[0], true); err != nil {
				return nil, err
			}
		}
	}

	return service, nil
}

func merge_hooks(layer1 []*Hook, layer2 []*Hook) []*Hook {
	var result []*Hook = append([]*Hook{}, layer1...)
	for _, layered_hook := range layer2 {
		var i = -1
		if layered_hook.Id != "" {
			i = slices.IndexFunc(result, func(h *Hook) bool { return h.Id == layered_hook.Id })
		}
		if i == -1 {
			// new hook, append to results
			result = append(result, layered_hook)
		} else {
			// replace hook with the new one
			result[i] = layered_hook
		}
	}
	return result
}

func fix_path(dir string, path *string, is_executable bool) error {
	if *path != "" && dir != "" && !strings.HasPrefix(*path, "/") && (!is_executable || strings.Contains(*path, "/")) {
		p := join_paths(dir, *path)
		*path = p
	}
	return nil
}

func (service *Service) FillDefaults() error {
	if service.PodTemplate == "" {
		service.PodTemplate = filepath.Join(service.BasePath, "pod.template")
	}
	if service.ProxyConfigTemplate == "" {
		service.PodTemplate = filepath.Join(service.BasePath, "proxy-config.template")
	}
	if service.CaddyLoadBalancer.ApiEndpoint == "" {
		service.CaddyLoadBalancer.ApiEndpoint = "http://localhost:2019"
	}
	if service.ConfigSetFile == "" {
		service.ConfigSetFile = service.FileName
	}
	return nil
}

func (service *Service) ComputeId() error {
	data, err := json.Marshal(service)
	if err != nil {
		return err
	}

	canon, err := jsoncanonicalizer.Transform(data)
	if err != nil {
		return err
	}

	shake := sha3.NewShake256()
	_, err = shake.Write(canon)
	if err != nil {
		return err
	}

	output := make([]byte, 16)
	_, err = shake.Read(output)
	if err != nil {
		return err
	}

	service.Id = fmt.Sprintf("%x", output)
	return nil
}

func join_paths(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	} else {
		return filepath.Join(base, path)
	}
}

func (service *Service) Vars() []string {
	var vars []string = []string{
		"CONDUCTOR_APP=" + service.AppName,
		"CONDUCTOR_INSTANCE=" + service.InstanceName,
	}
	for k, v := range service.Config {
		vars = append(vars, fmt.Sprintf("%s=%s", k, v))
	}
	return vars
}
