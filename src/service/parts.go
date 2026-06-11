package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mildred/conductor.go/src/caddy"
)

const PodType = "pod"
const FunctionType = "function"
const DaemonType = "daemon"

type ServiceParts []AbstractServicePart

type AbstractServicePart interface {
	PartName() string
	PartType() string
	FixPaths(string) error
	FillDefaults(service *Service) error
	ReverseProxyConfigs(ctx context.Context, service *Service) (configs caddy.ConfigItems, err error)
	GenericPart() *ServicePart
	IsSocketActivated() bool
}

type MinimalServicePart struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type ServicePart struct {
	MinimalServicePart
	PartIdTemplate    string   `json:"part_id_template"`
	ExcludeVars       []string `json:"exclude_vars"`
	ServiceDirectives []string `json:"service_directives,omitempty"`
}

func (part *ServicePart) PartName() string          { return part.Name }
func (part *ServicePart) PartType() string          { return part.Type }
func (part *ServicePart) GenericPart() *ServicePart { return part }

func (parts *ServiceParts) FindPart(name string) AbstractServicePart {
	for _, part := range *parts {
		if part.PartName() == name {
			return part
		}
	}
	return nil
}

func (parts *ServiceParts) FindMainPart() AbstractServicePart {
	if len(*parts) == 1 {
		return (*parts)[0]
	} else {
		return parts.FindPart("")
	}
}

func (parts *ServiceParts) FindPod(name string) *ServicePod {
	res, _ := parts.FindPart(name).(*ServicePod)
	return res
}

func (parts *ServiceParts) FindMainPod() *ServicePod {
	res, _ := parts.FindMainPart().(*ServicePod)
	return res
}

func (parts *ServiceParts) FindDaemon(name string) *ServiceDaemon {
	res, _ := parts.FindPart(name).(*ServiceDaemon)
	return res
}

func (parts *ServiceParts) FindMainDaemon() *ServiceDaemon {
	res, _ := parts.FindMainPart().(*ServiceDaemon)
	return res
}

func (parts *ServiceParts) FindFunction(name string) *ServiceFunction {
	res, _ := parts.FindPart(name).(*ServiceFunction)
	return res
}

func (parts *ServiceParts) FindMainFunction() *ServiceFunction {
	res, _ := parts.FindMainPart().(*ServiceFunction)
	return res
}

func (parts *ServiceParts) FillDefaults(service *Service) error {
	for _, part := range *parts {
		if err := part.FillDefaults(service); err != nil {
			return fmt.Errorf("on %v %v, %v", part.PartType(), part.PartName(), err)
		}
	}
	return nil
}

func (parts *ServiceParts) FixPaths(dir string) error {
	for _, part := range *parts {
		err := part.FixPaths(dir)
		if err != nil {
			return fmt.Errorf("on %v %v, %v", part.PartType(), part.PartName(), err)
		}
	}
	return nil
}

func (parts *ServiceParts) UnmarshalJSON(data []byte) error {
	var raw_parts []json.RawMessage
	err := json.Unmarshal(data, &raw_parts)
	if err != nil {
		return fmt.Errorf("unmarshalling parts, %v", err)
	}

	for _, raw_part := range raw_parts {
		var minimal_part MinimalServicePart
		err = json.Unmarshal(raw_part, &minimal_part)
		if err != nil {
			return fmt.Errorf("unmarshalling part for name and type, %v", err)
		}

		if minimal_part.Type == PodType {
			var existing_pod *ServicePod = parts.FindPod(minimal_part.Name)
			if existing_pod == nil {
				existing_pod = &ServicePod{
					ServicePart: ServicePart{
						MinimalServicePart: minimal_part,
					},
				}
				*parts = append(*parts, existing_pod)
			}
			err = json.Unmarshal(raw_part, existing_pod)
		} else if minimal_part.Type == DaemonType {
			p := parts.FindDaemon(minimal_part.Name)
			if p == nil {
				p = &ServiceDaemon{
					ServicePart: ServicePart{
						MinimalServicePart: minimal_part,
					},
				}
				*parts = append(*parts, p)
			}
			err = json.Unmarshal(raw_part, p)
		} else if minimal_part.Type == FunctionType {
			var existing_function *ServiceFunction = parts.FindFunction(minimal_part.Name)
			if existing_function == nil {
				existing_function = &ServiceFunction{
					ServicePart: ServicePart{
						MinimalServicePart: minimal_part,
					},
				}
				*parts = append(*parts, existing_function)
			}
			err = json.Unmarshal(raw_part, existing_function)
		} else {
			err = fmt.Errorf("cannot decode part type %q", minimal_part.Type)
		}
		if err != nil {
			return fmt.Errorf("unmarshalling pod %q, %v", minimal_part.Name, err)
		}
	}

	return nil
}
