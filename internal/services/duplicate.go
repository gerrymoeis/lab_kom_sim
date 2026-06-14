package services

import (
	"strings"
	"time"
	"unicode/utf8"

	"inventaris-lab-kom/internal/config"
)

func jaroWinkler(s1, s2 string) float64 {
	s1 = strings.ToLower(strings.TrimSpace(s1))
	s2 = strings.ToLower(strings.TrimSpace(s2))
	if s1 == s2 {
		return 1.0
	}
	runes1 := []rune(s1)
	runes2 := []rune(s2)
	len1 := len(runes1)
	len2 := len(runes2)
	if len1 == 0 || len2 == 0 {
		return 0.0
	}
	matchDist := max(len1, len2)/2 - 1
	if matchDist < 0 {
		matchDist = 0
	}
	matched1 := make([]bool, len1)
	matched2 := make([]bool, len2)
	var matches float64
	var transPositions []int
	for i := 0; i < len1; i++ {
		start := i - matchDist
		if start < 0 {
			start = 0
		}
		end := i + matchDist + 1
		if end > len2 {
			end = len2
		}
		for j := start; j < end; j++ {
			if matched2[j] {
				continue
			}
			if runes1[i] != runes2[j] {
				continue
			}
			matched1[i] = true
			matched2[j] = true
			matches++
			transPositions = append(transPositions, j)
			break
		}
	}
	if matches == 0 {
		return 0.0
	}
	var transpositions float64
	k := 0
	for i := 0; i < len1; i++ {
		if !matched1[i] {
			continue
		}
		for k < len(transPositions) {
			if !matched2[transPositions[k]] {
				k++
				continue
			}
			if runes1[i] != runes2[transPositions[k]] {
				transpositions++
			}
			k++
			break
		}
	}
	jaro := (matches/float64(len1) + matches/float64(len2) + (matches-transpositions/2)/matches) / 3.0
	prefix := 0
	maxPrefix := 4
	for i := 0; i < min(len1, len2, maxPrefix); i++ {
		if runes1[i] == runes2[i] {
			prefix++
		} else {
			break
		}
	}
	return jaro + float64(prefix)*0.1*(1.0-jaro)
}

func areNamesSimilar(name1, name2 string, cfg config.DuplicateCheckConfig) bool {
	return jaroWinkler(name1, name2) >= cfg.JaroWinklerThreshold
}

func areNIMsSimilar(nim1, nim2 string, cfg config.DuplicateCheckConfig) bool {
	nim1 = strings.ToUpper(strings.TrimSpace(nim1))
	nim1 = strings.ReplaceAll(nim1, " ", "")
	nim2 = strings.ToUpper(strings.TrimSpace(nim2))
	nim2 = strings.ReplaceAll(nim2, " ", "")
	if nim1 == nim2 {
		return true
	}
	lenDiff := len(nim1) - len(nim2)
	if lenDiff < 0 {
		lenDiff = -lenDiff
	}
	if lenDiff > 1 {
		return false
	}
	return countCharDifferences(nim1, nim2) <= cfg.MaxDiffNIM
}

func countCharDifferences(s1, s2 string) int {
	s1 = strings.ToLower(strings.TrimSpace(s1))
	s2 = strings.ToLower(strings.TrimSpace(s2))
	if s1 == s2 {
		return 0
	}
	len1 := utf8.RuneCountInString(s1)
	len2 := utf8.RuneCountInString(s2)
	matrix := make([][]int, len1+1)
	for i := range matrix {
		matrix[i] = make([]int, len2+1)
	}
	for i := 0; i <= len1; i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len2; j++ {
		matrix[0][j] = j
	}
	runes1 := []rune(s1)
	runes2 := []rune(s2)
	for i := 1; i <= len1; i++ {
		for j := 1; j <= len2; j++ {
			cost := 0
			if runes1[i-1] != runes2[j-1] {
				cost = 1
			}
			matrix[i][j] = min3(matrix[i-1][j]+1, matrix[i][j-1]+1, matrix[i-1][j-1]+cost)
		}
	}
	return matrix[len1][len2]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func IsDuplicateEntry(date1, date2 time.Time, time1, time2, name1, name2, nim1, nim2 string, cfg config.DuplicateCheckConfig) bool {
	if !date1.Equal(date2) {
		return false
	}
	if time1 != time2 {
		return false
	}
	return areNamesSimilar(name1, name2, cfg) || areNIMsSimilar(nim1, nim2, cfg)
}
