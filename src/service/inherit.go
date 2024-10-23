package service

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

type InheritFileBase struct {
	Path        string `json:"path"`
	IgnoreError bool   `json:"ignore_error"`
	SetConfig   bool   `json:"set_config"`
}

type InheritFile struct {
	InheritFileBase
	Inherit *InheritedFile
}

type InheritedFile struct {
	Inherit []*InheritFile `json:"inherit"`
}

type InheritedFileSingle struct {
	Inherit *InheritFile `json:"inherit"`
}

func (f *InheritFile) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, &f.Path)
	if err == nil {
		f.IgnoreError = strings.HasPrefix(f.Path, "-")
		f.Path = strings.TrimPrefix(f.Path, "-")

		return nil
	}

	return json.Unmarshal(data, &f.InheritFileBase)
}

func DecodeInherit(data []byte, dir string) (*InheritedFile, error) {
	inherited := &InheritedFile{}
	var inherited_single InheritedFileSingle
	err := json.Unmarshal(data, &inherited_single)

	if err == nil && inherited_single.Inherit != nil {
		inherited.Inherit = append(inherited.Inherit, inherited_single.Inherit)
	} else {
		err = json.Unmarshal(data, inherited)
		if err != nil {
			return nil, err
		}
	}

	if len(inherited.Inherit) > 0 {
		for _, inherit := range inherited.Inherit {
			inherit.Path = join_paths(dir, inherit.Path)

			if strings.HasSuffix(inherit.Path, "/") {
				inherit.Path = filepath.Join(inherit.Path, ConfigName)
			}
		}
	}

	return inherited, nil
}
