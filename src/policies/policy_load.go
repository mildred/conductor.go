package policies

import ()

func LoadPolicies() (*Policies, error) {
	var res = &Policies{
		ByName: map[string]*Policy{},
		ByPath: map[string]*Policy{},
	}

	list, err := PolicyList()
	if err != nil {
		return nil, err
	}

	for _, policy_dir := range list {
		policy, err := ReadFromDir(policy_dir, "")
		if err != nil {
			return nil, err
		}
		if res.ByName[policy.Name] == nil {
			res.ByName[policy.Name] = policy
		}
		res.ByPath[policy_dir] = policy
	}

	return res, nil
}
