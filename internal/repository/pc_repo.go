package repository

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/search"
	"inventaris-lab-kom/internal/util"
)

type PCRepository struct {
	db     *database.DB
	search *search.Builder
}

func NewPCRepository(db *database.DB) *PCRepository {
	return &PCRepository{db: db, search: search.New(db)}
}

type PCFilters struct {
	Status    string
	Placement string
	Search    string
	SortBy    string
	SortOrder string
	OS        string
}

func (r *PCRepository) List(filters PCFilters) ([]models.PC, error) {
	return r.listWithQuery(filters, "", 0, 0)
}

func (r *PCRepository) ListPaginated(filters PCFilters, page, pageSize int) ([]models.PC, int, error) {
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 20 }

	var total int
	r.db.QueryRow(r.buildCountQuery(filters), r.buildCountArgs(filters)...).Scan(&total)

	pcs, err := r.listWithQuery(filters, ` LIMIT ? OFFSET ?`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	return pcs, total, nil
}

func (r *PCRepository) buildWhereClause(filters PCFilters) (string, []any) {
	var clause string
	var args []any
	if filters.Status != "" {
		clause += ` AND status = ?`
		args = append(args, filters.Status)
	}
	if filters.Placement != "" {
		clause += ` AND placement = ?`
		args = append(args, filters.Placement)
	}
	if filters.Search != "" {
		sClause, sArgs := r.search.Where("pc", filters.Search)
		clause += sClause
		args = append(args, sArgs...)
	}
	if filters.OS != "" {
		clause += ` AND operating_system = ?`
		args = append(args, filters.OS)
	}
	return clause, args
}

func (r *PCRepository) buildCountQuery(filters PCFilters) string {
	clause, _ := r.buildWhereClause(filters)
	return `SELECT COUNT(*) FROM pcs WHERE 1=1` + clause
}

func (r *PCRepository) buildCountArgs(filters PCFilters) []any {
	_, args := r.buildWhereClause(filters)
	return args
}

