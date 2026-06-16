package config

type GridLayout struct {
	ColsPerRow []int
	HasGap     bool
	GapPos     int
}

type LabConfig struct {
	Name        string
	Title       string
	DBPath      string
	UploadDir   string
	Layout      GridLayout
	PublicBuild *PublicBuildConfig
}
