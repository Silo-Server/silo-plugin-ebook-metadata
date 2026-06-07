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
	if len(result) == 10 && !validISBN10(result) {
		return ""
	}
	if len(result) == 13 && !validISBN13(result) {
		return ""
	}
	return result
}

func validISBN10(value string) bool {
	sum := 0
	for i, r := range value {
		var digit int
		if r == 'X' {
			digit = 10
		} else {
			digit = int(r - '0')
		}
		sum += digit * (10 - i)
	}
	return sum%11 == 0
}

func validISBN13(value string) bool {
	sum := 0
	for i, r := range value {
		digit := int(r - '0')
		if i%2 == 1 {
			sum += digit * 3
			continue
		}
		sum += digit
	}
	return sum%10 == 0
}
