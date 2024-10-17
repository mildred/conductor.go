package service

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type PartialService struct {
	Inherit []string `json:"inherit"` // path to the inherited file
}

type Hook struct {
	When string   `jsob:"when"`
	Exec []string `json:"exec"`
}

type Service struct {
	BasePath            string
	AppName             string            `json:"app_name"`              // my-app
	InstanceName        string            `json:"instance_name"`         // staging
	Config              map[string]string `json:"config"`                // key-value pairs for config and templating, CHANNEL=staging
	PodTemplate         string            `json:"pod_template"`          // Template file for pod
	ConfigMapTemplate   string            `json:"config_map_template"`   // ConfigMap template file
	ProxyConfigTemplate string            `json:"proxy_config_template"` // Template file for the load-balancer config
	Hooks               []*Hook           `json:"hooks"`
}

const ConfigName = "conductor-service.json"

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
	}

	json.NewDecoder(f).Decode(partial)
	if len(partial.Inherit) > 0 {
		for _, inherit := range partial.Inherit {
			inherit = join_paths(dir, inherit)

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

	json.NewDecoder(f).Decode(service)

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
			if err := fix_path(dir, &hook.Exec[0], true); err != nil {
				return nil, err
			}
		}
	}

	return service, nil
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
	return nil
}

func join_paths(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	} else {
		return filepath.Join(base, path)
	}
}