func (r *PCRepository) listWithQuery(filters PCFilters, suffix string, limit, offset int) ([]models.PC, error) {
	query := `SELECT id, label, "row", "column", status, placement, processor, ram, storage, operating_system,
		serial_number, brand_model, pc_type, accessories, notes, last_checked FROM pcs WHERE 1=1`
	clause, args := r.buildWhereClause(filters)
	query += clause

	sortBy := filters.SortBy
	validSort := map[string]bool{"label": true, "status": true, "placement": true, "brand_model": true, "operating_system": true}
	if !validSort[sortBy] {
		sortBy = "label"
	}
	sortOrder := filters.SortOrder
	if sortOrder != "DESC" {
		sortOrder = "ASC"
	}
	if sortBy == "label" {
		query += ` ORDER BY
			CASE WHEN label GLOB 'pc-[0-9]*' THEN 1 WHEN label GLOB 'pc-cadangan-[0-9]*' THEN 3 ELSE 2 END,
			CASE WHEN label GLOB 'pc-[0-9]*' THEN CAST(SUBSTR(label, 4) AS INTEGER)
				WHEN label GLOB 'pc-cadangan-[0-9]*' THEN CAST(SUBSTR(label, 13) AS INTEGER) ELSE 0 END,
			label ` + sortOrder
	} else {
		query += fmt.Sprintf(` ORDER BY %s %s`, sortBy, sortOrder)
	}
	query += suffix
	if suffix != "" {
		args = append(args, limit, offset)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pcs []models.PC
	for rows.Next() {
		var pc models.PC
		var processor, ram, storage, os, sn, bm, pt, acc, notes, label sql.NullString
		var lastChecked sql.NullTime
		if err := rows.Scan(&pc.ID, &label, &pc.Row, &pc.Column, &pc.Status, &pc.Placement, &processor, &ram, &storage, &os,
			&sn, &bm, &pt, &acc, &notes, &lastChecked); err != nil {
			return nil, err
		}
		pc.Processor = valStr(processor)
		pc.RAM = valStr(ram)
		pc.Storage = valStr(storage)
		pc.OperatingSystem = valStr(os)
		pc.SerialNumber = valStr(sn)
		pc.BrandModel = valStr(bm)
		pc.PCType = valStr(pt)
		pc.Accessories = valStr(acc)
		pc.Notes = valStr(notes)
		pc.Label = valStr(label)
		if lastChecked.Valid {
			pc.LastChecked = &lastChecked.Time
		}
		pcs = append(pcs, pc)
	}
	return pcs, nil
}

func (r *PCRepository) NextLabel(placement string, isMahasiswa bool) string {
	switch {
	case placement == "cadangan":
		var max int
		r.db.QueryRow(`SELECT COALESCE(MAX(CAST(SUBSTR(label, 13) AS INTEGER)), 0) + 1 FROM pcs WHERE label GLOB 'pc-cadangan-[0-9]*'`).Scan(&max)
		return fmt.Sprintf("pc-cadangan-%d", max)
	case isMahasiswa:
		var max int
		r.db.QueryRow(`SELECT COALESCE(MAX(CAST(SUBSTR(label, 4) AS INTEGER)), 0) + 1 FROM pcs WHERE label GLOB 'pc-[0-9]*'`).Scan(&max)
		return fmt.Sprintf("pc-%d", max)
	default:
		return ""
	}
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

func (r *PCRepository) GetByLabel(label string) (*models.PC, error) {
	var pc models.PC
	var processor, ram, storage, os, notes, sn, bm, pt, acc, ps, pf, aid, lbl sql.NullString
	var pDate, lc sql.NullTime
	err := r.db.QueryRow(`SELECT id, label, "row", "column", status, placement, processor, ram, storage,
		purchase_date, notes, last_checked, asset_id, serial_number, operating_system,
		pc_type, brand_model, accessories, photo_serial, photo_front,
		created_at, updated_at FROM pcs WHERE label = ?`, label).
		Scan(&pc.ID, &lbl, &pc.Row, &pc.Column, &pc.Status, &pc.Placement, &processor, &ram, &storage,
			&pDate, &notes, &lc, &aid, &sn, &os,
			&pt, &bm, &acc, &ps, &pf, &pc.CreatedAt, &pc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	pc.Processor = valStr(processor)
	pc.RAM = valStr(ram)
	pc.Storage = valStr(storage)
	pc.OperatingSystem = valStr(os)
	pc.SerialNumber = valStr(sn)
	pc.BrandModel = valStr(bm)
	pc.PCType = valStr(pt)
	pc.Accessories = valStr(acc)
	pc.Notes = valStr(notes)
	pc.PhotoSerial = valStr(ps)
	pc.PhotoFront = valStr(pf)
	pc.AssetID = valStr(aid)
	pc.Label = valStr(lbl)
	if pDate.Valid {
		pc.PurchaseDate = &pDate.Time
	}
	if lc.Valid {
		pc.LastChecked = &lc.Time
	}
	return &pc, nil
}

func (r *PCRepository) GetByLabelEdit(label string) (*models.PC, error) {
	var pc models.PC
	var processor, ram, storage, os, notes, sn, bm, pt, acc, ps, pf, lbl sql.NullString
	var pDate, lc sql.NullTime
	err := r.db.QueryRow(`SELECT id, label, "row", "column", status, placement, processor, ram, storage,
		purchase_date, last_checked, operating_system, notes, pc_type, serial_number, brand_model,
		accessories, photo_serial, photo_front FROM pcs WHERE label = ?`, label).
		Scan(&pc.ID, &lbl, &pc.Row, &pc.Column, &pc.Status, &pc.Placement, &processor, &ram, &storage,
			&pDate, &lc, &os, &notes, &pt, &sn, &bm, &acc, &ps, &pf)
	if err != nil {
		return nil, err
	}
	pc.Processor = valStr(processor)
	pc.RAM = valStr(ram)
	pc.Storage = valStr(storage)
	pc.OperatingSystem = valStr(os)
	pc.SerialNumber = valStr(sn)
	pc.BrandModel = valStr(bm)
	pc.PCType = valStr(pt)
	pc.Accessories = valStr(acc)
	pc.Notes = valStr(notes)
	pc.PhotoSerial = valStr(ps)
	pc.PhotoFront = valStr(pf)
	pc.Label = valStr(lbl)
	if pDate.Valid {
		pc.PurchaseDate = &pDate.Time
	}
	if lc.Valid {
		pc.LastChecked = &lc.Time
	}
	return &pc, nil
}

func (r *PCRepository) GetIDByLabel(label string) (int, string, error) {
	var id int
	var lbl string
	err := r.db.QueryRow(`SELECT id, label FROM pcs WHERE label = ?`, label).Scan(&id, &lbl)
	return id, lbl, err
}

func (r *PCRepository) GetDeleteInfo(label string) (*models.PC, error) {
	var pc models.PC
	var status, sn, pt, bm sql.NullString
	err := r.db.QueryRow(`SELECT id, status, serial_number, pc_type, brand_model FROM pcs WHERE label = ?`, label).
		Scan(&pc.ID, &status, &sn, &pt, &bm)
	if err != nil {
		return nil, err
	}
	pc.Status = valStr(status)
	pc.SerialNumber = valStr(sn)
	pc.PCType = valStr(pt)
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

func (r *PCRepository) Create(row, col int, status, placement, processor, ram, storage, sn, os, pt, bm, acc, photoSerial, photoFront, label, purchaseDate, lastChecked, notes string) (sql.Result, error) {
	return r.db.Exec(`INSERT INTO pcs ("row", "column", status, placement, processor, ram, storage,
		serial_number, operating_system, pc_type, brand_model, accessories,
		photo_serial, photo_front, label, purchase_date, last_checked, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?)`,
		row, col, status, placement, processor, ram, storage, sn, os, pt, bm, acc, photoSerial, photoFront, label, purchaseDate, lastChecked, notes)
}

func (r *PCRepository) Update(label string, row, col int, status, placement, pt, sn, bm, acc, processor, ram, storage, os, notes, photoSerial, photoFront, newLabel, purchaseDate, lastChecked string) error {
	_, err := r.db.Exec(`UPDATE pcs SET "row"=?, "column"=?, status=?, placement=?, pc_type=?, serial_number=?, brand_model=?, accessories=?,
		processor=?, ram=?, storage=?, operating_system=?, notes=?, label=?,
		purchase_date=COALESCE(NULLIF(?, ''), purchase_date),
		last_checked=COALESCE(NULLIF(?, ''), last_checked),
		photo_serial=COALESCE(NULLIF(?, ''), photo_serial),
		photo_front=COALESCE(NULLIF(?, ''), photo_front),
		updated_at=CURRENT_TIMESTAMP
		WHERE label=?`,
		row, col, status, placement, pt, sn, bm, acc, processor, ram, storage, os, notes, newLabel, purchaseDate, lastChecked, photoSerial, photoFront, label)
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

func (r *PCRepository) DeleteByLabel(label string) error {
	_, err := r.db.Exec("DELETE FROM pcs WHERE label = ?", label)
	return err
}

func (r *PCRepository) GetAllStatus() ([]models.PC, error) {
	rows, err := r.db.Query(`SELECT id, label, status FROM pcs ORDER BY
		CASE WHEN label GLOB 'pc-[0-9]*' THEN 1 WHEN label GLOB 'pc-cadangan-[0-9]*' THEN 3 ELSE 2 END,
		CASE WHEN label GLOB 'pc-[0-9]*' THEN CAST(SUBSTR(label, 4) AS INTEGER)
			WHEN label GLOB 'pc-cadangan-[0-9]*' THEN CAST(SUBSTR(label, 13) AS INTEGER) ELSE 0 END,
		label`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pcs []models.PC
	for rows.Next() {
		var p models.PC
		var label sql.NullString
		if rows.Scan(&p.ID, &label, &p.Status) == nil {
			p.Label = valStr(label)
			pcs = append(pcs, p)
		}
	}
	return pcs, nil
}

func (r *PCRepository) GetStatus(id int) (string, error) {
	var status string
	err := r.db.QueryRow(`SELECT status FROM pcs WHERE id = ?`, id).Scan(&status)
	return status, err
}

func (r *PCRepository) SeedRequiredSoftware(pcID int) error {
	_, err := r.db.Exec(`INSERT INTO pc_software (pc_id, software_id, installed)
		SELECT ?, id, TRUE FROM software_catalog WHERE category = 'required'`, pcID)
	return err
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
slug := util.Slugify(name)
				pgErr := tx.QueryRow(`INSERT INTO software_catalog (name, category, description, slug) VALUES (?, 'other', ?, ?) RETURNING id`, name, desc, slug).Scan(&swID)
				if pgErr != nil {
					tx.Exec(`INSERT INTO software_catalog (name, category, description, slug) VALUES (?, 'other', ?, ?)`, name, desc, slug)
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

func (r *PCRepository) SwapLabels(labelA, labelB string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	pcA := struct{ id, row, col int }{}
	pcB := struct{ id, row, col int }{}
	if err := tx.QueryRow(`SELECT id, "row", "column" FROM pcs WHERE label = ?`, labelA).
		Scan(&pcA.id, &pcA.row, &pcA.col); err != nil {
		return err
	}
	if err := tx.QueryRow(`SELECT id, "row", "column" FROM pcs WHERE label = ?`, labelB).
		Scan(&pcB.id, &pcB.row, &pcB.col); err != nil {
		return err
	}

	// 3-step temp label swap to avoid UNIQUE violation
	tx.Exec(`UPDATE pcs SET label = '__SWAP_' || ? WHERE id = ?`, pcA.id, pcA.id)
	tx.Exec(`UPDATE pcs SET label = ? WHERE id = ?`, labelA, pcB.id)
	tx.Exec(`UPDATE pcs SET label = ? WHERE id = ?`, labelB, pcA.id)

	// Swap positions
	tx.Exec(`UPDATE pcs SET "row"=?, "column"=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, pcB.row, pcB.col, pcA.id)
	tx.Exec(`UPDATE pcs SET "row"=?, "column"=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, pcA.row, pcA.col, pcB.id)

	return tx.Commit()
}

func (r *PCRepository) ReplaceWithSpare(target, spare string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var tgtID, tgtRow, tgtCol int
	if err := tx.QueryRow(`SELECT id, "row", "column" FROM pcs WHERE label = ?`, target).
		Scan(&tgtID, &tgtRow, &tgtCol); err != nil {
		return err
	}
	var sprID int
	if err := tx.QueryRow(`SELECT id FROM pcs WHERE label = ?`, spare).
		Scan(&sprID); err != nil {
		return err
	}

	var next int
	tx.QueryRow(`SELECT COALESCE(MAX(CAST(SUBSTR(label, 13) AS INTEGER)), 0) + 1 FROM pcs WHERE label GLOB 'pc-cadangan-[0-9]*'`).Scan(&next)
	newLabel := fmt.Sprintf("pc-cadangan-%d", next)

	// 3-step temp label swap
	tx.Exec(`UPDATE pcs SET label = '__SWAP_' || ? WHERE id = ?`, tgtID, tgtID)
	tx.Exec(`UPDATE pcs SET label = ? WHERE id = ?`, target, sprID)
	tx.Exec(`UPDATE pcs SET label = ? WHERE id = ?`, newLabel, tgtID)

	// Spare takes target's position as dipakai
	tx.Exec(`UPDATE pcs SET "row"=?, "column"=?, placement='dipakai', updated_at=CURRENT_TIMESTAMP WHERE id=?`, tgtRow, tgtCol, sprID)
	// Target becomes cadangan
	tx.Exec(`UPDATE pcs SET "row"=0, "column"=0, placement='cadangan', updated_at=CURRENT_TIMESTAMP WHERE id=?`, tgtID)

	return tx.Commit()
}

func (r *PCRepository) MoveRowToCadangan(row int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`SELECT id FROM pcs WHERE "row" = ? AND placement = 'dipakai'`, row)
	if err != nil {
		return err
	}
	var ids []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()

	if len(ids) == 0 {
		return nil
	}

	var next int
	tx.QueryRow(`SELECT COALESCE(MAX(CAST(SUBSTR(label, 13) AS INTEGER)), 0) + 1 FROM pcs WHERE label GLOB 'pc-cadangan-[0-9]*'`).Scan(&next)

	for _, id := range ids {
		label := fmt.Sprintf("pc-cadangan-%d", next)
		next++
		tx.Exec(`UPDATE pcs SET label=?, "row"=0, "column"=0, placement='cadangan', updated_at=CURRENT_TIMESTAMP WHERE id=?`, label, id)
	}

	return tx.Commit()
}

func (r *PCRepository) MoveToPosition(label string, row, col int) error {
	newLabel := fmt.Sprintf("pc-%d", (row-1)*8+col)

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var id int
	if err := tx.QueryRow(`SELECT id FROM pcs WHERE label=?`, label).Scan(&id); err != nil {
		return err
	}

	var clash int
	if err := tx.QueryRow(`SELECT id FROM pcs WHERE label=? AND id!=?`, newLabel, id).Scan(&clash); err == nil {
		return fmt.Errorf("label %s sudah digunakan oleh PC lain", newLabel)
	}

	if _, err := tx.Exec(`UPDATE pcs SET label=?, "row"=?, "column"=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, newLabel, row, col, id); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *PCRepository) PlaceCadangan(label string, row, col int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var id int
	if err := tx.QueryRow(`SELECT id FROM pcs WHERE label=?`, label).Scan(&id); err != nil {
		return err
	}

	newLabel := fmt.Sprintf("pc-%d", (row-1)*8+col)
	var clash int
	tx.QueryRow(`SELECT id FROM pcs WHERE label=? AND id!=?`, newLabel, id).Scan(&clash)

	tx.Exec(`UPDATE pcs SET label=?, "row"=?, "column"=?, placement='dipakai', updated_at=CURRENT_TIMESTAMP WHERE id=?`, newLabel, row, col, id)
	tx.Exec(`DELETE FROM pc_software WHERE pc_id=?`, id)
	if err := tx.Commit(); err != nil {
		return err
	}

	_ = r.SeedRequiredSoftware(id)
	return nil
}

func (r *PCRepository) GetDistinctOS() ([]string, error) {
	rows, err := r.db.Query(`SELECT DISTINCT operating_system FROM pcs WHERE operating_system != '' ORDER BY operating_system`)
	if err != nil { return nil, err }
	defer rows.Close()
	var oss []string
	for rows.Next() {
		var os string
		if err := rows.Scan(&os); err != nil { return nil, err }
		oss = append(oss, os)
	}
	return oss, rows.Err()
}

func (r *PCRepository) ExportAll() ([]models.PC, error) {
	rows, err := r.db.Query(`SELECT label, "row", "column", status, placement, pc_type, serial_number, brand_model,
		processor, ram, storage, operating_system, accessories, purchase_date, last_checked, notes
		FROM pcs ORDER BY
		CASE WHEN label GLOB 'pc-[0-9]*' THEN 1 WHEN label GLOB 'pc-cadangan-[0-9]*' THEN 3 ELSE 2 END,
		CASE WHEN label GLOB 'pc-[0-9]*' THEN CAST(SUBSTR(label, 4) AS INTEGER)
			WHEN label GLOB 'pc-cadangan-[0-9]*' THEN CAST(SUBSTR(label, 13) AS INTEGER) ELSE 0 END,
		label`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pcs []models.PC
	for rows.Next() {
		var pc models.PC
		var label, placement, pt, sn, bm, proc, mem, stor, os, acc, pd, lc, n sql.NullString
		if err := rows.Scan(&label, &pc.Row, &pc.Column, &pc.Status, &placement, &pt, &sn, &bm,
			&proc, &mem, &stor, &os, &acc, &pd, &lc, &n); err != nil {
			return nil, err
		}
		pc.Label = valStr(label)
		pc.Placement = valStr(placement)
		pc.PCType = valStr(pt)
		pc.SerialNumber = valStr(sn)
		pc.BrandModel = valStr(bm)
		pc.Processor = valStr(proc)
		pc.RAM = valStr(mem)
		pc.Storage = valStr(stor)
		pc.OperatingSystem = valStr(os)
		pc.Accessories = valStr(acc)
		pc.Notes = valStr(n)
		pcs = append(pcs, pc)
	}
	return pcs, nil
}
