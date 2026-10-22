package utils

import (
	"os"
	"strings"
)

func Expand(s string, env func(string) string) string {
	return os.Expand(s, env)
}

func ExpandVars(s string, vars []string) string {
	return Expand(s, func(env string) string {
		for _, v := range vars {
			if strings.HasPrefix(v, env+"=") {
				split := strings.SplitN(v, "=", 2)
				if len(split) < 2 {
					return ""
				} else {
					return split[1]
				}
			}
		}
		return ""
	})
}

func ExpandAllVars(strs []string, vars []string) []string {
	var res []string
	for _, s := range strs {
		res = append(res, ExpandVars(s, vars))
	}
	return res
}
