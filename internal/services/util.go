package services

import (
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

func ParseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

func MustParseDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func CopyFile(src, dst string) error {
	s, err := os.Open(src); if err != nil { return err }; defer s.Close()
	d, err := os.Create(dst); if err != nil { return err }; defer d.Close()
	_, err = io.Copy(d, s); return err
}

func ToTitleCaseWithAbbr(text string) string {
	text = strings.TrimSpace(text)
	if text == "" { return "" }
	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")
	words := strings.Fields(text)
	for i, w := range words {
		if len(w) > 0 { words[i] = strings.ToUpper(string(w[0])) + strings.ToLower(w[1:]) }
	}
	r := strings.Join(words, " ")
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
