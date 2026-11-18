package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mildred/conductor.go/src/caddy"
)

type ServiceSocket struct {
	ServicePart
	ServiceDaemonNotSocketActivated

	Once          bool                  `json:"once,omitempty"`
	Activate      ServiceSocketActivate `json:"activate,omitempty"`
	Directives    []string              `json:"directives,omitempty"`
	RegisterTubes [][]string            `json:"register_tubes,omitempty"`
}

type ServiceSocketActivate struct {
	Part *string
}

func (ssa *ServiceSocketActivate) MarshalJSON() ([]byte, error) {
	if ssa.Part == nil {
		return json.Marshal(false)
	} else if *ssa.Part == "" {
		return json.Marshal(true)
	} else {
		return json.Marshal(*ssa.Part)
	}
}

func (ssa *ServiceSocketActivate) UnmarshalJSON(data []byte) error {
	var val interface{}
	json.Unmarshal(data, &val)

	if val == nil {
		*ssa = ServiceSocketActivate{nil}
		return nil
	}

	switch v := val.(type) {
	case bool:
		if v {
			var part string = ""
			*ssa = ServiceSocketActivate{&part}
		} else {
			*ssa = ServiceSocketActivate{nil}
		}
		return nil
	case string:
		*ssa = ServiceSocketActivate{&v}
		return nil
	}

	return fmt.Errorf("cannot unmarshal activate field %v, must be a part name (string), true, false or null", string(data))
}

func (part *ServiceSocket) FixPaths(dir string) error {
	return nil
}

func (part *ServiceSocket) FillDefaults(service *Service) error {
	return nil
}

func (part *ServiceSocket) ReverseProxyConfigs(ctx context.Context, service *Service) (configs caddy.ConfigItems, err error) {
	return nil, nil
}
