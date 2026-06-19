package repository

import (
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
	var colsPerRowJSON string
	var hasGapInt, gapPos int
	err := r.db.QueryRow(
		`SELECT cols_per_row, has_gap, gap_pos FROM grid_layouts WHERE lab_url_path = ?`,
		labURLPath,
	).Scan(&colsPerRowJSON, &hasGapInt, &gapPos)
	if err != nil {
		return nil, err
	}

	var colsPerRow []int
	if err := json.Unmarshal([]byte(colsPerRowJSON), &colsPerRow); err != nil {
		return nil, err
	}

	return &config.GridLayout{
		ColsPerRow: colsPerRow,
		HasGap:     hasGapInt == 1,
		GapPos:     gapPos,
	}, nil
}

func (r *LayoutRepository) Upsert(labURLPath string, colsPerRow []int, hasGap bool, gapPos int) error {
	colsJSON, err := json.Marshal(colsPerRow)
	if err != nil {
		return err
	}
	hg := 0
	if hasGap {
		hg = 1
	}
	_, err = r.db.Exec(`INSERT INTO grid_layouts (lab_url_path, cols_per_row, has_gap, gap_pos)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(lab_url_path) DO UPDATE SET
		cols_per_row = excluded.cols_per_row,
		has_gap = excluded.has_gap,
		gap_pos = excluded.gap_pos,
		updated_at = CURRENT_TIMESTAMP`,
		labURLPath, string(colsJSON), hg, gapPos)
	return err
}

func (r *LayoutRepository) List() ([]config.GridLayout, error) {
	rows, err := r.db.Query(`SELECT lab_url_path, cols_per_row, has_gap, gap_pos FROM grid_layouts ORDER BY lab_url_path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var layouts []config.GridLayout
	for rows.Next() {
		var labURLPath, colsJSON string
		var hasGapInt, gapPos int
		if err := rows.Scan(&labURLPath, &colsJSON, &hasGapInt, &gapPos); err != nil {
			return nil, err
		}
		var cols []int
		if err := json.Unmarshal([]byte(colsJSON), &cols); err != nil {
			return nil, err
		}
		layouts = append(layouts, config.GridLayout{
			ColsPerRow: cols,
			HasGap:     hasGapInt == 1,
			GapPos:     gapPos,
		})
	}
	return layouts, nil
}
