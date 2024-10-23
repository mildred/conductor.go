package utils

import (
	"sort"
)

func SortedStringKeys[Value any](m map[string]Value) []string {
	keys := make([]string, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}
