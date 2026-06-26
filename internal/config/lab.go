package config

import (
	"database/sql"
	"encoding/json"
)

type GridLayout struct {
	ColsPerRow []int
	HasGap     bool       // Deprecated, kept for backward compat
	GapPos     int        // Deprecated, kept for backward compat
	RowGaps    [][]int    // Per-row gap positions (1-indexed), e.g. [[3],[],[5,7]]
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
		RowGaps:    [][]int{{}, {}, {}, {}, {}},
	},
	"labkom-vokasi-1": {
		ColsPerRow: []int{10, 8, 9, 9},
		HasGap:     true,
		GapPos:     4,
		RowGaps:    [][]int{{}, {}, {4}, {4}},
	},
	// New format (URLPath as key)
	"lab-kom-mi": {
		ColsPerRow: []int{8, 8, 8, 8, 8},
		HasGap:     false,
		RowGaps:    [][]int{{}, {}, {}, {}, {}},
	},
	"vokasi": {
		ColsPerRow: []int{10, 8, 9, 9},
		HasGap:     true,
		GapPos:     4,
		RowGaps:    [][]int{{}, {}, {4}, {4}},
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
		var colsJSON, rowGapsJSON sql.NullString
		var hasGapInt, gapPosInt int
		err := globalDB.QueryRow(
			`SELECT cols_per_row, has_gap, gap_pos, row_gaps FROM grid_layouts WHERE lab_url_path = ?`,
			labURLPath).Scan(&colsJSON, &hasGapInt, &gapPosInt, &rowGapsJSON)
		if err == nil && colsJSON.Valid {
			var cols []int
			if json.Unmarshal([]byte(colsJSON.String), &cols) == nil && len(cols) > 0 {
				gl := GridLayout{
					ColsPerRow: cols,
					HasGap:     hasGapInt == 1,
					GapPos:     gapPosInt,
				}
				if rowGapsJSON.Valid && rowGapsJSON.String != "" {
					if json.Unmarshal([]byte(rowGapsJSON.String), &gl.RowGaps) != nil {
						gl.RowGaps = nil
					}
				}
				if gl.RowGaps == nil {
					gl.RowGaps = RowGapsFromOld(cols, gl.HasGap, gl.GapPos)
				}
				return gl
			}
		}
	}

	if l, ok := DefaultGridLayouts[labURLPath]; ok {
		if l.RowGaps == nil {
			l.RowGaps = RowGapsFromOld(l.ColsPerRow, l.HasGap, l.GapPos)
		}
		return l
	}
	cols := []int{8, 8, 8, 8, 8}
	return GridLayout{ColsPerRow: cols, RowGaps: RowGapsFromOld(cols, false, 0)}
}

func (g GridLayout) ColsAtRow(rowIndex int) int {
	if rowIndex < 0 || rowIndex >= len(g.ColsPerRow) {
		return 8
	}
	return g.ColsPerRow[rowIndex]
}

func RowGapsFromOld(colsPerRow []int, hasGap bool, gapPos int) [][]int {
	gaps := make([][]int, len(colsPerRow))
	if hasGap && gapPos > 0 {
		for i := range gaps {
			if gapPos <= colsPerRow[i] {
				gaps[i] = []int{gapPos}
			}
		}
	}
	return gaps
}

func (g GridLayout) PositionFromRowCol(row, col int) (int, bool) {
	if row < 1 || row > len(g.ColsPerRow) {
		return 0, false
	}
	if col < 1 || col > g.ColsPerRow[row-1] {
		return 0, false
	}
	pos := 0
	for i := 0; i < row-1; i++ {
		pos += g.ColsPerRow[i]
	}
	pos += col
	return pos, true
}
