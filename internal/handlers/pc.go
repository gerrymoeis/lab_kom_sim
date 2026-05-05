package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"

	"github.com/gin-gonic/gin"
)

// PCList renders list of all PCs
func (h *Handler) PCList(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	rows, err := h.db.Query(`
		SELECT id, pc_number, row, column, status, processor, ram, storage, 
		       purchase_date, notes, last_checked, 
		       asset_id, serial_number, brand, model, operating_system, physical_condition,
		       created_at, updated_at
		FROM pcs
		ORDER BY pc_number
	`)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data PC",
		})
		return
	}
	defer rows.Close()

	var pcs []models.PC
	for rows.Next() {
		var pc models.PC
		err := rows.Scan(&pc.ID, &pc.PCNumber, &pc.Row, &pc.Column, &pc.Status,
			&pc.Processor, &pc.RAM, &pc.Storage, &pc.PurchaseDate, &pc.Notes,
			&pc.LastChecked, 
			&pc.AssetID, &pc.SerialNumber, &pc.Brand, &pc.Model, &pc.OperatingSystem, &pc.PhysicalCondition,
			&pc.CreatedAt, &pc.UpdatedAt)
		if err != nil {
			continue
		}
		pcs = append(pcs, pc)
	}

	c.HTML(http.StatusOK, "pc/list.html", gin.H{
		"title":    "Daftar PC - Sistem Inventaris Lab",
		"username": username,
		"role":     role,
		"pcs":      pcs,
	})
}

// PCDetail shows detail of a PC
func (h *Handler) PCDetail(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	id := c.Param("id")
	var pc models.PC
	err := h.db.QueryRow(`
		SELECT id, pc_number, row, column, status, processor, ram, storage,
		       purchase_date, notes, last_checked,
		       asset_id, serial_number, brand, model, operating_system, physical_condition,
		       created_at, updated_at
		FROM pcs WHERE id = ?
	`, id).Scan(&pc.ID, &pc.PCNumber, &pc.Row, &pc.Column, &pc.Status,
		&pc.Processor, &pc.RAM, &pc.Storage, &pc.PurchaseDate, &pc.Notes,
		&pc.LastChecked,
		&pc.AssetID, &pc.SerialNumber, &pc.Brand, &pc.Model, &pc.OperatingSystem, &pc.PhysicalCondition,
		&pc.CreatedAt, &pc.UpdatedAt)

	if err == sql.ErrNoRows {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "PC Tidak Ditemukan",
			"message": "PC yang Anda cari tidak ditemukan",
		})
		return
	}

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data PC",
		})
		return
	}

	// Get software installed on this PC
	softwareRows, err := h.db.Query(`
		SELECT id, name, version, license, install_date, notes
		FROM software WHERE pc_id = ?
		ORDER BY name
	`, id)

	var software []models.Software
	if err == nil {
		defer softwareRows.Close()
		for softwareRows.Next() {
			var sw models.Software
			if err := softwareRows.Scan(&sw.ID, &sw.Name, &sw.Version, &sw.License, 
				&sw.InstallDate, &sw.Notes); err == nil {
				software = append(software, sw)
			}
		}
	}

	c.HTML(http.StatusOK, "pc/detail.html", gin.H{
		"title":    "Detail PC - Sistem Inventaris Lab",
		"username": username,
		"role":     role,
		"pc":       pc,
		"software": software,
	})
}

// PCCreatePage renders PC creation form
func (h *Handler) PCCreatePage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	c.HTML(http.StatusOK, "pc/create.html", gin.H{
		"title":    "Tambah PC Baru - Sistem Inventaris Lab",
		"username": username,
		"role":     role,
	})
}

