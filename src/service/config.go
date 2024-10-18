package service

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/coreos/go-systemd/v22/unit"
	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"golang.org/x/crypto/sha3"

	"github.com/mildred/conductor.go/src/dirs"
)

type PartialService struct {
	Inherit []string `json:"inherit"` // path to the inherited file
}

type Hook struct {
	Id   string   `json:"id"`
	When string   `json:"when"`
	Exec []string `json:"exec"`
}

type CaddyMapping struct {
	Port int `json:"port"`
}

type CaddyConfig struct {
	ApiEndpoint string         `json:"api_endpoint"`
	Mapping     []CaddyMapping `json:"mapping"`
}

type Service struct {
	BasePath                string
	Id                      string
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

func LoadServiceDir(dir string, fix_paths bool) (*Service, error) {
	return LoadServiceAndFillDefaults(filepath.Join(dir, ConfigName), fix_paths)
}

func LoadServiceAndFillDefaults(path string, fix_paths bool) (*Service, error) {
	service, err := LoadService(path, fix_paths, nil)
	if err != nil {
		return nil, err
	}

	err = service.FillDefaults()
	if err != nil {
		return nil, err
	}

	err = service.ComputeId()
	if err != nil {
		return nil, err
	}

	return service, nil
}

func LoadService(path string, fix_paths bool, base *Service) (*Service, error) {
	dir := filepath.Dir(path)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	var partial *PartialService = &PartialService{}
	var service *Service = base

	if service == nil {
		service = &Service{}
		service.Config = map[string]string{}
		service.Hooks = []*Hook{}
	}

	err = json.NewDecoder(f).Decode(partial)
	if err != nil {
		return nil, err
	}

	if len(partial.Inherit) > 0 {
		for _, inherit := range partial.Inherit {
			may_fail := strings.HasPrefix(inherit, "-")
			inherit = join_paths(dir, strings.TrimPrefix(inherit, "-"))

			if strings.HasSuffix(inherit, "/") {
				inherit = filepath.Join(inherit, ConfigName)
			}

			if may_fail {
				_, err = os.Stat(inherit)
				if err != nil && os.IsNotExist(err) {
					log.Printf("service: %s could inherit from %s (not found)", dir, inherit)
					continue
				} else if err != nil {
					return nil, err
				}
			}

			log.Printf("service: %s inherit from %s", dir, inherit)
			service, err = LoadService(inherit, true, service)
			if err != nil {
				return nil, err
			}
		}
	}

	if service == nil {
		service = &Service{}
	}

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	last_hooks := service.Hooks
	service.Hooks = []*Hook{}

	err = json.NewDecoder(f).Decode(service)
	if err != nil {
		return nil, err
	}

	service.Hooks = merge_hooks(last_hooks, service.Hooks)

	service.BasePath = dir

	if fix_paths {
		if err := fix_path(dir, &service.PodTemplate, false); err != nil {
			return nil, err
		}

		if err := fix_path(dir, &service.ProxyConfigTemplate, false); err != nil {
			return nil, err
		}

		if err := fix_path(dir, &service.BasePath, false); err != nil {
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
