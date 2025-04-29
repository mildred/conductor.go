package peers

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/mildred/conductor.go/src/policies"
)

func PrintInviteToken(policy_name string) error {
	policy, err := GetPolicy(policy_name)
	if err != nil {
		return err
	}

	matchers := policy.FindAllMatchers(map[string]string{"peer-invite": ""})

	count := 0

	for _, m := range matchers {
		for _, b := range m.Bearer {
			count = count + 1
			fmt.Println(b.JWTSecretBase64)
		}
	}

	if count == 0 {
		pub_key, sec_key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}

		sec64 := base64.StdEncoding.EncodeToString(sec_key)
		pub64 := base64.StdEncoding.EncodeToString(pub_key)

		fmt.Println(sec64)

		policy.Match = append(policy.Match, &policies.Matcher{
			Meta: map[string]string{
				"peer-invite": "1",
			},
			DefAuthz: policies.AuthorizationList{
				"peer-invite": true,
			},
			Bearer: []*policies.MatchBearer{
				&policies.MatchBearer{
					JWTAlg:          "EdDSA",
					JWTSecretBase64: sec64,
					JWTKeyBase64:    pub64,
				},
			},
		})
		policy.Save()
	}

	return nil
}
