package service

import (
	"encoding/json"
	"fmt"
)

type ConfigValueKind int

const (
	ConfigValueNull   = 0
	ConfigValueString = iota
	ConfigValueTrue
	ConfigValueFalse
)

type ConfigValue struct {
	Kind ConfigValueKind
	Str  string
}

func (v *ConfigValue) String() string {
	return v.Str
}

func (v *ConfigValue) UnmarshalJSON(data []byte) error {
	var value interface{}
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}

	switch val := value.(type) {
	case string:
		*v = ConfigValue{ConfigValueString, val}
	case float64:
		*v = ConfigValue{ConfigValueString, fmt.Sprintf("%v", val)}
	case bool:
		if val {
			*v = ConfigValue{ConfigValueTrue, "true"}
		} else {
			*v = ConfigValue{ConfigValueFalse, "false"}
		}
	case nil:
		*v = ConfigValue{ConfigValueNull, ""}
	default:
		return fmt.Errorf("cannot decode configuration value %s, must be a string", string(data))
	}

	return nil
}

func (v *ConfigValue) MarshalJSON() ([]byte, error) {
	switch v.Kind {
	case ConfigValueNull:
		return []byte("null"), nil
	case ConfigValueTrue:
		return []byte("true"), nil
	case ConfigValueFalse:
		return []byte("false"), nil
	}

	return json.Marshal(v.Str)
}