// PCCreate handles PC creation
func (h *Handler) PCCreate(c *gin.Context) {
	pcNumber, _ := strconv.Atoi(c.PostForm("pc_number"))
	row, _ := strconv.Atoi(c.PostForm("row"))
	column, _ := strconv.Atoi(c.PostForm("column"))
	status := c.PostForm("status")
	processor := c.PostForm("processor")
	ram := c.PostForm("ram")
	storage := c.PostForm("storage")
	purchaseDate := c.PostForm("purchase_date")
	notes := c.PostForm("notes")

	// Validate
	if pcNumber < 1 || pcNumber > 40 {
		c.HTML(http.StatusBadRequest, "pc/create.html", gin.H{
			"title": "Tambah PC Baru",
			"error": "Nomor PC harus antara 1-40",
		})
		return
	}

	if row < 1 || row > 5 || column < 1 || column > 8 {
		c.HTML(http.StatusBadRequest, "pc/create.html", gin.H{
			"title": "Tambah PC Baru",
			"error": "Posisi baris (1-5) dan kolom (1-8) tidak valid",
		})
		return
	}

	var purchaseDatePtr *string
	if purchaseDate != "" {
		purchaseDatePtr = &purchaseDate
	}

	_, err := h.db.Exec(`
		INSERT INTO pcs (pc_number, row, column, status, processor, ram, storage, purchase_date, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, pcNumber, row, column, status, processor, ram, storage, purchaseDatePtr, notes)

	if err != nil {
		c.HTML(http.StatusInternalServerError, "pc/create.html", gin.H{
			"title": "Tambah PC Baru",
			"error": "Gagal menyimpan data PC. Mungkin nomor PC sudah digunakan.",
		})
		return
	}

	c.Redirect(http.StatusFound, "/pc")
}

// PCEditPage renders PC edit form
func (h *Handler) PCEditPage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	id := c.Param("id")
	var pc models.PC
	var purchaseDateStr sql.NullString

	err := h.db.QueryRow(`
		SELECT id, pc_number, row, column, status, processor, ram, storage,
		       purchase_date, notes
		FROM pcs WHERE id = ?
	`, id).Scan(&pc.ID, &pc.PCNumber, &pc.Row, &pc.Column, &pc.Status,
		&pc.Processor, &pc.RAM, &pc.Storage, &purchaseDateStr, &pc.Notes)

	if err == sql.ErrNoRows {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "PC Tidak Ditemukan",
			"message": "PC yang Anda cari tidak ditemukan",
		})
		return
	}

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data PC",
		})
		return
	}

	var purchaseDateFormatted string
	if purchaseDateStr.Valid {
		if t, err := time.Parse("2006-01-02", purchaseDateStr.String); err == nil {
			purchaseDateFormatted = t.Format("2006-01-02")
		}
	}

	c.HTML(http.StatusOK, "pc/edit.html", gin.H{
		"title":        "Edit PC - Sistem Inventaris Lab",
		"username":     username,
		"role":         role,
		"pc":           pc,
		"purchaseDate": purchaseDateFormatted,
	})
}

// PCEdit handles PC update
func (h *Handler) PCEdit(c *gin.Context) {
	id := c.Param("id")
	status := c.PostForm("status")
	processor := c.PostForm("processor")
	ram := c.PostForm("ram")
	storage := c.PostForm("storage")
	purchaseDate := c.PostForm("purchase_date")
	notes := c.PostForm("notes")

	var purchaseDatePtr *string
	if purchaseDate != "" {
		purchaseDatePtr = &purchaseDate
	}

	_, err := h.db.Exec(`
		UPDATE pcs 
		SET status = ?, processor = ?, ram = ?, storage = ?, 
		    purchase_date = ?, notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, processor, ram, storage, purchaseDatePtr, notes, id)

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengupdate data PC",
		})
		return
	}

	c.Redirect(http.StatusFound, "/pc/"+id)
}

// PCDelete handles PC deletion
func (h *Handler) PCDelete(c *gin.Context) {
	id := c.Param("id")

	_, err := h.db.Exec("DELETE FROM pcs WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menghapus PC",
		})
		return
	}

	c.Redirect(http.StatusFound, "/pc")
}

