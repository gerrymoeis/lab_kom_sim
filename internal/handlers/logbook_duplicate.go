package handlers

import (
	"inventaris-lab-kom/internal/config"
	"strings"
	"time"
	"unicode/utf8"
)

// countCharDifferences counts the number of character differences between two strings
// Returns the minimum number of single-character edits (insertions, deletions, substitutions)
func countCharDifferences(s1, s2 string) int {
	// Normalize: lowercase and trim
	s1 = strings.ToLower(strings.TrimSpace(s1))
	s2 = strings.ToLower(strings.TrimSpace(s2))
	
	if s1 == s2 {
		return 0
	}
	
	len1 := utf8.RuneCountInString(s1)
	len2 := utf8.RuneCountInString(s2)
	
	// Simple Levenshtein distance (edit distance)
	// Create matrix
	matrix := make([][]int, len1+1)
	for i := range matrix {
		matrix[i] = make([]int, len2+1)
	}
	
	// Initialize first row and column
	for i := 0; i <= len1; i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len2; j++ {
		matrix[0][j] = j
	}
	
	// Convert strings to rune slices for proper character handling
	runes1 := []rune(s1)
	runes2 := []rune(s2)
	
	// Fill matrix
	for i := 1; i <= len1; i++ {
		for j := 1; j <= len2; j++ {
			cost := 0
			if runes1[i-1] != runes2[j-1] {
				cost = 1
			}
			
			// Minimum of: deletion, insertion, substitution
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}
	
	return matrix[len1][len2]
}

// min returns the minimum of three integers
func min(a, b, c int) int {
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

// areNamesSimilar checks if two names are similar based on word-by-word comparison
// Short words (≤5 chars): max 1 char difference
// Long words (>5 chars): max 2 char difference
func areNamesSimilar(name1, name2 string, cfg config.DuplicateCheckConfig) bool {
	// Normalize: lowercase, trim, remove extra spaces
	name1 = strings.ToLower(strings.TrimSpace(name1))
	name2 = strings.ToLower(strings.TrimSpace(name2))
	
	// Exact match
	if name1 == name2 {
		return true
	}
	
	// Check length difference
	lenDiff := len(name1) - len(name2)
	if lenDiff < 0 {
		lenDiff = -lenDiff
	}
	if lenDiff > cfg.MaxLengthDiff {
		return false
	}
	
	// Split into words
	words1 := strings.Fields(name1)
	words2 := strings.Fields(name2)
	
	// Must have same number of words
	if len(words1) != len(words2) {
		return false
	}
	
	// Compare each word pair
	for i := 0; i < len(words1); i++ {
		word1 := words1[i]
		word2 := words2[i]
		
		// Exact match - continue
		if word1 == word2 {
			continue
		}
		
		// Calculate character differences
		diff := countCharDifferences(word1, word2)
		
		// Determine threshold based on word length
		wordLen := len(word1)
		if len(word2) > wordLen {
			wordLen = len(word2)
		}
		
		var threshold int
		if wordLen <= 5 {
			threshold = cfg.MaxDiffShortWord
		} else {
			threshold = cfg.MaxDiffLongWord
		}
		
		// If difference exceeds threshold, names are not similar
		if diff > threshold {
			return false
		}
	}
	
	// All words are similar
	return true
}

// areNIMsSimilar checks if two NIMs are similar (max 1 digit difference)
func areNIMsSimilar(nim1, nim2 string, cfg config.DuplicateCheckConfig) bool {
	// Normalize: uppercase, trim, remove spaces
	nim1 = strings.ToUpper(strings.TrimSpace(nim1))
	nim1 = strings.ReplaceAll(nim1, " ", "")
	
	nim2 = strings.ToUpper(strings.TrimSpace(nim2))
	nim2 = strings.ReplaceAll(nim2, " ", "")
	
	// Exact match
	if nim1 == nim2 {
		return true
	}
	
	// Must have same length
	if len(nim1) != len(nim2) {
		return false
	}
	
	// Count differences
	diff := countCharDifferences(nim1, nim2)
	
	// Check against threshold
	return diff <= cfg.MaxDiffNIM
}

// isDuplicateEntry checks if two logbook entries are duplicates based on similarity
// Duplicate criteria:
// - Same date
// - Same time_in
// - Similar name (word-by-word with threshold) OR similar NIM (max 1 digit diff)
func isDuplicateEntry(date1, date2 time.Time, time1, time2, name1, name2, nim1, nim2 string, cfg config.DuplicateCheckConfig) bool {
	// Must have same date
	if !date1.Equal(date2) {
		return false
	}
	
	// Must have same time_in
	if time1 != time2 {
		return false
	}
	
	// Check name similarity
	nameSimilar := areNamesSimilar(name1, name2, cfg)
	
	// Check NIM similarity
	nimSimilar := areNIMsSimilar(nim1, nim2, cfg)
	
	// Duplicate if EITHER name is similar OR NIM is similar
	// (same person at same date/time)
	return nameSimilar || nimSimilar
}
