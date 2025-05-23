package deployment

import (
	"encoding/json"
	"fmt"

	"github.com/mildred/conductor.go/src/caddy"
	"github.com/mildred/conductor.go/src/service"
)

type DeploymentFunction struct {
	*service.ServiceFunction
}

func (f *DeploymentFunction) ProxyConfig(depl *Deployment) (caddy.ConfigItems, error) {
	var result caddy.ConfigItems
	proxies, err := f.ReverseProxy(depl.Service)
	if err != nil {
		return nil, err
	}

	for _, reverse := range proxies {
		if reverse.UpstreamsPath == "" {
			continue
		}

		config_id := f.CaddyConfigName(depl.Service, reverse.Name)
		config, err := json.Marshal(map[string]interface{}{
			"@id":  config_id + ".upstream",
			"dial": fmt.Sprintf("unix/%s", DeploymentSocketPath(depl.DeploymentName)),
		})
		if err != nil {
			return nil, err
		}

		result = append(result, &caddy.ConfigItem{
			MountPoint: reverse.UpstreamsPath,
			Config:     config,
		})
	}

	return result, nil
}
