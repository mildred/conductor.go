package utils

import (
	"strings"
)

func ToDNSName(s string) string {
	return strings.ReplaceAll(s, "_", "-")
}
