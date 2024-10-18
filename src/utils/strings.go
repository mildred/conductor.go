package utils

func Compact(args ...string) []string {
	var res []string
	for _, s := range args {
		if s != "" {
			res = append(res, s)
		}
	}
	return res
}

func IntersectHoles(list *[]string, list2 []string) {
	for i, item := range *list {
		found := false
		for _, item2 := range list2 {
			if item == item2 {
				found = true
				break
			}
		}
		if !found {
			(*list)[i] = ""
		}
	}
}
