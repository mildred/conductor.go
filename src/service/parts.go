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
			err = parts.UnmarshalJSONPod(minimal_part.Name, raw_part)
		} else if minimal_part.Type == DaemonType {
			err = parts.UnmarshalJSONDaemon(minimal_part.Name, raw_part)
		} else if minimal_part.Type == FunctionType {
			err = parts.UnmarshalJSONFunction(minimal_part.Name, raw_part)
		} else {
			err = fmt.Errorf("cannot decode part type %q", minimal_part.Type)
		}
		if err != nil {
			return fmt.Errorf("unmarshalling pod %q, %v", minimal_part.Name, err)
		}
	}

	return nil
}

func (parts *ServiceParts) UnmarshalJSONPod(name string, raw_part json.RawMessage) error {
	var existing_pod *ServicePod = parts.FindPod(name)
	if existing_pod == nil {
		existing_pod = &ServicePod{
			ServicePart: ServicePart{
				MinimalServicePart: MinimalServicePart{name, PodType},
			},
		}
		if p := parts.FindPart(name); p != nil {
			return fmt.Errorf("Cannot add new part %+v of type %s, a %v if the same name exists", name, PodType, p.PartType())
		}
		*parts = append(*parts, existing_pod)
	}
	return json.Unmarshal(raw_part, existing_pod)
}

func (parts *ServiceParts) UnmarshalJSONDaemon(name string, raw_part json.RawMessage) error {
	p := parts.FindDaemon(name)
	if p == nil {
		p = &ServiceDaemon{
			ServicePart: ServicePart{
				MinimalServicePart: MinimalServicePart{name, DaemonType},
			},
		}
		if p := parts.FindPart(name); p != nil {
			return fmt.Errorf("Cannot add new part %+v of type %s, a %v if the same name exists", name, DaemonType, p.PartType())
		}
		*parts = append(*parts, p)
	}
	return json.Unmarshal(raw_part, p)
}

func (parts *ServiceParts) UnmarshalJSONFunction(name string, raw_part json.RawMessage) error {
	var existing_function *ServiceFunction = parts.FindFunction(name)
	if existing_function == nil {
		existing_function = &ServiceFunction{
			ServicePart: ServicePart{
				MinimalServicePart: MinimalServicePart{name, FunctionType},
			},
		}
		if p := parts.FindPart(name); p != nil {
			return fmt.Errorf("Cannot add new part %+v of type %s, a %v if the same name exists", name, FunctionType, p.PartType())
		}
		*parts = append(*parts, existing_function)
	}
	return json.Unmarshal(raw_part, existing_function)
}
