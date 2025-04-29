package policies

import (
	"path"

	"github.com/rodaine/table"
)

func PrintList() error {
	tbl := table.New("NAME", "PATH").WithPrintHeaders(true)

	list, err := PolicyList()
	if err != nil {
		return err
	}

	for _, policy_dir := range list {
		tbl.AddRow(path.Base(policy_dir), policy_dir)
	}

	tbl.Print()

	return nil
}
