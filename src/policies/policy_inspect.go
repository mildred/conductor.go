package policies

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func InspectCommand(name string) error {
	dir, err := PolicyFind(name)
	if err != nil {
		return err
	}

	policy, err := ReadFromDir(dir, "")
	if err != nil {
		return err
	}

	msg, err := policy.Inspect()
	if err != nil {
		return err
	}

	fmt.Println(string(msg))

	return nil
}

func (p *Policy) Inspect() (json.RawMessage, error) {
	exported := struct {
		*Policy
		Path string `json:"_path"`
	}{
		Policy: p,
		Path:   p.PolicyDir,
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	err := enc.Encode(exported)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
