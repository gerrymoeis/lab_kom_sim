package repository

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
)

type PCRepository struct {
	db *database.DB
}

func NewPCRepository(db *database.DB) *PCRepository {
	return &PCRepository{db: db}
}

type PCFilters struct {
	Status    string
	Search    string
	SortBy    string
	SortOrder string
}

func (r *PCRepository) List(filters PCFilters) ([]models.PC, error) {
	query := `SELECT id, pc_number, "row", "column", status, processor, ram, storage, operating_system,
		serial_number, brand_model, device_type, accessories, notes, action_notes, last_checked FROM pcs WHERE 1=1`
	var args []interface{}

	if filters.Status != "" {
		query += ` AND status = ?`
		args = append(args, filters.Status)
	}
	if filters.Search != "" {
		query += ` AND (CAST(pc_number AS TEXT) LIKE ? OR serial_number LIKE ? OR brand_model LIKE ?)`
		s := "%" + filters.Search + "%"
		args = append(args, s, s, s)
	}

	sortBy := filters.SortBy
	validSort := map[string]bool{"pc_number": true, "status": true, "brand_model": true, "operating_system": true}
	if !validSort[sortBy] {
		sortBy = "pc_number"
	}
	sortOrder := filters.SortOrder
	if sortOrder != "DESC" {
		sortOrder = "ASC"
	}
	query += fmt.Sprintf(` ORDER BY %s %s`, sortBy, sortOrder)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pcs []models.PC
	for rows.Next() {
		var pc models.PC
		var processor, ram, storage, os, sn, bm, dt, acc, notes, an sql.NullString
		var lastChecked sql.NullTime
		if err := rows.Scan(&pc.ID, &pc.PCNumber, &pc.Row, &pc.Column, &pc.Status, &processor, &ram, &storage, &os,
			&sn, &bm, &dt, &acc, &notes, &an, &lastChecked); err != nil {
			return nil, err
		}
		pc.Processor = valStr(processor)
		pc.RAM = valStr(ram)
		pc.Storage = valStr(storage)
		pc.OperatingSystem = valStr(os)
		pc.SerialNumber = valStr(sn)
		pc.BrandModel = valStr(bm)
		pc.DeviceType = valStr(dt)
		pc.Accessories = valStr(acc)
		pc.Notes = valStr(notes)
		pc.ActionNotes = valStr(an)
		if lastChecked.Valid {
			pc.LastChecked = &lastChecked.Time
		}
		pcs = append(pcs, pc)
	}
	return pcs, nil
}

