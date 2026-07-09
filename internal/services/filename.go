package services

import (
	"fmt"
	"inventaris-lab-kom/internal/timeutil"
)

func GenerateZIPFilename(prefix string) string {
	timestamp := timeutil.Now().Format("20060102")
	return fmt.Sprintf("%s_%s.zip", prefix, timestamp)
}
