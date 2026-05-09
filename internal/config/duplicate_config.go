package config

// DuplicateCheckConfig holds configuration for duplicate detection
type DuplicateCheckConfig struct {
	// MaxDiffShortWord is max character difference for short words (≤5 chars)
	MaxDiffShortWord int
	
	// MaxDiffLongWord is max character difference for long words (>5 chars)
	MaxDiffLongWord int
	
	// MaxDiffNIM is max digit difference for NIM comparison
	MaxDiffNIM int
	
	// MaxLengthDiff is max length difference between compared strings
	MaxLengthDiff int
}

// DefaultDuplicateConfig provides default thresholds for duplicate detection
var DefaultDuplicateConfig = DuplicateCheckConfig{
	MaxDiffShortWord: 1,  // Short words: max 1 char difference
	MaxDiffLongWord:  2,  // Long words: max 2 char difference
	MaxDiffNIM:       1,  // NIM: max 1 digit difference
	MaxLengthDiff:    2,  // Max 2 chars length difference
}
