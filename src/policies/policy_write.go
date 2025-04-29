package policies

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
)

func (p *Policy) Save() error {
	if p.PolicyDir == "" {
		return fmt.Errorf("Cannot save policy with empty path")
	}

	return p.WriteToDir(p.PolicyDir)
}

func (p *Policy) WriteToDir(dir string) error {
	fname := path.Join(dir, ConfigName)

	err := os.MkdirAll(dir, 0777)
	if err != nil {
		return err
	}

	f, err := os.Create(fname)
	if err != nil {
		return err
	}

	defer f.Close()

	p.PolicyDir = dir
	return json.NewEncoder(f).Encode(p)
}
