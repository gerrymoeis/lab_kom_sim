package config

type DuplicateCheckConfig struct {
	JaroWinklerThreshold float64
	MaxDiffNIM           int
}

var DefaultDuplicateConfig = DuplicateCheckConfig{
	JaroWinklerThreshold: 0.88,
	MaxDiffNIM:           2,
}
