package deployment

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/mildred/conductor.go/src/caddy"
	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/tmpl"
)

type DeploymentPod struct {
	*service.ServicePod
	IPAddress string `json:"ip_address"`
}

type DeploymentDaemon struct {
	*service.ServiceDaemon
}

func (pod *DeploymentPod) TemplatePod(ctx context.Context, depl *Deployment) error {
	log.Printf("prepare: Templating the pod\n")
	vars, err := depl.Vars()
	if err != nil {
		return err
	}

	res, err := tmpl.RunTemplate(ctx, pod.PodTemplate, vars)
	if err != nil {
		return err
	}
	depl.TemplatedPod = res

	res, err = tmpl.RunTemplate(ctx, pod.ConfigMapTemplate, vars)
	if err != nil {
		return err
	}
	depl.TemplatedConfigMap = res

	return nil
}

func (pod *DeploymentPod) ProxyConfig(depl *Deployment) (caddy.ConfigItems, error) {
	var result caddy.ConfigItems
	proxies, err := pod.ReverseProxy(pod, depl.Service)
	if err != nil {
		return nil, err
	}

	for _, reverse := range proxies {
		if reverse.UpstreamsPath == "" {
			continue
		}

		config_id := pod.CaddyConfigName(depl.Service, reverse.Name)
		config, err := json.Marshal(map[string]interface{}{
			"@id":  config_id + ".upstream",
			"dial": fmt.Sprintf("%s:%d", pod.IPAddress, reverse.Port),
		})
		if err != nil {
			return nil, err
		}

		result = append(result, &caddy.ConfigItem{
			MountPoint: reverse.UpstreamsPath,
			Config:     config,
			Source:     "pod",
		})
	}

	return result, nil
}
