package service

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

type ServicePod struct {
	Name              string                `json:"name"`
	ServiceDirectives []string              `json:"service_directives,omitempty"`
	PodTemplate       string                `json:"pod_template,omitempty"`        // Template file for pod
	ConfigMapTemplate string                `json:"config_map_template,omitempty"` // ConfigMap template file
	ReverseProxy      ServicePodProxyConfig `json:"reverse_proxy"`
}

type ServicePodProxyConfig struct {
	MountPoint    string          `json:"mount_point"`
	Route         json.RawMessage `json:"proxy_route"` // TODO:
	UpstreamsPath string          `json:"upstreams_path"`
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
		if err := fix_path(dir, &pod.PodTemplate, false); err != nil {
			return err
		}

		if err := fix_path(dir, &pod.ConfigMapTemplate, false); err != nil {
			return err
		}
	}
	return nil
}

func (pods *ServicePods) FillDefaults(service *Service) {
	for _, pod := range *pods {
		if pod.PodTemplate == "" {
			pod.PodTemplate = filepath.Join(service.BasePath, "pod.template")
		}
	}
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
