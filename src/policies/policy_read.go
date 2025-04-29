package policies

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
)

func ReadFromName(name string) (*Policy, error) {
	dir, err := PolicyFind(name)
	if err != nil {
		return nil, err
	}

	return ReadFromDir(dir, name)
}

func ReadFromDir(dir string, name string) (*Policy, error) {
	empty_if_missing := false

	if name == "" {
		name = path.Base(dir)
	}

	fname := path.Join(dir, ConfigName)
	policy := &Policy{
		PolicyDir: dir,
	}

	f, err := os.Open(fname)
	if empty_if_missing && err != nil && os.IsNotExist(err) {
		// Keep default fields only
	} else if err != nil {
		return nil, err
	} else {
		defer f.Close()

		err := json.NewDecoder(f).Decode(policy)
		if err != nil {
			return nil, err
		}
	}

	if policy.Name != "" && policy.Name != name {
		return nil, fmt.Errorf("%s policy name %q should be %q", dir, policy.Name, name)
	}

	return policy, nil
}