// PCAssetInfoPage renders asset information form
func (h *Handler) PCAssetInfoPage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	id := c.Param("id")
	var pc models.PC
	var purchaseDateStr sql.NullString

	err := h.db.QueryRow(`
		SELECT id, pc_number, row, column, status, processor, ram, storage,
		       purchase_date, notes,
		       asset_id, serial_number, brand, model, operating_system, physical_condition
		FROM pcs WHERE id = ?
	`, id).Scan(&pc.ID, &pc.PCNumber, &pc.Row, &pc.Column, &pc.Status,
		&pc.Processor, &pc.RAM, &pc.Storage, &purchaseDateStr, &pc.Notes,
		&pc.AssetID, &pc.SerialNumber, &pc.Brand, &pc.Model, &pc.OperatingSystem, &pc.PhysicalCondition)

	if err == sql.ErrNoRows {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "PC Tidak Ditemukan",
			"message": "PC yang Anda cari tidak ditemukan",
		})
		return
	}

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data PC",
		})
		return
	}

	var purchaseDateFormatted string
	if purchaseDateStr.Valid {
		if t, err := time.Parse("2006-01-02", purchaseDateStr.String); err == nil {
			purchaseDateFormatted = t.Format("2006-01-02")
		}
	}

	c.HTML(http.StatusOK, "pc/asset-info.html", gin.H{
		"title":        "Input Aset Lengkap - Sistem Inventaris Lab",
		"username":     username,
		"role":         role,
		"pc":           pc,
		"purchaseDate": purchaseDateFormatted,
	})
}

// PCAssetInfoUpdate handles asset information update
func (h *Handler) PCAssetInfoUpdate(c *gin.Context) {
	id := c.Param("id")
	
	// Get form data
	assetID := c.PostForm("asset_id")
	serialNumber := c.PostForm("serial_number")
	brand := c.PostForm("brand")
	model := c.PostForm("model")
	processor := c.PostForm("processor")
	ram := c.PostForm("ram")
	storage := c.PostForm("storage")
	operatingSystem := c.PostForm("operating_system")
	physicalCondition := c.PostForm("physical_condition")
	status := c.PostForm("status")
	purchaseDate := c.PostForm("purchase_date")
	notes := c.PostForm("notes")

	// Auto-generate asset_id if empty
	if assetID == "" {
		var pcNumber int
		err := h.db.QueryRow("SELECT pc_number FROM pcs WHERE id = ?", id).Scan(&pcNumber)
		if err == nil {
			assetID = fmt.Sprintf("LAB-PC-%03d", pcNumber)
		}
	}

	// Set default physical_condition if empty
	if physicalCondition == "" {
		physicalCondition = "baik"
	}

	var purchaseDatePtr *string
	if purchaseDate != "" {
		purchaseDatePtr = &purchaseDate
	}

	// Update database
	_, err := h.db.Exec(`
		UPDATE pcs 
		SET asset_id = ?, serial_number = ?, brand = ?, model = ?,
		    processor = ?, ram = ?, storage = ?,
		    operating_system = ?, physical_condition = ?, status = ?,
		    purchase_date = ?, notes = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, assetID, serialNumber, brand, model,
		processor, ram, storage,
		operatingSystem, physicalCondition, status,
		purchaseDatePtr, notes, id)

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengupdate data aset: " + err.Error(),
		})
		return
	}

	c.Redirect(http.StatusFound, "/pc/"+id)
}

// PCStatusAPI returns PC status for API calls
func (h *Handler) PCStatusAPI(c *gin.Context) {
	rows, err := h.db.Query(`
		SELECT id, pc_number, status
		FROM pcs
		ORDER BY pc_number
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengambil data",
		})
		return
	}
	defer rows.Close()

	var pcs []map[string]interface{}
	for rows.Next() {
		var id, pcNumber int
		var status string
		if err := rows.Scan(&id, &pcNumber, &status); err == nil {
			pcs = append(pcs, map[string]interface{}{
				"id":        id,
				"pc_number": pcNumber,
				"status":    status,
			})
		}
	}

	c.JSON(http.StatusOK, pcs)
}

// UpdatePCStatusAPI updates PC status via API
func (h *Handler) UpdatePCStatusAPI(c *gin.Context) {
	id := c.Param("id")
	status := c.PostForm("status")

	if status == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Status tidak boleh kosong",
		})
		return
	}

	_, err := h.db.Exec(`
		UPDATE pcs 
		SET status = ?, last_checked = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengupdate status",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Status berhasil diupdate",
	})
}
