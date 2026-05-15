package handlers

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

func (h *Handler) PCList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	status := c.Query("status")
	search := c.Query("search")
	sortBy := c.DefaultQuery("sort_by", "pc_number")
	sortOrder := c.DefaultQuery("sort_order", "ASC")

	query := `SELECT id, pc_number, "row", "column", status, processor, ram, storage, operating_system,
		serial_number, brand_model, device_type, accessories, notes, action_notes, last_checked FROM pcs WHERE 1=1`
	var args []interface{}

	if status != "" { query += ` AND status = ?`; args = append(args, status) }
	if search != "" {
		query += ` AND (pc_number::TEXT LIKE ? OR serial_number LIKE ? OR brand_model LIKE ?)`
		s := "%" + search + "%"; args = append(args, s, s, s)
	}

	validSort := map[string]bool{"pc_number": true, "status": true, "brand_model": true, "operating_system": true}
	if !validSort[sortBy] { sortBy = "pc_number" }
	if sortOrder != "DESC" { sortOrder = "ASC" }

	pcs := []models.PC{}
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM pcs`); err != nil { h.errHTML(c, "Gagal mengambil data PC"); return }
	// ... (list query would go here)
	_ = pcs

	rows, err := h.db.Query(query+fmt.Sprintf(` ORDER BY %s %s`, sortBy, sortOrder), args...)
	if err != nil { h.errHTML(c, "Gagal mengambil data PC"); return }
	defer rows.Close()

	for rows.Next() {
		var pc models.PC
		var processor, ram, storage, os, sn, bm, dt, acc, notes, an sql.NullString
		var lastChecked sql.NullTime
		if rows.Scan(&pc.ID, &pc.PCNumber, &pc.Row, &pc.Column, &pc.Status, &processor, &ram, &storage, &os,
			&sn, &bm, &dt, &acc, &notes, &an, &lastChecked) != nil { continue }
		if processor.Valid { pc.Processor = processor.String }; if ram.Valid { pc.RAM = ram.String }
		if storage.Valid { pc.Storage = storage.String }; if os.Valid { pc.OperatingSystem = os.String }
		if sn.Valid { pc.SerialNumber = sn.String }; if bm.Valid { pc.BrandModel = bm.String }
		if dt.Valid { pc.DeviceType = dt.String }; if acc.Valid { pc.Accessories = acc.String }
		if notes.Valid { pc.Notes = notes.String }; if an.Valid { pc.ActionNotes = an.String }
		if lastChecked.Valid { pc.LastChecked = &lastChecked.Time }
		pcs = append(pcs, pc)
	}

	statusCounts := map[string]int{}
	if cr, _ := h.db.Query(`SELECT status, COUNT(*) FROM pcs GROUP BY status`); cr != nil {
		defer cr.Close()
		for cr.Next() { var s string; var c int; cr.Scan(&s, &c); statusCounts[s] = c }
	}

	c.HTML(http.StatusOK, "pc/list.html", gin.H{
		"title": "Daftar PC", "currentPage": "pc",
		"username": username, "role": role, "pcs": pcs,
		"statusCounts": statusCounts,
		"filters": gin.H{"status": status, "search": search, "sort_by": sortBy, "sort_order": sortOrder},
	})
}

func (h *Handler) PCDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	num := c.Param("pc_number")
	var pc models.PC
	var processor, ram, storage, assetID, sn, brand, model, os, cond, notes sql.NullString
	var dt, bm, acc, an, ps, pf sql.NullString
	var pDate, lc sql.NullTime

	err := h.db.QueryRow(`SELECT id, pc_number, "row", "column", status, processor, ram, storage,
		purchase_date, notes, last_checked, asset_id, serial_number, brand, model, operating_system,
		physical_condition, device_type, brand_model, accessories, action_notes, photo_serial, photo_front,
		created_at, updated_at FROM pcs WHERE pc_number = ?`, num).
		Scan(&pc.ID, &pc.PCNumber, &pc.Row, &pc.Column, &pc.Status, &processor, &ram, &storage,
			&pDate, &notes, &lc, &assetID, &sn, &brand, &model, &os, &cond, &dt, &bm, &acc, &an, &ps, &pf,
			&pc.CreatedAt, &pc.UpdatedAt)
	if err != nil { h.errHTML(c, "PC tidak ditemukan"); return }

	if processor.Valid { pc.Processor = processor.String }; if ram.Valid { pc.RAM = ram.String }
	if storage.Valid { pc.Storage = storage.String }; if notes.Valid { pc.Notes = notes.String }
	if os.Valid { pc.OperatingSystem = os.String }; if sn.Valid { pc.SerialNumber = sn.String }
	if bm.Valid { pc.BrandModel = bm.String }; if dt.Valid { pc.DeviceType = dt.String }
	if acc.Valid { pc.Accessories = acc.String }; if pDate.Valid { pc.PurchaseDate = &pDate.Time }
	if lc.Valid { pc.LastChecked = &lc.Time }

	var requiredSW, otherSW []models.PCSoftware
	if sr, _ := h.db.Query(`SELECT sc.id, sc.name, sc.category, COALESCE(ps.installed, FALSE), sc.description
		FROM software_catalog sc LEFT JOIN pc_software ps ON sc.id = ps.software_id AND ps.pc_id = ?
		ORDER BY sc.category, sc.name`, pc.ID); sr != nil {
		defer sr.Close()
		for sr.Next() {
			var id int; var name, cat, desc string; var installed bool
			if sr.Scan(&id, &name, &cat, &installed, &desc) == nil {
				sw := models.PCSoftware{PCID: pc.ID, SoftwareID: id, Installed: installed, SoftwareName: name, Category: cat, Description: desc}
				if cat == "required" { requiredSW = append(requiredSW, sw) } else if installed { otherSW = append(otherSW, sw) }
			}
		}
	}

	lcFormatted := ""
	if pc.LastChecked != nil { lcFormatted = pc.LastChecked.Format("02/01/2006 15:04") }
	c.HTML(http.StatusOK, "pc/detail.html", gin.H{
		"title": "Detail PC", "currentPage": "pc",
		"username": username, "role": role, "pc": pc,
		"requiredSW": requiredSW, "otherSW": otherSW,
		"lastCheckedFormatted": lcFormatted,
	})
}

func (h *Handler) PCCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	c.HTML(http.StatusOK, "pc/create.html", gin.H{
		"title": "Tambah PC Baru", "currentPage": "pc",
		"username": username, "role": role,
	})
}

func (h *Handler) PCCreate(c *gin.Context) {
	numStr := c.PostForm("pc_number")
	rowStr := c.PostForm("row")
	colStr := c.PostForm("column")
	status := c.DefaultPostForm("status", "normal")
	sn := c.PostForm("serial_number")
	os := c.PostForm("operating_system")
	dt := c.DefaultPostForm("device_type", "PC All-in-one")
	bm := c.DefaultPostForm("brand_model", "Axioo Mypc One Pro K7-24 (16N9)")
	acc := c.DefaultPostForm("accessories", "Keyboard & Mouse Axioo (Wired Set)")
	processor := c.DefaultPostForm("processor", "Intel Core i7")
	ram := c.DefaultPostForm("ram", "16GB DDR4")
	storage := c.DefaultPostForm("storage", "1TB NVMe")

	num, _ := strconv.Atoi(numStr)
	row, _ := strconv.Atoi(rowStr)
	col, _ := strconv.Atoi(colStr)

	if num < 1 || num > 40 || row < 1 || row > 5 || col < 1 || col > 8 || sn == "" || os == "" {
		c.HTML(http.StatusBadRequest, "pc/create.html", gin.H{
			"title": "Tambah PC Baru", "error": "Lengkapi data yang diperlukan",
		})
		return
	}

	_, err := h.db.Exec(`INSERT INTO pcs (pc_number, "row", "column", status, processor, ram, storage,
		serial_number, operating_system, device_type, brand_model, accessories, physical_condition)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'baik')`,
		num, row, col, status, processor, ram, storage, sn, os, dt, bm, acc)
	if err != nil {
		h.logCreateError(c, "pc", map[string]interface{}{"pc_number": num, "serial_number": sn}, err.Error())
		c.HTML(http.StatusInternalServerError, "pc/create.html", gin.H{
			"title": "Tambah PC Baru", "error": "Gagal menyimpan. Mungkin nomor PC sudah digunakan.",
		})
		return
	}

	var pcID int
	h.db.QueryRow(`SELECT id FROM pcs WHERE pc_number = ?`, num).Scan(&pcID)
	if pcID > 0 {
		h.logCreate(c, "pc", pcID, map[string]interface{}{
			"pc_number": num, "serial_number": sn, "operating_system": os,
		})

		// Seed required software for this PC
		swRows, _ := h.db.Query(`SELECT id FROM software_catalog WHERE category = 'required'`)
		if swRows != nil {
			defer swRows.Close()
			for swRows.Next() {
				var swID int
				swRows.Scan(&swID)
				h.db.Exec(`INSERT INTO pc_software (pc_id, software_id, installed) VALUES (?, ?, TRUE)`, pcID, swID)
			}
		}
	}
	c.Redirect(http.StatusFound, "/pc")
}

// ─── Edit ─────────────────────────────────────────────────────────

func (h *Handler) PCEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	num := c.Param("pc_number")
	var pc models.PC
	var processor, ram, storage, os, notes sql.NullString
	var dt, sn, bm, acc, an, ps, pf sql.NullString
	var pDate, lc sql.NullString

	err := h.db.QueryRow(`SELECT id, pc_number, "row", "column", status, processor, ram, storage,
		purchase_date, last_checked, operating_system, notes, device_type, serial_number, brand_model,
		accessories, action_notes, photo_serial, photo_front FROM pcs WHERE pc_number = ?`, num).
		Scan(&pc.ID, &pc.PCNumber, &pc.Row, &pc.Column, &pc.Status, &processor, &ram, &storage,
			&pDate, &lc, &os, &notes, &dt, &sn, &bm, &acc, &an, &ps, &pf)
	if err != nil { h.errHTML(c, "PC tidak ditemukan"); return }

	if processor.Valid { pc.Processor = processor.String }; if ram.Valid { pc.RAM = ram.String }
	if storage.Valid { pc.Storage = storage.String }; if os.Valid { pc.OperatingSystem = os.String }
	if sn.Valid { pc.SerialNumber = sn.String }; if bm.Valid { pc.BrandModel = bm.String }
	if dt.Valid { pc.DeviceType = dt.String }; if acc.Valid { pc.Accessories = acc.String }
	if notes.Valid { pc.Notes = notes.String }; if an.Valid { pc.ActionNotes = an.String }

	var requiredSW, otherSW []models.PCSoftware
	if sr, _ := h.db.Query(`SELECT sc.id, sc.name, sc.category, COALESCE(ps.installed, FALSE), sc.description
		FROM software_catalog sc LEFT JOIN pc_software ps ON sc.id = ps.software_id AND ps.pc_id = ?
		ORDER BY sc.category, sc.name`, pc.ID); sr != nil {
		defer sr.Close()
		for sr.Next() {
			var id int; var name, cat, desc string; var installed bool
			if sr.Scan(&id, &name, &cat, &installed, &desc) == nil {
				sw := models.PCSoftware{PCID: pc.ID, SoftwareID: id, Installed: installed, SoftwareName: name, Category: cat, Description: desc}
				if cat == "required" { requiredSW = append(requiredSW, sw) } else if installed { otherSW = append(otherSW, sw) }
			}
		}
	}

	c.HTML(http.StatusOK, "pc/edit.html", gin.H{
		"title": "Edit PC", "currentPage": "pc",
		"username": username, "role": role, "pc": pc,
		"requiredSW": requiredSW, "otherSW": otherSW,
	})
}

func (h *Handler) PCEdit(c *gin.Context) {
	num := c.Param("pc_number")
	status := c.PostForm("status")
	sn := c.PostForm("serial_number")
	os := c.PostForm("operating_system")
	dt := c.PostForm("device_type")
	bm := c.PostForm("brand_model")
	acc := c.PostForm("accessories")
	processor := c.PostForm("processor")
	ram := c.PostForm("ram")
	storage := c.PostForm("storage")
	notes := c.PostForm("notes")
	an := c.PostForm("action_notes")

	if sn == "" || os == "" { h.errHTML(c, "Serial Number dan OS wajib diisi"); return }

	var pcID, oldNum int
	h.db.QueryRow(`SELECT id, pc_number FROM pcs WHERE pc_number = ?`, num).Scan(&pcID, &oldNum)

	_, err := h.db.Exec(`UPDATE pcs SET status=?, device_type=?, serial_number=?, brand_model=?, accessories=?,
		processor=?, ram=?, storage=?, operating_system=?, notes=?, action_notes=?, updated_at=CURRENT_TIMESTAMP
		WHERE pc_number=?`, status, dt, sn, bm, acc, processor, ram, storage, os, notes, an, num)
	if err != nil {
		h.logUpdateError(c, "pc", pcID, map[string]interface{}{"pc_number": num}, err.Error())
		h.errHTML(c, "Gagal mengupdate PC")
		return
	}

	h.logUpdate(c, "pc", pcID,
		map[string]interface{}{"pc_number": num, "serial_number": sn},
		map[string]interface{}{"status": status, "serial_number": sn},
	)

	requiredIDs := c.PostFormArray("required_sw[]")
	otherNames := c.PostFormArray("other_name[]")
	otherDescs := c.PostFormArray("other_desc[]")
	if err := syncPCSoftware(h.db, pcID, requiredIDs, otherNames, otherDescs); err != nil {
		h.logUpdateError(c, "pc", pcID, map[string]interface{}{"pc_id": pcID}, err.Error())
		h.errHTML(c, "Gagal menyimpan software PC")
		return
	}

	c.Redirect(http.StatusFound, "/pc")
}

// ─── Delete ───────────────────────────────────────────────────────

func (h *Handler) PCDelete(c *gin.Context) {
	num := c.Param("pc_number")
	var pcID int; var status, sn, dt, bm sql.NullString
	err := h.db.QueryRow(`SELECT id, status, serial_number, device_type, brand_model FROM pcs WHERE pc_number = ?`, num).
		Scan(&pcID, &status, &sn, &dt, &bm)
	if err != nil { h.errJSON(c, 500, "Gagal mengambil data PC"); return }

	if _, err := h.db.Exec("DELETE FROM pcs WHERE pc_number = ?", num); err != nil {
		h.logDeleteError(c, "pc", pcID, map[string]interface{}{"pc_number": num}, err.Error())
		h.errJSON(c, 500, "Gagal menghapus PC"); return
	}

	h.logDelete(c, "pc", pcID, map[string]interface{}{
		"pc_number": num, "serial_number": sn.String, "device_type": dt.String,
	})
	c.Redirect(http.StatusFound, "/pc")
}

// ─── API ──────────────────────────────────────────────────────────

func (h *Handler) PCStatusAPI(c *gin.Context) {
	rows, _ := h.db.Query(`SELECT id, pc_number, status FROM pcs ORDER BY pc_number`)
	if rows != nil {
		defer rows.Close()
		var pcs []map[string]interface{}
		for rows.Next() {
			var id, num int; var s string
			if rows.Scan(&id, &num, &s) == nil {
				pcs = append(pcs, map[string]interface{}{"id": id, "pc_number": num, "status": s})
			}
		}
		c.JSON(http.StatusOK, pcs); return
	}
	c.JSON(http.StatusOK, []interface{}{})
}

func (h *Handler) UpdatePCStatusAPI(c *gin.Context) {
	id := c.Param("id")
	status := c.PostForm("status")
	if status == "" { h.errJSON(c, 400, "Status harus diisi"); return }
	h.db.Exec(`UPDATE pcs SET status=?, last_checked=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, id)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Status berhasil diupdate"})
}

