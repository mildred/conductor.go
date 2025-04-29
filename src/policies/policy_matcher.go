package policies

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type MatchBearer struct {
	Token           string            `json:"token,omitempty"`             // Raw bearer token
	JWTAlg          string            `json:"jwt_alg,omitempty"`           // JWT algorithm
	JWTSecretBase64 string            `json:"jwt_secret_base64,omitempty"` // JWT secret key or shared secret
	JWTKeyBase64    string            `json:"jwt_key_base64,omitempty"`    // JWT public key
	Authorizations  AuthorizationList `json:"authorizations,omitempty"`
}

type AuthorizationList map[string]bool

func (authz AuthorizationList) Get(name string) bool {
	auth, ok := authz[name]
	if !ok {
		auth, ok = authz[""]
	}
	return auth
}

type Matcher struct {
	// A matcher matches if all of its defined matchers succeeds. if nothing
	// defined, it does not match
	Meta           map[string]string `json:"meta,omitempty"`                   // Metadata
	Authorizations AuthorizationMap  `json:"authorizations,omitempty"`         // Transform authorizations
	DefAuthz       AuthorizationList `json:"default_authorizations,omitempty"` // Default authorizations
	Always         bool              `json:"always,omitempty"`                 // Always match (conditioned to other defined matchers)
	Never          bool              `json:"never,omitempty"`                  // Never match, can be used to disable a matcher
	Skip           bool              `json:"skip,omitempty"`                   // Skip matches other than always and never
	All            []*Matcher        `json:"all,omitempty"`                    // All must match
	Any            []*Matcher        `json:"any,omitempty"`                    // Any must match
	None           []*Matcher        `json:"none,omitempty"`                   // None must match
	Bearer         []*MatchBearer    `json:"bearer,omitempty"`                 // A bearer token in the list should match
	Origin         []string          `json:"origin,omitempty"`                 // One of these origins must match the Origin header
	Policy         *PolicyRef        `json:"policy,omitempty"`                 // Match policy by name, fail if it does not exist
}

type PolicyRef PolicyRefImplem

type PolicyRefImplem struct {
	Name           string           `json:"name"`
	Authorizations AuthorizationMap `json:"authorizations,omitempty"` // Authorization transformation
}

// if authorizations is nil, authorizations is reset
// else, authorization is transformed using the map
// if the authorization is not found in the map, try with the empty string key
// if the authorization is not found, forward the authorization as it is
type AuthorizationMap map[string]string

func (a AuthorizationMap) MapAuthorization(authorization string) string {
	auth := authorization
	if a == nil {
		auth = ""
	} else {
		auth = a[auth]
		if auth == "" {
			auth = a[""]
		}
		if auth == "" {
			auth = authorization
		}
	}
	return auth
}

func (pr *PolicyRef) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, &pr.Name)
	if err == nil {
		return nil
	}

	var res PolicyRefImplem
	err = json.Unmarshal(data, &res)
	if err != nil {
		return err
	}

	*pr = PolicyRef(res)
	return nil
}

type PolicyMatcher interface {
	Matching(mc *MatchContext) (bool, error)
}

type MatchContext struct {
	*Policies
	Request *http.Request
}

func (m *Matcher) FindByMeta(meta map[string]string) *Matcher {
	list := m.FindAllByMeta(meta)
	if len(list) > 0 {
		return list[0]
	}
	return nil
}

func (m *Matcher) FindAllByMeta(meta map[string]string) []*Matcher {
	var res []*Matcher

	if m.Meta != nil {
		matching := true
		for k, v := range meta {
			if mv, vfound := m.Meta[k]; (v == "" && vfound) || (mv == v) {
				continue
			}
			matching = false
			break
		}
		if matching {
			res = append(res, m)
		}
	}

	for _, sub := range [][]*Matcher{m.All, m.Any, m.None} {
		for _, matcher := range sub {
			res = append(res, matcher.FindAllByMeta(meta)...)
		}
	}
	return res
}

func (m *Matcher) Matching(mc *MatchContext, authorization string, res_meta map[string]string) (bool, error, *Matcher) {
	var err error
	var num int
	var matcher *Matcher = m

	if m.Authorizations != nil {
		authorization = m.Authorizations.MapAuthorization(authorization)
	}

	if m.Always {
		num += 1
	}

	if m.Never {
		return false, nil, m
	}

	if m.Skip {
		return num > 0, nil, m
	}

	if len(m.All) > 0 {
		num += 1
		for _, m := range m.All {
			res, err, _ := m.Matching(mc, authorization, res_meta)
			if err != nil {
				return false, err, m
			} else if !res {
				return false, nil, m
			}
		}
	}

	if len(m.Any) > 0 {
		num += 1
		res := false
		for _, m := range m.Any {
			res, err, matcher = m.Matching(mc, authorization, res_meta)
			if err != nil {
				return false, err, m
			} else if res {
				break
			}
		}
		if !res {
			return false, nil, m
		}
	}

	if len(m.None) > 0 {
		num += 1
		for _, m := range m.None {
			res, err, mat := m.Matching(mc, authorization, res_meta)
			if err != nil {
				return false, err, mat
			} else if res {
				return false, nil, mat
			}
		}
	}

	if len(m.Bearer) > 0 {
		num += 1
		res := false
		for _, bearer := range m.Bearer {
			res, err = bearer.Matching(mc, authorization, m.DefAuthz)
			if err != nil {
				return false, err, m
			} else if res {
				break
			}
		}
		if !res {
			return false, nil, m
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
			return false, nil, m
		}
	}

	if m.Policy != nil && m.Policy.Name != "" {
		num += 1
		policy := mc.ByName[m.Policy.Name]
		if policy == nil {
			return false, nil, m
		}
		auth := m.Policy.Authorizations.MapAuthorization(authorization)
		var res bool
		res, err, matcher = policy.Matching(mc, auth, res_meta)
		if err != nil {
			return false, err, m
		} else if !res {
			return false, nil, matcher
		}
	}

	if num > 0 && res_meta != nil {
		for k, v := range m.Meta {
			if _, found := res_meta[k]; !found {
				res_meta[k] = v
			}
		}
	}

	return num > 0, nil, matcher
}

func (m *MatchBearer) JWTSecret() ([]byte, error) {
	if m.JWTSecretBase64 == "" {
		return nil, nil
	}

	return base64.RawStdEncoding.DecodeString(m.JWTSecretBase64)
}

func (m *MatchBearer) JWTKey() ([]byte, error) {
	if m.JWTKeyBase64 == "" {
		return nil, nil
	}

	return base64.RawStdEncoding.DecodeString(m.JWTKeyBase64)
}

func (m *MatchBearer) Matching(mc *MatchContext, authorization string, default_authz AuthorizationList) (bool, error) {
	authz := m.Authorizations
	if authz == nil {
		authz = default_authz
	}
	if authz != nil {
		if !authz.Get(authorization) {
			return false, nil
		}
	}

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

		if m.JWTAlg != "" && (m.JWTSecretBase64 != "" || m.JWTKeyBase64 != "") {
			num += 1

			t, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
				if t.Method.Alg() != m.JWTAlg {
					return nil, fmt.Errorf("algorithm %s is different than %s", m.JWTAlg, t.Method.Alg())
				}
				key, err := m.JWTKey()
				if err != nil {
					return nil, err
				}
				if key == nil {
					key, err = m.JWTSecret()
				}
				if err != nil {
					return nil, err
				}
				return key, nil
			})
			if err != nil || !t.Valid {
				continue
			}
		}

		return num > 0, nil
	}
	return false, nil
}
