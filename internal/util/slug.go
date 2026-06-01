package util

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

var reNonSlug = regexp.MustCompile(`[^a-z0-9-]+`)
var reMultiDash = regexp.MustCompile(`-+`)

func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = reNonSlug.ReplaceAllString(s, "-")
	s = reMultiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		h := sha256.Sum256([]byte(s))
		s = fmt.Sprintf("item-%x", h[:8])
	}
	return s
}
