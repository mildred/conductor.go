package utils

import (
	"encoding/json"
	"fmt"
	"time"
)

type JSONDuration time.Duration

func (d JSONDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *JSONDuration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = JSONDuration(time.Duration(value))
		return nil
	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = JSONDuration(tmp)
		return nil
	default:
		return fmt.Errorf("invalid duration %+v", v)
	}
}
