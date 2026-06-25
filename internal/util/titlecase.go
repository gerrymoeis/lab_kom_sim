package util

import (
	"regexp"
	"strings"
)

func ToTitleCase(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	re := regexp.MustCompile(`\s+`)
	s = re.ReplaceAllString(s, " ")
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) == 0 {
			continue
		}
		if len(w) > 1 && strings.ToUpper(w) == w {
			continue
		}
		words[i] = strings.ToUpper(string(w[0])) + w[1:]
	}
	return strings.Join(words, " ")
}

func ToTitleCaseWithAbbr(s string) string {
	r := ToTitleCase(s)
	r = regexp.MustCompile(`\b([A-Z])([A-Z])\b`).ReplaceAllString(r, "$1.$2")
	return strings.TrimSuffix(r, ".")
}

func SanitizeText(s string) string {
	s = strings.TrimSpace(s)
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	return s
}

func ToUpperTrim(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}
