package policies

import (
	"path"

	"github.com/rodaine/table"
)

func Print(name string) error {
	dir, err := PolicyFind(name)
	if err != nil {
		return err
	}

	policy, err := ReadFromDir(dir, "")
	if err != nil {
		return err
	}

	tbl := table.New("Name", path.Base(policy.PolicyDir))
	tbl.AddRow("Path", policy.PolicyDir)

	tbl.Print()

	return nil
}