func (r *PCRepository) GetStatusCounts() (map[string]int, error) {
	statusCounts := map[string]int{}
	rows, err := r.db.Query(`SELECT status, COUNT(*) FROM pcs GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var s string
		var c int
		if err := rows.Scan(&s, &c); err != nil {
			return nil, err
		}
		statusCounts[s] = c
	}
	return statusCounts, nil
}

func (r *PCRepository) GetByPCNumber(num int) (*models.PC, error) {
	var pc models.PC
	var processor, ram, storage, os, notes, sn, bm, dt, acc, an, ps, pf, aid, brand, model sql.NullString
	var pDate, lc sql.NullTime
	err := r.db.QueryRow(`SELECT id, pc_number, "row", "column", status, processor, ram, storage,
		purchase_date, notes, last_checked, asset_id, serial_number, brand, model, operating_system,
		physical_condition, device_type, brand_model, accessories, action_notes, photo_serial, photo_front,
		created_at, updated_at FROM pcs WHERE pc_number = ?`, num).
		Scan(&pc.ID, &pc.PCNumber, &pc.Row, &pc.Column, &pc.Status, &processor, &ram, &storage,
			&pDate, &notes, &lc, &aid, &sn, &brand, &model, &os,
			&pc.PhysicalCondition, &dt, &bm, &acc, &an, &ps, &pf, &pc.CreatedAt, &pc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	pc.Processor = valStr(processor)
	pc.RAM = valStr(ram)
	pc.Storage = valStr(storage)
	pc.OperatingSystem = valStr(os)
	pc.SerialNumber = valStr(sn)
	pc.BrandModel = valStr(bm)
	pc.DeviceType = valStr(dt)
	pc.Accessories = valStr(acc)
	pc.ActionNotes = valStr(an)
	pc.Notes = valStr(notes)
	pc.PhotoSerial = valStr(ps)
	pc.PhotoFront = valStr(pf)
	pc.AssetID = valStr(aid)
	pc.Brand = valStr(brand)
	pc.Model = valStr(model)
	if pDate.Valid {
		pc.PurchaseDate = &pDate.Time
	}
	if lc.Valid {
		pc.LastChecked = &lc.Time
	}
	return &pc, nil
}

func (r *PCRepository) GetByPCNumberEdit(num int) (*models.PC, error) {
	var pc models.PC
	var processor, ram, storage, os, notes, sn, bm, dt, acc, an, ps, pf sql.NullString
	var pDate, lc sql.NullString
	err := r.db.QueryRow(`SELECT id, pc_number, "row", "column", status, processor, ram, storage,
		purchase_date, last_checked, operating_system, notes, device_type, serial_number, brand_model,
		accessories, action_notes, photo_serial, photo_front FROM pcs WHERE pc_number = ?`, num).
		Scan(&pc.ID, &pc.PCNumber, &pc.Row, &pc.Column, &pc.Status, &processor, &ram, &storage,
			&pDate, &lc, &os, &notes, &dt, &sn, &bm, &acc, &an, &ps, &pf)
	if err != nil {
		return nil, err
	}
	pc.Processor = valStr(processor)
	pc.RAM = valStr(ram)
	pc.Storage = valStr(storage)
	pc.OperatingSystem = valStr(os)
	pc.SerialNumber = valStr(sn)
	pc.BrandModel = valStr(bm)
	pc.DeviceType = valStr(dt)
	pc.Accessories = valStr(acc)
	pc.Notes = valStr(notes)
	pc.ActionNotes = valStr(an)
	pc.PhotoSerial = valStr(ps)
	pc.PhotoFront = valStr(pf)
	return &pc, nil
}

func (r *PCRepository) GetIDByPCNumber(num int) (int, int, error) {
	var id, oldNum int
	err := r.db.QueryRow(`SELECT id, pc_number FROM pcs WHERE pc_number = ?`, num).Scan(&id, &oldNum)
	return id, oldNum, err
}

func (r *PCRepository) GetDeleteInfo(num int) (*models.PC, error) {
	var pc models.PC
	var status, sn, dt, bm sql.NullString
	err := r.db.QueryRow(`SELECT id, status, serial_number, device_type, brand_model FROM pcs WHERE pc_number = ?`, num).
		Scan(&pc.ID, &status, &sn, &dt, &bm)
	if err != nil {
		return nil, err
	}
	pc.Status = valStr(status)
	pc.SerialNumber = valStr(sn)
	pc.DeviceType = valStr(dt)
	pc.BrandModel = valStr(bm)
	return &pc, nil
}

func (r *PCRepository) GetSoftware(pcID int) (requiredSW, otherSW []models.PCSoftware, err error) {
	rows, err := r.db.Query(`SELECT sc.id, sc.name, sc.category, COALESCE(ps.installed, FALSE), sc.description
		FROM software_catalog sc LEFT JOIN pc_software ps ON sc.id = ps.software_id AND ps.pc_id = ?
		ORDER BY sc.category, sc.name`, pcID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name, cat, desc string
		var installed bool
		if err := rows.Scan(&id, &name, &cat, &installed, &desc); err != nil {
			return nil, nil, err
		}
		sw := models.PCSoftware{
			PCID: pcID, SoftwareID: id, Installed: installed,
			SoftwareName: name, Category: cat, Description: desc,
		}
		if cat == "required" {
			requiredSW = append(requiredSW, sw)
		} else if installed {
			otherSW = append(otherSW, sw)
		}
	}
	return requiredSW, otherSW, nil
}

func (r *PCRepository) Create(num, row, col int, status, processor, ram, storage, sn, os, dt, bm, acc, photoSerial, photoFront string) (sql.Result, error) {
	return r.db.Exec(`INSERT INTO pcs (pc_number, "row", "column", status, processor, ram, storage,
		serial_number, operating_system, device_type, brand_model, accessories, physical_condition,
		photo_serial, photo_front)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'baik', ?, ?)`,
		num, row, col, status, processor, ram, storage, sn, os, dt, bm, acc, photoSerial, photoFront)
}

func (r *PCRepository) Update(num int, status, dt, sn, bm, acc, processor, ram, storage, os, notes, an, photoSerial, photoFront string) error {
	_, err := r.db.Exec(`UPDATE pcs SET status=?, device_type=?, serial_number=?, brand_model=?, accessories=?,
		processor=?, ram=?, storage=?, operating_system=?, notes=?, action_notes=?,
		photo_serial=COALESCE(NULLIF(?, ''), photo_serial),
		photo_front=COALESCE(NULLIF(?, ''), photo_front),
		updated_at=CURRENT_TIMESTAMP
		WHERE pc_number=?`,
		status, dt, sn, bm, acc, processor, ram, storage, os, notes, an, photoSerial, photoFront, num)
	return err
}

func (r *PCRepository) UpdateStatus(id int, status string) error {
	_, err := r.db.Exec(`UPDATE pcs SET status=?, last_checked=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, id)
	return err
}

