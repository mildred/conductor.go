package service

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/mildred/conductor.go/src/caddy"
)

type ServicePod struct {
	Name                 string                  `json:"name"`
	PartIdTemplate       string                  `json:"part_id_template"`
	ExcludeVars          []string                `json:"exclude_vars"`
	ServiceDirectives    []string                `json:"service_directives,omitempty"`
	PodTemplate          string                  `json:"pod_template,omitempty"`        // Template file for pod
	ConfigMapTemplate    string                  `json:"config_map_template,omitempty"` // ConfigMap template file
	ProvidedReverseProxy []ServicePodProxyConfig `json:"reverse_proxy"`
}

type ServicePodProxyConfig struct {
	Name          string          `json:"name"`
	MountPoint    string          `json:"mount_point"`
	Route         json.RawMessage `json:"route"`
	UpstreamsPath string          `json:"upstreams_path"`
	Port          int             `json:"port"`
}

type ServicePods []*ServicePod

func (pods *ServicePods) FindPod(name string) *ServicePod {
	for _, pod := range *pods {
		if pod.Name == name {
			return pod
		}
	}
	return nil
}

func (pods *ServicePods) FindMainPod() *ServicePod {
	if len(*pods) == 1 {
		return (*pods)[0]
	} else {
		return pods.FindPod("")
	}
}

func (pods *ServicePods) FixPaths(dir string) error {
	for _, pod := range *pods {
		if err := fix_path(dir, &pod.PartIdTemplate, false); err != nil {
			return err
		}

		if err := fix_path(dir, &pod.PodTemplate, false); err != nil {
			return err
		}

		if err := fix_path(dir, &pod.ConfigMapTemplate, false); err != nil {
			return err
		}
	}
	return nil
}

func (pods *ServicePods) FillDefaults(service *Service) error {
	for _, pod := range *pods {
		if pod.PodTemplate == "" {
			pod.PodTemplate = filepath.Join(service.BasePath, "pod.template")
		}
	}
	return nil
}

func (pod *ServicePod) ReverseProxy(service *Service) (res []ServicePodProxyConfig, err error) {
	var names []string

	for i, proxy := range pod.ProvidedReverseProxy {
		if proxy.UpstreamsPath == "" {
			continue
		}

		if proxy.Name == "" {
			proxy.Name = fmt.Sprintf("%d", i)
		}

		if slices.Contains(names, proxy.Name) {
			return nil, fmt.Errorf("Reverse proxy configuration %+v appears more than once", proxy.Name)
		}
		names = append(names, proxy.Name)

		if len(proxy.Route) != 0 {
			err := caddyConfigSetId(&proxy.Route, pod.CaddyConfigName(service, proxy.Name))
			if err != nil {
				return nil, err
			}
		}

		if proxy.MountPoint == "" {
			proxy.MountPoint = "conductor-server/routes"
		}

		res = append(res, proxy)
	}
	return
}

func (pod *ServicePod) ReverseProxyConfigs(service *Service) (configs caddy.ConfigItems, err error) {
	proxies, err := pod.ReverseProxy(service)
	if err != nil {
		return nil, err
	}

	for _, proxy := range proxies {
		configs = append(configs, &caddy.ConfigItem{
			MountPoint: proxy.MountPoint,
			Config:     proxy.Route,
		})
	}
	return
}

func (pod *ServicePod) CaddyConfigName(service *Service, name string) string {
	return fmt.Sprintf("conductor-pod.%s.%s.%s.%s", service.AppName, service.InstanceName, pod.Name, name)
}

func (pods *ServicePods) UnmarshalJSON(data []byte) error {
	var raw_pods []json.RawMessage
	err := json.Unmarshal(data, &raw_pods)
	if err != nil {
		return fmt.Errorf("unmarshalling pods, %v", err)
	}

	for _, raw_pod := range raw_pods {
		var pod_name struct {
			Name string `json:"name"`
		}
		err = json.Unmarshal(raw_pod, &pod_name)
		if err != nil {
			return fmt.Errorf("unmarshalling pod for name, %v", err)
		}
		var existing_pod *ServicePod = pods.FindPod(pod_name.Name)
		if existing_pod == nil {
			existing_pod = &ServicePod{}
			*pods = append(*pods, existing_pod)
		}
		err = json.Unmarshal(raw_pod, existing_pod)
		if err != nil {
			return fmt.Errorf("unmarshalling pod %q, %v", pod_name.Name, err)
		}
	}

	return nil
}
