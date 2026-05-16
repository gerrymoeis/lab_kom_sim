package services

import (
	"strings"
	"time"
	"unicode/utf8"

	"inventaris-lab-kom/internal/config"
)

func countCharDifferences(s1, s2 string) int {
	s1 = strings.ToLower(strings.TrimSpace(s1))
	s2 = strings.ToLower(strings.TrimSpace(s2))
	if s1 == s2 { return 0 }

	len1 := utf8.RuneCountInString(s1)
	len2 := utf8.RuneCountInString(s2)
	matrix := make([][]int, len1+1)
	for i := range matrix { matrix[i] = make([]int, len2+1) }
	for i := 0; i <= len1; i++ { matrix[i][0] = i }
	for j := 0; j <= len2; j++ { matrix[0][j] = j }

	runes1 := []rune(s1)
	runes2 := []rune(s2)
	for i := 1; i <= len1; i++ {
		for j := 1; j <= len2; j++ {
			cost := 0
			if runes1[i-1] != runes2[j-1] { cost = 1 }
			matrix[i][j] = min3(matrix[i-1][j]+1, matrix[i][j-1]+1, matrix[i-1][j-1]+cost)
		}
	}
	return matrix[len1][len2]
}

func min3(a, b, c int) int {
	if a < b { if a < c { return a }; return c }
	if b < c { return b }; return c
}

func areNamesSimilar(name1, name2 string, cfg config.DuplicateCheckConfig) bool {
	name1 = strings.ToLower(strings.TrimSpace(name1))
	name2 = strings.ToLower(strings.TrimSpace(name2))
	if name1 == name2 { return true }

	lenDiff := len(name1) - len(name2)
	if lenDiff < 0 { lenDiff = -lenDiff }
	if lenDiff > cfg.MaxLengthDiff { return false }

	words1 := strings.Fields(name1)
	words2 := strings.Fields(name2)
	if len(words1) != len(words2) { return false }

	for i := 0; i < len(words1); i++ {
		if words1[i] == words2[i] { continue }
		diff := countCharDifferences(words1[i], words2[i])
		wordLen := len(words1[i])
		if len(words2[i]) > wordLen { wordLen = len(words2[i]) }
		threshold := cfg.MaxDiffShortWord
		if wordLen > 5 { threshold = cfg.MaxDiffLongWord }
		if diff > threshold { return false }
	}
	return true
}

func areNIMsSimilar(nim1, nim2 string, cfg config.DuplicateCheckConfig) bool {
	nim1 = strings.ToUpper(strings.TrimSpace(nim1))
	nim1 = strings.ReplaceAll(nim1, " ", "")
	nim2 = strings.ToUpper(strings.TrimSpace(nim2))
	nim2 = strings.ReplaceAll(nim2, " ", "")
	if nim1 == nim2 { return true }
	if len(nim1) != len(nim2) { return false }
	return countCharDifferences(nim1, nim2) <= cfg.MaxDiffNIM
}

func IsDuplicateEntry(date1, date2 time.Time, time1, time2, name1, name2, nim1, nim2 string, cfg config.DuplicateCheckConfig) bool {
	if !date1.Equal(date2) { return false }
	if time1 != time2 { return false }
	return areNamesSimilar(name1, name2, cfg) || areNIMsSimilar(nim1, nim2, cfg)
}
