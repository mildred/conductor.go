package service

import (
	"strings"

	"github.com/mildred/conductor.go/src/utils"
)

type HelpFlag struct {
	Name string
	Help string
}

func (c *ServiceCommand) GetHelpFlags() []HelpFlag {
	var res []HelpFlag
	for _, help := range c.HelpFlags {
		if len(help) == 0 {
			continue
		}

		res = append(res, HelpFlag{
			Name: help[0],
			Help: strings.Join(help[1:], " "),
		})
	}
	return res
}

func (c *ServiceCommand) GetTabbedHelpFlags() *utils.Tabbed {
	result := &utils.Tabbed{
		Rows: c.HelpFlags,
	}
	return result.Tabulate()
}
