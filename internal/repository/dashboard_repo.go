package repository

import (
	"database/sql"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
)

type DashboardRepository struct {
	db DBTX
}

func NewDashboardRepository(db *database.DB) *DashboardRepository {
	return &DashboardRepository{db: db}
}

func (r *DashboardRepository) ListPCs() ([]models.PC, error) {
	rows, err := r.db.Query(`SELECT id, label, "row", "column", status, placement,
		processor, ram, storage, operating_system, notes, last_checked 
		FROM pcs ORDER BY "row", "column"`)
	if err != nil { return nil, err }
	defer rows.Close()

	var pcs []models.PC
	for rows.Next() {
		var pc models.PC
		var processor, ram, storage, os, notes, label sql.NullString
		var lastChecked sql.NullTime
		if err := rows.Scan(&pc.ID, &label, &pc.Row, &pc.Column, &pc.Status, &pc.Placement,
			&processor, &ram, &storage, &os, &notes, &lastChecked); err != nil {
			return nil, err
		}
		pc.Processor = valStr(processor)
		pc.RAM = valStr(ram)
		pc.Storage = valStr(storage)
		pc.OperatingSystem = valStr(os)
		pc.Notes = valStr(notes)
		pc.Label = valStr(label)
		if lastChecked.Valid { pc.LastChecked = &lastChecked.Time }
		pcs = append(pcs, pc)
	}
	return pcs, nil
}

func (r *DashboardRepository) CountAll() (deviceCount, softwareCount int, err error) {
	err = r.db.QueryRow(`SELECT (SELECT COUNT(*) FROM devices), (SELECT COUNT(*) FROM software_catalog)`).Scan(&deviceCount, &softwareCount)
	return
}
