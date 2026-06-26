package repository

import (
	"database/sql"
	"encoding/json"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
)

type LayoutRepository struct {
	db DBTX
}

func NewLayoutRepository(db *database.DB) *LayoutRepository {
	return &LayoutRepository{db: db}
}

func (r *LayoutRepository) GetByLab(labURLPath string) (*config.GridLayout, error) {
	var colsPerRowJSON, rowGapsJSON sql.NullString
	var hasGapInt, gapPos int
	err := r.db.QueryRow(
		`SELECT cols_per_row, has_gap, gap_pos, row_gaps FROM grid_layouts WHERE lab_url_path = ?`,
		labURLPath,
	).Scan(&colsPerRowJSON, &hasGapInt, &gapPos, &rowGapsJSON)
	if err != nil {
		return nil, err
	}

	if !colsPerRowJSON.Valid {
		return nil, nil
	}

	var colsPerRow []int
	if err := json.Unmarshal([]byte(colsPerRowJSON.String), &colsPerRow); err != nil {
		return nil, err
	}

	gl := &config.GridLayout{
		ColsPerRow: colsPerRow,
		HasGap:     hasGapInt == 1,
		GapPos:     gapPos,
	}

	if rowGapsJSON.Valid && rowGapsJSON.String != "" {
		if json.Unmarshal([]byte(rowGapsJSON.String), &gl.RowGaps) != nil {
			gl.RowGaps = nil
		}
	}
	if gl.RowGaps == nil {
		gl.RowGaps = config.RowGapsFromOld(colsPerRow, gl.HasGap, gl.GapPos)
	}

	return gl, nil
}

func (r *LayoutRepository) Upsert(labURLPath string, colsPerRow []int, hasGap bool, gapPos int, rowGaps [][]int) error {
	colsJSON, err := json.Marshal(colsPerRow)
	if err != nil {
		return err
	}
	hg := 0
	if hasGap {
		hg = 1
	}
	rowGapsJSON, err := json.Marshal(rowGaps)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(`INSERT INTO grid_layouts (lab_url_path, cols_per_row, has_gap, gap_pos, row_gaps)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(lab_url_path) DO UPDATE SET
		cols_per_row = excluded.cols_per_row,
		has_gap = excluded.has_gap,
		gap_pos = excluded.gap_pos,
		row_gaps = excluded.row_gaps,
		updated_at = CURRENT_TIMESTAMP`,
		labURLPath, string(colsJSON), hg, gapPos, string(rowGapsJSON))
	return err
}

func (r *LayoutRepository) List() ([]config.GridLayout, error) {
	rows, err := r.db.Query(`SELECT lab_url_path, cols_per_row, has_gap, gap_pos, row_gaps FROM grid_layouts ORDER BY lab_url_path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var layouts []config.GridLayout
	for rows.Next() {
		var labURLPath, colsJSON string
		var hasGapInt, gapPos int
		var rowGapsJSON sql.NullString
		if err := rows.Scan(&labURLPath, &colsJSON, &hasGapInt, &gapPos, &rowGapsJSON); err != nil {
			return nil, err
		}
		var cols []int
		if err := json.Unmarshal([]byte(colsJSON), &cols); err != nil {
			return nil, err
		}
		gl := config.GridLayout{
			ColsPerRow: cols,
			HasGap:     hasGapInt == 1,
			GapPos:     gapPos,
		}
		if rowGapsJSON.Valid && rowGapsJSON.String != "" {
			if json.Unmarshal([]byte(rowGapsJSON.String), &gl.RowGaps) != nil {
				gl.RowGaps = nil
			}
		}
		if gl.RowGaps == nil {
			gl.RowGaps = config.RowGapsFromOld(cols, gl.HasGap, gl.GapPos)
		}
		layouts = append(layouts, gl)
	}
	return layouts, nil
}
