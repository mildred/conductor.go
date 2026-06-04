package service

import (
	"context"
	"fmt"

	"github.com/mildred/conductor.go/src/caddy"
)

type ServiceDaemon struct {
	ServicePart
	ServiceDaemonReverseProxy
	ServiceDaemonNotSocketActivated
	Exec []string `json:"exec"`
}

func (part *ServiceDaemon) ReverseProxyConfigs(ctx context.Context, service *Service) (configs caddy.ConfigItems, err error) {
	return part.ReverseProxyConfigs2(part, service)
}

func (part *ServiceDaemon) FixPaths(dir string) error {
	if err := fix_path(dir, &part.PartIdTemplate, false); err != nil {
		return err
	}
	return nil
}

func (part *ServiceDaemon) FillDefaults(service *Service) error {
	return nil
}

func (part *ServiceDaemon) CaddyConfigName(service *Service, name string) string {
	return fmt.Sprintf("conductor-daemon.%s.%s.%s.%s", service.AppName, service.InstanceName, part.Name, name)
}
