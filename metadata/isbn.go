package metadata

import (
	"strings"
	"unicode"
)

func NormalizeISBN(value string) string {
	var normalized strings.Builder
	for _, r := range value {
		switch {
		case r == '-' || unicode.IsSpace(r):
			continue
		case r >= '0' && r <= '9':
			normalized.WriteRune(r)
		case r == 'x' || r == 'X':
			normalized.WriteRune('X')
		default:
			return ""
		}
	}

	result := normalized.String()
	if len(result) != 10 && len(result) != 13 {
		return ""
	}
	if strings.Contains(result[:len(result)-1], "X") || (len(result) == 13 && strings.HasSuffix(result, "X")) {
		return ""
	}
	return result
}
