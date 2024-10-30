package policies

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/mildred/conductor.go/src/dirs"
)

var PolyciesConfigDirs = dirs.MultiJoin("policies", dirs.SelfConfigDirs...)
var PolyciesConfigHome = dirs.Join(dirs.SelfConfigHome, "policies")

type MatchBearer struct {
	Token           string `json:"token,omitempty"`             // Raw bearer token
	JWTAlg          string `json:"jwt_alg,omitempty"`           // JWT algorithm
	JWTSecretBase64 string `json:"jwt_secret_base64,omitempty"` // JWT secret
}

type Matcher struct {
	// A matcher matches if all of its defined matchers succeeds. if nothing
	// defined, it does not match
	Always bool           `json:"always,omitempty"` // Always match (conditioned to other defined matchers)
	Never  bool           `json:"never,omitempty"`  // Never match, can be used to disable a matcher
	Skip   bool           `json:"skip,omitempty"`   // Skip matches other than always and never
	All    []*Matcher     `json:"all,omitempty"`    // All must match
	Any    []*Matcher     `json:"any,omitempty"`    // Any must match
	None   []*Matcher     `json:"none,omitempty"`   // None must match
	Bearer []*MatchBearer `json:"bearer,omitempty"` // A bearer token in the list should match
	Origin []string       `json:"origin,omitempty"` // One of these origins must match the Origin header
	Policy string         `json:"policy,omitempty"` // Match policy by name, fail if it does not exist
}

type Policy struct {
	Name  string     `json:"name"`            // Must correspond to file name
	Match []*Matcher `json:"match,omitempty"` // Policy match if any matcher succeeds
}

type PolicyMatcher interface {
	Matching(mc *MatchContext) (bool, error)
}

type Policies struct {
	ByName map[string]*Policy
}

type MatchContext struct {
	*Policies
	Request *http.Request
}

func LoadPolicies() (*Policies, error) {
	var res = &Policies{
		ByName: map[string]*Policy{},
	}

	for _, config_dir := range PolyciesConfigDirs {
		entries, err := os.ReadDir(config_dir)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}

		for _, ent := range entries {
			policy, err := ReadPolicy(path.Join(config_dir, ent.Name()), ent.Name())
			if err != nil {
				return nil, err
			}
			res.ByName[ent.Name()] = policy
		}
	}

	return res, nil
}

func ReadPolicy(filename, name string) (*Policy, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	policy := &Policy{}
	err = json.Unmarshal(data, policy)
	if err != nil {
		return nil, err
	}

	if policy.Name != "" && policy.Name != name {
		return nil, fmt.Errorf("%s policy name %q should be %q", filename, policy.Name, name)
	}

	return policy, nil
}

func (p *Policy) Matching(mc *MatchContext) (res bool, err error) {
	res = false
	for _, m := range p.Match {
		res, err = m.Matching(mc)
		if err != nil || res {
			return
		}
	}
	return
}

func (m *Matcher) Matching(mc *MatchContext) (bool, error) {
	var err error
	var num int

	if m.Always {
		num += 1
	}

	if m.Never {
		return false, nil
	}

	if m.Skip {
		return num > 0, nil
	}

	if len(m.All) > 0 {
		num += 1
		for _, m := range m.All {
			res, err := m.Matching(mc)
			if err != nil {
				return false, err
			} else if !res {
				return false, nil
			}
		}
	}

	if len(m.Any) > 0 {
		num += 1
		res := false
		for _, m := range m.Any {
			res, err = m.Matching(mc)
			if err != nil {
				return false, err
			} else if res {
				break
			}
		}
		if !res {
			return false, nil
		}
	}

	if len(m.None) > 0 {
		num += 1
		for _, m := range m.None {
			res, err := m.Matching(mc)
			if err != nil {
				return false, err
			} else if res {
				return false, nil
			}
		}
	}

	if len(m.Bearer) > 0 {
		num += 1
		res := false
		for _, bearer := range m.Bearer {
			res, err = bearer.Matching(mc)
			if err != nil {
				return false, err
			} else if res {
				break
			}
		}
		if !res {
			return false, nil
		}
	}

	if len(m.Origin) > 0 {
		num += 1
		res := false
		for _, origin := range m.Origin {
			if mc.Request.Header.Get("Origin") == origin {
				res = true
				break
			}
		}
		if !res {
			return false, nil
		}
	}

	if m.Policy != "" {
		num += 1
		policy := mc.ByName[m.Policy]
		if policy == nil {
			return false, nil
		}
		res, err := policy.Matching(mc)
		if err != nil {
			return false, err
		} else if !res {
			return false, nil
		}
	}

	return num > 0, nil
}

func (m *MatchBearer) JWTSecret() ([]byte, error) {
	return base64.RawStdEncoding.DecodeString(m.JWTSecretBase64)
}

func (m *MatchBearer) Matching(mc *MatchContext) (bool, error) {
	for _, auth := range mc.Request.Header.Values("Authorization") {
		s := strings.SplitN(auth, " ", 2)
		if len(s) != 2 || strings.ToLower(s[0]) != "bearer" {
			continue
		}
		token := strings.TrimSpace(s[1])
		num := 0

		if m.Token != "" {
			num += 1
			if m.Token != token {
				continue
			}
		}

		if m.JWTAlg != "" && m.JWTSecretBase64 != "" {
			num += 1

			t, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
				if t.Method.Alg() != m.JWTAlg {
					return nil, fmt.Errorf("algorithm %s is different than %s", m.JWTAlg, t.Method.Alg())
				}
				return m.JWTSecret()
			})
			if err != nil || !t.Valid {
				continue
			}
		}

		return num > 0, nil
	}
	return false, nil
}
