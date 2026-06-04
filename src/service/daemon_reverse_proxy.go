package service

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/mildred/conductor.go/src/caddy"
)

type PartCaddyConfigName interface {
	CaddyConfigName(service *Service, proxy_name string) string
}

type ServiceDaemonReverseProxy struct {
	ProvidedReverseProxy []ServiceDaemonProxyConfig `json:"reverse_proxy"`
}

type ServiceDaemonProxyConfig struct {
	Name          string          `json:"name"`
	MountPoint    string          `json:"mount_point"`
	Route         json.RawMessage `json:"route"`
	UpstreamsPath string          `json:"upstreams_path"`
	Port          int             `json:"port"`
}

func (pod *ServiceDaemonReverseProxy) ReverseProxy(part PartCaddyConfigName, service *Service) ([]ServiceDaemonProxyConfig, error) {
	var res []ServiceDaemonProxyConfig
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
			err := caddyConfigSetId(&proxy.Route, part.CaddyConfigName(service, proxy.Name))
			if err != nil {
				return nil, err
			}
		}

		if proxy.MountPoint == "" {
			proxy.MountPoint = "conductor-server/routes"
		}

		res = append(res, proxy)
	}
	return res, nil
}

func (pod *ServiceDaemonReverseProxy) ReverseProxyConfigs2(part PartCaddyConfigName, service *Service) (configs caddy.ConfigItems, err error) {
	res, err := pod.ReverseProxy(part, service)
	if err != nil {
		return nil, err
	}

	for _, proxy := range res {
		configs = append(configs, &caddy.ConfigItem{
			MountPoint: proxy.MountPoint,
			Config:     proxy.Route,
		})
	}
	return
}
