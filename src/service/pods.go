package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mildred/conductor.go/src/caddy"
	"github.com/mildred/conductor.go/src/utils"
)

type ServicePod struct {
	ServicePart
	ServiceDaemonReverseProxy
	ServiceDaemonSocketActivated
	PodTemplate       string `json:"pod_template,omitempty"`        // Template file for pod
	ConfigMapTemplate string `json:"config_map_template,omitempty"` // ConfigMap template file
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

func (pod *ServicePod) ReverseProxyConfigs(ctx context.Context, service *Service) (configs caddy.ConfigItems, err error) {
	return pod.ReverseProxyConfigs2(pod, service)
}

func (pod *ServicePod) FixPaths(dir string) error {
	if err := fix_path(dir, &pod.PartIdTemplate, false); err != nil {
		return err
	}

	if err := fix_path(dir, &pod.PodTemplate, false); err != nil {
		return err
	}

	if err := fix_path(dir, &pod.ConfigMapTemplate, false); err != nil {
		return err
	}
	return nil
}

func (pod *ServicePod) FillDefaults(service *Service) error {
	if pod.PodTemplate == "" {
		pod.PodTemplate = filepath.Join(service.BasePath, "pod.template")
	}
	return nil
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
			existing_pod = &ServicePod{
				ServicePart: ServicePart{
					MinimalServicePart: MinimalServicePart{
						Name: pod_name.Name,
						Type: PodType,
					},
				},
			}
			*pods = append(*pods, existing_pod)
		}
		err = json.Unmarshal(raw_pod, existing_pod)
		if err != nil {
			return fmt.Errorf("unmarshalling pod %q, %v", pod_name.Name, err)
		}
	}

	return nil
}

func (pod *ServicePod) HasSocketProxy() bool {
	return pod.SocketActivated && pod.SocketProxy.Target != ""
}

func (pod *ServicePod) ExpandedSocketDirectives(vars []string) []string {
	return utils.ExpandAllVars(pod.SocketDirectives, vars)
}

func (proxy *SocketProxyConfig) ExpandedTarget(vars []string) string {
	return utils.ExpandVars(proxy.Target, vars)
}

func (proxy *SocketProxyConfig) TargetProtocol(vars []string) string {
	target := proxy.ExpandedTarget(vars)
	if strings.HasPrefix(target, "/") || strings.HasPrefix(target, "@") {
		return "unix"
	} else {
		return "tcp"
	}
}

func (proxy *SocketProxyConfig) CmdlineArgs(vars []string) []string {
	var res []string
	if proxy.ConnectionsMax != nil {
		res = append(res, fmt.Sprintf("--connections-max=%d", *proxy.ConnectionsMax))
	}
	if proxy.ExitIdleTime != "" {
		res = append(res, fmt.Sprintf("--exit-idle-time=%s", proxy.ExitIdleTime))
	}
	if proxy.ProxyProtocol != "" {
		res = append(res, fmt.Sprintf("--proxy-protocol=%s", proxy.ProxyProtocol))
	}
	res = append(res, proxy.Args...)
	return append(res, utils.ExpandVars(proxy.Target, vars))
}
