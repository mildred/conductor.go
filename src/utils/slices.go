package utils

import "slices"

func Compact[V comparable](args ...V) []V {
	var empty V
	var res []V
	for _, s := range args {
		if s != empty {
			res = append(res, s)
		}
	}
	return res
}

func CompactFunc[V any](isempty func(v V) bool, args ...V) []V {
	var res []V
	for _, s := range args {
		if !isempty(s) {
			res = append(res, s)
		}
	}
	return res
}

func IntersectHoles[V comparable](list *[]V, list2 []V) {
	var empty V
	for i, item := range *list {
		if !slices.Contains(list2, item) {
			(*list)[i] = empty
		}
	}
}

func IntersectHolesFunc[V any](list *[]V, list2 []V, eql func(v1, v2 V) bool) {
	var empty V
	for i, item := range *list {
		found := slices.ContainsFunc(list2, func(item2 V) bool {
			return eql(item, item2)
		})
		if !found {
			(*list)[i] = empty
		}
	}
}
