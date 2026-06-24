package config

import (
	"database/sql"
	"encoding/json"
)

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
	EnvIndex    int    // N from LABS_<N>_* (0 = from old format, cannot permanently delete)
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

var globalDB interface {
	QueryRow(string, ...any) *sql.Row
}

func SetGlobalDB(db interface{ QueryRow(string, ...any) *sql.Row }) {
	globalDB = db
}

func GetGridLayout(labURLPath string) GridLayout {
	if globalDB != nil {
		var colsJSON string
		var hasGapInt, gapPosInt int
		err := globalDB.QueryRow(
			`SELECT cols_per_row, has_gap, gap_pos FROM grid_layouts WHERE lab_url_path = ?`,
			labURLPath).Scan(&colsJSON, &hasGapInt, &gapPosInt)
		if err == nil {
			var cols []int
			if json.Unmarshal([]byte(colsJSON), &cols) == nil && len(cols) > 0 {
				return GridLayout{
					ColsPerRow: cols,
					HasGap:     hasGapInt == 1,
					GapPos:     gapPosInt,
				}
			}
		}
	}

	if l, ok := DefaultGridLayouts[labURLPath]; ok {
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
