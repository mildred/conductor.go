package service

import (
	"encoding/json"
	"fmt"

	"github.com/mildred/conductor.go/src/utils"
)

type AddrConfig struct {
	Enabled bool
	Name    *string
}

func (s *AddrConfig) GetName(root_name, default_name string) string {
	if !s.Enabled {
		return ""
	}

	part_name := default_name
	if s.Name != nil {
		part_name = *s.Name
	}

	if part_name != "" {
		return utils.ToDNSName(part_name) + "." + utils.ToDNSName(root_name)
	} else {
		return utils.ToDNSName(root_name)
	}
}

func (s *AddrConfig) UnmarshalJSON(data []byte) error {
	var val interface{}
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}

	if val == nil {
		*s = AddrConfig{
			Enabled: false,
			Name:    nil,
		}
		return nil
	}

	switch v := val.(type) {
	case bool:
		*s = AddrConfig{
			Enabled: v,
			Name:    nil,
		}
		return nil
	case string:
		*s = AddrConfig{
			Enabled: true,
			Name:    &v,
		}
		return nil
	}

	return fmt.Errorf("Invalid JSON data %s for address config, expect boolean or string", string(data))
}

func (s *AddrConfig) MarshalJSON() ([]byte, error) {
	if !s.Enabled {
		return json.Marshal(false)
	} else if s.Name == nil {
		return json.Marshal(true)
	} else {
		return json.Marshal(s.Name)
	}
}
