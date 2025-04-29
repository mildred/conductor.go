package peers

import (
	"github.com/rodaine/table"
)

func PrintList(policy_name string) error {
	policy, err := GetPolicy(policy_name)
	if err != nil {
		return err
	}

	matchers := policy.FindAllMatchers(map[string]string{"peer-hostname": ""})

	tbl := table.New("HOSTNAME", "POLICY").WithPrintHeaders(true)

	for _, m := range matchers {
		tbl.AddRow(m.Meta["peer-hostname"], policy_name)
	}

	tbl.Print()

	return nil
}
