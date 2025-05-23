package service_public

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/coreos/go-systemd/v22/dbus"

	"github.com/mildred/conductor.go/src/caddy"

	. "github.com/mildred/conductor.go/src/service"
)

type InspectState struct {
	UnitStatus dbus.UnitStatus `json:"unit_status"`
}

func Inspect(service *Service, state *InspectState) (json.RawMessage, error) {
	proxy_config, err := service.ProxyConfig()
	if err != nil {
		return nil, err
	}

	exported := struct {
		*Service
		Inherit       []*InheritFile    `json:"inherit"`
		State         *InspectState     `json:"_state,omitempty"`
		BasePath      string            `json:"_base_path"`
		FileName      string            `json:"_file_name"`
		ConfigSetFile string            `json:"_config_set_file"`
		Name          string            `json:"_name"`
		Id            string            `json:"_id"`
		ProxyConfig   caddy.ConfigItems `json:"_proxy_config"`
	}{
		Service:       service,
		State:         state,
		Inherit:       service.Inherit.Inherit,
		BasePath:      service.BasePath,
		FileName:      service.FileName,
		ConfigSetFile: service.ConfigSetFile,
		Name:          service.Name,
		Id:            service.Id,
		ProxyConfig:   proxy_config,
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	err = enc.Encode(exported)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func PrintInspect(services ...string) error {
	if len(services) == 0 {
		return PrintInspect(".")
	}

	for _, name := range services {
		service, err := LoadServiceByName(name)
		if err != nil {
			return err
		}

		msg, err := Inspect(service, nil)
		if err != nil {
			return err
		}

		fmt.Println(string(msg))
	}
	return nil
}
