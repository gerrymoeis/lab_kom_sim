package config

type GridLayout struct {
	ColsPerRow []int
	HasGap     bool
	GapPos     int
}

type LabConfig struct {
	ID          string // "MI-1", "VOKASI-1" — lookup folder seeds/<lowercase(ID)>/
	Title       string // "Lab Kom MI 1" — display name
	DBPath      string // "data/lab_mi_1.db"
	URLPath     string // "lab-kom-mi" — routing URL, cookies, map keys
	UploadDir   string // "uploads/lab-kom-mi"
	Layout      GridLayout
	PublicBuild *PublicBuildConfig
}

var DefaultGridLayouts = map[string]GridLayout{
	// Old format (lab.Name as key) — backward compat
	"labkom-mi": {
		ColsPerRow: []int{8, 8, 8, 8, 8},
		HasGap:     false,
	},
	"labkom-vokasi-1": {
		ColsPerRow: []int{10, 8, 9, 9},
		HasGap:     true,
		GapPos:     4,
	},
	// New format (URLPath as key)
	"lab-kom-mi": {
		ColsPerRow: []int{8, 8, 8, 8, 8},
		HasGap:     false,
	},
	"vokasi": {
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
