package services

import (
	"io"
	"os"
	"time"

	"inventaris-lab-kom/internal/util"
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
	return util.ToTitleCaseWithAbbr(text)
}

func SanitizeText(s string) string {
	return util.SanitizeText(s)
}

func ToUpperTrim(s string) string {
	return util.ToUpperTrim(s)
}
