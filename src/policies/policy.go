package policies

import (
	"strings"

	"github.com/mildred/conductor.go/src/dirs"
)

var ConfigName = "policy.json"

var PolyciesConfigDirs = dirs.MultiJoin("policies", dirs.SelfConfigDirs...)
var PolyciesConfigHome = dirs.Join(dirs.SelfConfigHome, "policies")
var PolicyDirs = dirs.MultiJoin("policies", append([]string{dirs.SelfRuntimeDir}, append(dirs.SelfConfigDirs, dirs.SelfDataDirs...)...)...)

type Policy struct {
	Name                 string     `json:"name"` // Must correspond to file name
	PolicyDir            string     `json:"-"`
	Match                []*Matcher `json:"match,omitempty"` // Policy match if any matcher succeeds
	DefaultAuthorization string     `json:"default_authorization,omitempty"`
}

func PolicyRealpath(dir string) (string, error) {
	return dirs.DirConfigRealpath(dir, ConfigName)
}

func PolicyHomeDir(name string) (string, error) {
	if strings.Contains(name, "/") {
		return PolicyRealpath(name)
	} else {
		return dirs.Join(dirs.SelfConfigHome, "policies", name), nil
	}
}

type Policies struct {
	ByName map[string]*Policy
	ByPath map[string]*Policy
}

func (p *Policy) Matching(mc *MatchContext, authorization string, res_meta map[string]string) (res bool, err error, matcher *Matcher) {
	if authorization == "" {
		authorization = p.DefaultAuthorization
	}

	res = false
	for _, m := range p.Match {
		res, err, matcher = m.Matching(mc, authorization, res_meta)
		if err != nil || res {
			return
		}
	}
	return
}

func (p *Policy) FindAllMatchers(meta map[string]string) []*Matcher {
	var res []*Matcher
	for _, m := range p.Match {
		res = append(res, m.FindAllByMeta(meta)...)
	}
	return res
}

func (p *Policy) FindMatcher(meta map[string]string) *Matcher {
	for _, m := range p.Match {
		if res := m.FindByMeta(meta); res != nil {
			return res
		}
	}
	return nil
}
