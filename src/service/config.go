package service

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type PartialService struct {
	Inherit []string `json:"inherit"` // path to the inherited file
}

type Hook struct {
	Id   string   `json:"id"`
	When string   `json:"when"`
	Exec []string `json:"exec"`
}

type Service struct {
	BasePath            string
	AppName             string            `json:"app_name",omitempty`              // my-app
	InstanceName        string            `json:"instance_name",omitempty`         // staging
	Config              map[string]string `json:"config",omitempty`                // key-value pairs for config and templating, CHANNEL=staging
	PodTemplate         string            `json:"pod_template",omitempty`          // Template file for pod
	ConfigMapTemplate   string            `json:"config_map_template",omitempty`   // ConfigMap template file
	ProxyConfigTemplate string            `json:"proxy_config_template",omitempty` // Template file for the load-balancer config
	Hooks               []*Hook           `json:"hooks",omitempty`
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
		service.Config = map[string]string{}
		service.Hooks = []*Hook{}
	}

	err = json.NewDecoder(f).Decode(partial)
	if err != nil {
		return nil, err
	}

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
	log.Printf("merge: %+v\nwith: %+v\n", layer1, layer2)
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
	return nil
}

func join_paths(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	} else {
		return filepath.Join(base, path)
	}
}
