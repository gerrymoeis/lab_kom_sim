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

var DefaultGridLayouts = map[string]GridLayout{
	"labkom-mi": {
		ColsPerRow: []int{8, 8, 8, 8, 8},
		HasGap:     false,
	},
	"labkom-vokasi-1": {
		ColsPerRow: []int{10, 8, 9, 9},
		HasGap:     true,
		GapPos:     4,
	},
}

func GetGridLayout(labName string) GridLayout {
	if l, ok := DefaultGridLayouts[labName]; ok {
		return l
	}
	return GridLayout{ColsPerRow: []int{8, 8, 8, 8, 8}}
}

func (g GridLayout) ColsAtRow(rowIndex int) int {
	if rowIndex < 0 || rowIndex >= len(g.ColsPerRow) {
		return 8
	}
	return g.ColsPerRow[rowIndex]
}
