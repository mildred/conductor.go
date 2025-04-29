package peers

import (
	"github.com/mildred/conductor.go/src/policies"
)

func GetPolicy(policy_name string) (*policies.Policy, error) {
	policy, err := policies.ReadFromName(policy_name)
	if err != nil {
		if _, not_found := err.(*policies.PolicyNotFoundError); !not_found {
			return nil, err
		}
		return policies.Create(policy_name)
	}
	return policy, nil
}
