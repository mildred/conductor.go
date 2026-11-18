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

func UniqueAnywhere[T comparable](s []T) []T {
	seen := make(map[T]struct{}, len(s))
	out := make([]T, 0, len(s))

	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

func Union[V comparable](list *[]V, list2 []V) {
	for _, item2 := range list2 {
		if !slices.Contains(*list, item2) {
			*list = append(*list, item2)
		}
	}
}

func UnionFunc[V any](list *[]V, list2 []V, eql func(v1, v2 V) bool) {
	for _, item2 := range list2 {
		if !slices.ContainsFunc(*list, func(item V) bool {
			return eql(item2, item)
		}) {
			*list = append(*list, item2)
		}
	}
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

func Filter[T any](items []T, keep func(T) bool) []T {
	result := make([]T, 0, len(items))
	for _, item := range items {
		if keep(item) {
			result = append(result, item)
		}
	}
	return result
}

func Map[T, U any](items []T, f func(T) U) []U {
	result := make([]U, 0, len(items))
	for _, item := range items {
		result = append(result, f(item))
	}
	return result
}

func MapHash[T any, U comparable, V any](items []T, f func(T) (U, V)) map[U]V {
	result := make(map[U]V, len(items))
	for _, item := range items {
		u, v := f(item)
		result[u] = v
	}
	return result
}
