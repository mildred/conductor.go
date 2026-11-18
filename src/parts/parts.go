package parts

import (
	"encoding/json"
	"fmt"
	"slices"
)

type PartType string

const (
	NoPartType   PartType = ""
	PodType               = "pod"
	FunctionType          = "function"
	DaemonType            = "daemon"
	SocketType            = "socket"
)

var AllPartTypes = []PartType{
	NoPartType,
	PodType,
	FunctionType,
	DaemonType,
	SocketType,
}

type ErrUnknownPartType struct {
	UnknownType string
}

func (e *ErrUnknownPartType) Error() string {
	return fmt.Sprintf("unknown part type \"%s\"", e.UnknownType)
}

func (s *PartType) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	if !slices.Contains(AllPartTypes, PartType(str)) {
		return &ErrUnknownPartType{str}
	}

	*s = PartType(str)
	return nil
}