func (r *PCRepository) Delete(pcID int) error {
	_, err := r.db.Exec("DELETE FROM pcs WHERE id = ?", pcID)
	return err
}

func (r *PCRepository) DeleteByPCNumber(num int) error {
	_, err := r.db.Exec("DELETE FROM pcs WHERE pc_number = ?", num)
	return err
}

func (r *PCRepository) GetAllStatus() ([]models.PC, error) {
	rows, err := r.db.Query(`SELECT id, pc_number, status FROM pcs ORDER BY pc_number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pcs []models.PC
	for rows.Next() {
		var p models.PC
		if rows.Scan(&p.ID, &p.PCNumber, &p.Status) == nil {
			pcs = append(pcs, p)
		}
	}
	return pcs, nil
}

func (r *PCRepository) SeedRequiredSoftware(pcID int) error {
	swRows, err := r.db.Query(`SELECT id FROM software_catalog WHERE category = 'required'`)
	if err != nil {
		return err
	}
	defer swRows.Close()

	for swRows.Next() {
		var swID int
		swRows.Scan(&swID)
		r.db.Exec(`INSERT INTO pc_software (pc_id, software_id, installed) VALUES (?, ?, TRUE)`, pcID, swID)
	}
	return nil
}

func (r *PCRepository) SyncSoftware(pcID int, requiredIDs []string, otherNames, otherDescs []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.Exec(`DELETE FROM pc_software WHERE pc_id = ?`, pcID)

	checked := map[int]bool{}
	for _, idStr := range requiredIDs {
		if id, e := strconv.Atoi(idStr); e == nil {
			checked[id] = true
		}
	}

	for swID := range checked {
		tx.Exec(`INSERT INTO pc_software (pc_id, software_id, installed) VALUES (?, ?, TRUE)`, pcID, swID)
	}

	for i, name := range otherNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		desc := ""
		if i < len(otherDescs) {
			desc = strings.TrimSpace(otherDescs[i])
			if desc == "-" {
				desc = ""
			}
		}

		swID := 0
		if err2 := tx.QueryRow(`SELECT id FROM software_catalog WHERE name = ?`, name).Scan(&swID); err2 != nil {
			pgErr := tx.QueryRow(`INSERT INTO software_catalog (name, category, description) VALUES (?, 'other', ?) RETURNING id`, name, desc).Scan(&swID)
			if pgErr != nil {
				tx.Exec(`INSERT INTO software_catalog (name, category, description) VALUES (?, 'other', ?)`, name, desc)
				tx.QueryRow(`SELECT id FROM software_catalog WHERE name = ?`, name).Scan(&swID)
			}
		} else if desc != "" {
			tx.Exec(`UPDATE software_catalog SET description = ? WHERE id = ?`, desc, swID)
		}
		if swID > 0 {
			tx.Exec(`INSERT INTO pc_software (pc_id, software_id, installed) VALUES (?, ?, TRUE)`, pcID, swID)
		}
	}

	return tx.Commit()
}

func (r *PCRepository) ExportAll() ([]models.PC, error) {
	rows, err := r.db.Query(`SELECT pc_number, "row", "column", status, device_type, serial_number, brand_model,
		processor, ram, storage, operating_system, accessories, purchase_date, last_checked, notes, action_notes
		FROM pcs ORDER BY pc_number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pcs []models.PC
	for rows.Next() {
		var pc models.PC
		var dt, sn, bm, proc, mem, stor, os, acc, pd, lc, n, an sql.NullString
		if err := rows.Scan(&pc.PCNumber, &pc.Row, &pc.Column, &pc.Status, &dt, &sn, &bm,
			&proc, &mem, &stor, &os, &acc, &pd, &lc, &n, &an); err != nil {
			return nil, err
		}
		pc.DeviceType = valStr(dt)
		pc.SerialNumber = valStr(sn)
		pc.BrandModel = valStr(bm)
		pc.Processor = valStr(proc)
		pc.RAM = valStr(mem)
		pc.Storage = valStr(stor)
		pc.OperatingSystem = valStr(os)
		pc.Accessories = valStr(acc)
		pc.Notes = valStr(n)
		pc.ActionNotes = valStr(an)
		pcs = append(pcs, pc)
	}
	return pcs, nil
}