// ─── Export ───────────────────────────────────────────────────────

func (h *Handler) PCExport(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat export"); return }

	rows, _ := h.db.Query(`SELECT pc_number, "row", "column", status, device_type, serial_number, brand_model,
		processor, ram, storage, operating_system, accessories, purchase_date, last_checked, notes, action_notes
		FROM pcs ORDER BY pc_number`)

	type PCData struct{ Num, Row, Col int; Status string; Dt, Sn, Bm, Proc, Mem, Stor, Os, Acc, Pd, Lc, N, An sql.NullString }
	var list []PCData
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var p PCData
			if rows.Scan(&p.Num, &p.Row, &p.Col, &p.Status, &p.Dt, &p.Sn, &p.Bm, &p.Proc, &p.Mem, &p.Stor, &p.Os, &p.Acc, &p.Pd, &p.Lc, &p.N, &p.An) == nil {
				list = append(list, p)
			}
		}
	}

	data := [][]interface{}{}
	for i, p := range list {
		get := func(ns sql.NullString) string { if ns.Valid { return ns.String }; return "-" }
		pos := fmt.Sprintf("Baris %d - Kolom %d", p.Row, p.Col)
		data = append(data, []interface{}{i + 1, fmt.Sprintf("PC-%02d", p.Num), pos, p.Status, get(p.Dt), get(p.Sn), get(p.Bm),
			get(p.Proc), get(p.Mem), get(p.Stor), get(p.Os), get(p.Acc), get(p.Pd), get(p.Lc), get(p.N), get(p.An)})
	}

	svc := services.NewExcelService()
	f, _ := svc.GenerateExcel(services.ExcelExportConfig{
		SheetName: "Daftar PC",
		Headers: []string{"No", "PC", "Posisi", "Status", "Device Type", "Serial Number", "Brand & Model",
			"Processor", "RAM", "Storage", "OS", "Accessories", "Purchase Date", "Last Checked", "Notes", "Action Notes"},
		Data: data,
		ColumnWidths: map[string]float64{"A": 5, "B": 10, "C": 15, "D": 10, "E": 22, "F": 18, "G": 28,
			"H": 22, "I": 12, "J": 12, "K": 22, "L": 28, "M": 14, "N": 18, "O": 30, "P": 30},
	})
	defer f.Close()

	fn := svc.GenerateFilename("pc_export")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+fn)
	f.Write(c.Writer)
}

// ─── Software Sync ────────────────────────────────────────────────

func syncPCSoftware(db *database.DB, pcID int, requiredIDs []string, otherNames, otherDescs []string) error {
	var err error

	tx, err := db.Begin()
	if err != nil { return err }
	defer tx.Rollback()

	tx.Exec(`DELETE FROM pc_software WHERE pc_id = ?`, pcID)

	checked := map[int]bool{}
	for _, idStr := range requiredIDs {
		if id, e := strconv.Atoi(idStr); e == nil { checked[id] = true }
	}

	for swID := range checked {
		tx.Exec(`INSERT INTO pc_software (pc_id, software_id, installed) VALUES (?, ?, TRUE)`, pcID, swID)
	}

	for i, name := range otherNames {
		name = strings.TrimSpace(name); if name == "" { continue }
		desc := ""; if i < len(otherDescs) { desc = strings.TrimSpace(otherDescs[i]); if desc == "-" { desc = "" } }

		var swID int
		err = tx.QueryRow(`SELECT id FROM software_catalog WHERE name = ?`, name).Scan(&swID)
		if err != nil {
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

// ─── Helper ───────────────────────────────────────────────────────

func copyFile(src, dst string) error {
	s, err := os.Open(src); if err != nil { return err }; defer s.Close()
	d, err := os.Create(dst); if err != nil { return err }; defer d.Close()
	_, err = io.Copy(d, s); return err
}
