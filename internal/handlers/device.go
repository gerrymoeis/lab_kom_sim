package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"inventaris-lab-kom/internal/models"

	"github.com/gin-gonic/gin"
)

// ─── Helpers ──────────────────────────────────────────────────────

func (h *Handler) fetchDeviceTypes() []models.DeviceType {
	rows, err := h.db.Query(`SELECT id, name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location FROM device_types ORDER BY category, name`)
	if err != nil { return nil }
	defer rows.Close()

	var dts []models.DeviceType
	for rows.Next() {
		var dt models.DeviceType
		var b, m, p, l sql.NullString
		if rows.Scan(&dt.ID, &dt.Name, &dt.Category, &b, &m, &dt.ItemType, &dt.IsLoanable, &dt.IsConsumable, &p, &l) != nil { continue }
		dt.Brand = valStr(b); dt.Model = valStr(m)
		dts = append(dts, dt)
	}
	return dts
}

// ─── List ─────────────────────────────────────────────────────────

func (h *Handler) DeviceList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	switch c.DefaultQuery("tab", "list") {
	case "list":   h.deviceListTab(c, username, role)
	case "types":  h.deviceTypesTab(c, username, role)
	case "loans":  h.deviceLoansTab(c, username, role)
	case "usages": h.deviceUsagesTab(c, username, role)
	}
}

func (h *Handler) deviceListTab(c *gin.Context, username, role string) {
	search := c.Query("search")
	category := c.Query("category")

	query := `SELECT d.id, d.device_type_id, d.asset_code, d.name, dt.category, d.brand, d.model,
		d.item_type, d.quantity_total, d.quantity_available, d.condition, d.location, d.created_at
		FROM devices d JOIN device_types dt ON d.device_type_id = dt.id WHERE 1=1`
	var args []interface{}
	if search != "" {
		query += ` AND (d.name LIKE ? OR d.asset_code LIKE ? OR d.serial_number LIKE ?)`
		s := "%" + search + "%"; args = append(args, s, s, s)
	}
	if category != "" { query += ` AND dt.category = ?`; args = append(args, category) }
	query += ` ORDER BY d.asset_code`

	rows, err := h.db.Query(query, args...)
	if err != nil { h.errHTML(c, "Gagal mengambil data perangkat"); return }
	defer rows.Close()

	var devices []models.DeviceWithCategory
	for rows.Next() {
		var d models.DeviceWithCategory
		if rows.Scan(&d.ID, &d.DeviceTypeID, &d.AssetCode, &d.Name, &d.Category, &d.Brand, &d.Model,
			&d.ItemType, &d.QuantityTotal, &d.QuantityAvailable, &d.Condition, &d.Location, &d.CreatedAt) == nil {
			devices = append(devices, d)
		}
	}

	c.HTML(http.StatusOK, "device/list.html", gin.H{
		"title": "Manajemen Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"activeTab": "list", "devices": devices,
		"deviceTypes": h.fetchDeviceTypes(),
		"filters": gin.H{"search": search, "category": category},
	})
}

func (h *Handler) deviceTypesTab(c *gin.Context, username, role string) {
	search := c.Query("search")
	category := c.Query("category")

	query := `SELECT id, name, category, brand, model, item_type, is_loanable, is_consumable,
		asset_code_prefix, default_location, notes_template, created_at FROM device_types WHERE 1=1`
	var args []interface{}
	if search != "" { query += ` AND (name LIKE ? OR category LIKE ?)`; s := "%" + search + "%"; args = append(args, s, s) }
	if category != "" { query += ` AND category = ?`; args = append(args, category) }
	query += ` ORDER BY category, name`

	rows, err := h.db.Query(query, args...)
	if err != nil { h.errHTML(c, "Gagal mengambil data jenis barang"); return }
	defer rows.Close()

	var dts []models.DeviceType
	for rows.Next() {
		var dt models.DeviceType
		var b, m, p, l, n sql.NullString
		if rows.Scan(&dt.ID, &dt.Name, &dt.Category, &b, &m, &dt.ItemType, &dt.IsLoanable, &dt.IsConsumable, &p, &l, &n, &dt.CreatedAt) != nil { continue }
		dt.Brand = valStr(b); dt.Model = valStr(m)
		dts = append(dts, dt)
	}
	c.HTML(http.StatusOK, "device/list.html", gin.H{
		"title": "Jenis Barang", "currentPage": "devices",
		"username": username, "role": role,
		"activeTab": "types", "deviceTypes": dts,
	})
}

func (h *Handler) deviceLoansTab(c *gin.Context, username, role string) {
	rows, err := h.db.Query(`SELECT l.id, l.device_id, d.name, d.asset_code, l.borrower_name, l.borrower_type,
		l.loan_date, l.expected_return_date, l.actual_return_date, l.quantity, l.status, l.purpose,
		CASE WHEN l.actual_return_date IS NOT NULL THEN 'returned'
			WHEN l.expected_return_date IS NOT NULL AND CURRENT_DATE > l.expected_return_date THEN 'overdue'
			ELSE 'active' END
		FROM device_loans l JOIN devices d ON l.device_id = d.id ORDER BY l.loan_date DESC`)
	if err != nil { h.errHTML(c, "Gagal mengambil data peminjaman"); return }
	defer rows.Close()

	type LoanRow struct {
		ID, DeviceID int; DeviceName, AssetCode, BorrowerName, BorrowerType string
		LoanDate time.Time; ExpectedReturnDate, ActualReturnDate sql.NullTime
		Quantity int; Status, Purpose, ComputedStatus string
	}
	var loans []LoanRow
	for rows.Next() {
		var l LoanRow
		if rows.Scan(&l.ID, &l.DeviceID, &l.DeviceName, &l.AssetCode, &l.BorrowerName, &l.BorrowerType,
			&l.LoanDate, &l.ExpectedReturnDate, &l.ActualReturnDate, &l.Quantity, &l.Status, &l.Purpose, &l.ComputedStatus) == nil {
			loans = append(loans, l)
		}
	}
	c.HTML(http.StatusOK, "device/list.html", gin.H{
		"title": "Peminjaman", "currentPage": "devices",
		"username": username, "role": role,
		"activeTab": "loans", "loans": loans,
	})
}

func (h *Handler) deviceUsagesTab(c *gin.Context, username, role string) {
	rows, err := h.db.Query(`SELECT u.id, u.device_id, d.asset_code, d.name, u.user_name, u.user_type,
		u.usage_date, u.quantity, u.is_available, u.purpose
		FROM device_usages u JOIN devices d ON u.device_id = d.id ORDER BY u.usage_date DESC`)
	if err != nil { h.errHTML(c, "Gagal mengambil data pemakaian"); return }
	defer rows.Close()

	var usages []models.DeviceUsage
	for rows.Next() {
		var u models.DeviceUsage
		var ac, dn string
		if rows.Scan(&u.ID, &u.DeviceID, &ac, &dn, &u.UserName, &u.UserType, &u.UsageDate, &u.Quantity, &u.IsAvailable, &u.Purpose) == nil {
			u.DeviceAssetCode = ac; u.DeviceName = dn
			usages = append(usages, u)
		}
	}
	c.HTML(http.StatusOK, "device/list.html", gin.H{
		"title": "Pemakaian", "currentPage": "devices",
		"username": username, "role": role,
		"activeTab": "usages", "usages": usages,
	})
}

// ─── Create ───────────────────────────────────────────────────────

func (h *Handler) DeviceCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	c.HTML(http.StatusOK, "device/create.html", gin.H{
		"title": "Tambah Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"deviceTypes": h.fetchDeviceTypes(),
	})
}

func (h *Handler) DeviceCreate(c *gin.Context) {
	dtID := c.PostForm("device_type_id")
	name := c.PostForm("name")
	brand := c.PostForm("brand")
	model := c.PostForm("model")
	serial := c.PostForm("serial_number")
	itemType := c.PostForm("item_type")
	itemMode := c.PostForm("item_mode")
	qtyStr := c.PostForm("quantity_total")
	condition := c.PostForm("condition")
	location := c.PostForm("location")
	purchaseDate := c.PostForm("purchase_date")
	notes := c.PostForm("notes")

	qty, _ := strconv.Atoi(qtyStr)
	if dtID == "" || name == "" || qty <= 0 {
		c.HTML(http.StatusBadRequest, "device/create.html", gin.H{
			"title": "Tambah Perangkat", "error": "Lengkapi data yang diperlukan",
		})
		return
	}

	var prefix string
	h.db.QueryRow(`SELECT asset_code_prefix FROM device_types WHERE id = ?`, dtID).Scan(&prefix)
	code := fmt.Sprintf("%s-001", prefix)

	tx, err := h.db.Begin()
	if err != nil { h.errHTML(c, "Gagal memulai transaksi"); return }
	defer tx.Rollback()

	result, err := tx.Exec(`INSERT INTO devices (device_type_id, asset_code, name, brand, model, serial_number,
		item_type, is_loanable, is_consumable, quantity_total, quantity_available, condition, location, purchase_date, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dtID, code, name, brand, model, serial, itemType,
		itemMode == "loanable", itemMode == "consumable",
		qty, qty, condition, location, purchaseDate, notes)
	if err != nil {
		h.logCreateError(c, "device", map[string]interface{}{"name": name}, err.Error())
		c.HTML(http.StatusInternalServerError, "device/create.html", gin.H{
			"title": "Tambah Perangkat", "error": "Gagal menyimpan perangkat",
		})
		return
	}
	tx.Commit()

	id, _ := result.LastInsertId()
	h.logCreate(c, "device", int(id), map[string]interface{}{"name": name, "asset_code": code})
	c.Redirect(http.StatusFound, "/devices")
}

// ─── Detail ───────────────────────────────────────────────────────

func (h *Handler) DeviceDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id := c.Param("id")
	var d models.DeviceWithCategory
	err := h.db.QueryRow(`SELECT d.id, d.device_type_id, d.asset_code, d.name, dt.category, d.brand, d.model,
		d.serial_number, d.item_type, d.is_loanable, d.is_consumable, d.quantity_total, d.quantity_available,
		d.condition, d.location, d.purchase_date, d.notes, d.created_at, d.updated_at
		FROM devices d JOIN device_types dt ON d.device_type_id = dt.id WHERE d.id = ?`, id).
		Scan(&d.ID, &d.DeviceTypeID, &d.AssetCode, &d.Name, &d.Category, &d.Brand, &d.Model,
			&d.SerialNumber, &d.ItemType, &d.IsLoanable, &d.IsConsumable, &d.QuantityTotal,
			&d.QuantityAvailable, &d.Condition, &d.Location, &d.PurchaseDate, &d.Notes, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows { h.errHTML(c, "Perangkat tidak ditemukan"); return }
	if err != nil { h.errHTML(c, "Gagal mengambil data perangkat"); return }

	var dtName string
	h.db.QueryRow(`SELECT name FROM device_types WHERE id = ?`, d.DeviceTypeID).Scan(&dtName)

	var loans []models.DeviceLoan
	if lr, _ := h.db.Query(`SELECT id, borrower_name, loan_date, expected_return_date, actual_return_date, quantity, status FROM device_loans WHERE device_id = ? ORDER BY loan_date DESC LIMIT 10`, id); lr != nil {
		defer lr.Close()
		for lr.Next() {
			var l models.DeviceLoan
			if lr.Scan(&l.ID, &l.BorrowerName, &l.LoanDate, &l.ExpectedReturnDate, &l.ActualReturnDate, &l.Quantity, &l.Status) == nil {
				loans = append(loans, l)
			}
		}
	}

	var usages []models.DeviceUsage
	if d.IsConsumable {
		if ur, _ := h.db.Query(`SELECT id, user_name, usage_date, quantity, purpose, is_available FROM device_usages WHERE device_id = ? ORDER BY usage_date DESC LIMIT 10`, id); ur != nil {
			defer ur.Close()
			for ur.Next() {
				var u models.DeviceUsage
				if ur.Scan(&u.ID, &u.UserName, &u.UsageDate, &u.Quantity, &u.Purpose, &u.IsAvailable) == nil {
					usages = append(usages, u)
				}
			}
		}
	}

	c.HTML(http.StatusOK, "device/detail.html", gin.H{
		"title": "Detail Perangkat", "currentPage": "devices",
		"username": username, "role": role, "device": d,
		"deviceTypeName": dtName, "loans": loans, "usages": usages,
	})
}

// ─── Edit ─────────────────────────────────────────────────────────

func (h *Handler) DeviceEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id := c.Param("id")
	var d models.Device
	var brand, model, serial, location, notes, pDate sql.NullString
	err := h.db.QueryRow(`SELECT id, device_type_id, asset_code, name, brand, model, serial_number, item_type,
		is_loanable, is_consumable, quantity_total, quantity_available, condition, location, purchase_date, notes
		FROM devices WHERE id = ?`, id).
		Scan(&d.ID, &d.DeviceTypeID, &d.AssetCode, &d.Name, &brand, &model, &serial, &d.ItemType,
			&d.IsLoanable, &d.IsConsumable, &d.QuantityTotal, &d.QuantityAvailable, &d.Condition, &location, &pDate, &notes)
	if err != nil { h.errHTML(c, "Perangkat tidak ditemukan"); return }

	c.HTML(http.StatusOK, "device/edit.html", gin.H{
		"title": "Edit Perangkat", "currentPage": "devices",
		"username": username, "role": role, "device": d,
		"deviceTypes": h.fetchDeviceTypes(),
	})
}

func (h *Handler) DeviceEdit(c *gin.Context) {
	id := c.Param("id")
	dtID := c.PostForm("device_type_id")
	name := c.PostForm("name")
	brand := c.PostForm("brand")
	model := c.PostForm("model")
	serial := c.PostForm("serial_number")
	itemType := c.PostForm("item_type")
	itemMode := c.PostForm("item_mode")
	qtyTotalStr := c.PostForm("quantity_total")
	qtyAvailStr := c.PostForm("quantity_available")
	condition := c.PostForm("condition")
	location := c.PostForm("location")
	pDateForm := c.PostForm("purchase_date")
	notes := c.PostForm("notes")

	qtyTotal, _ := strconv.Atoi(qtyTotalStr)
	qtyAvail, _ := strconv.Atoi(qtyAvailStr)

	if _, err := h.db.Exec(`UPDATE devices SET device_type_id=?, name=?, brand=?, model=?, serial_number=?,
		item_type=?, is_loanable=?, is_consumable=?, quantity_total=?, quantity_available=?, condition=?,
		location=?, purchase_date=?, notes=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		dtID, name, brand, model, serial, itemType, itemMode == "loanable", itemMode == "consumable",
		qtyTotal, qtyAvail, condition, location, pDateForm, notes, id); err != nil {
		h.logUpdateError(c, "device", 0, map[string]interface{}{"id": id}, err.Error())
		h.errHTML(c, "Gagal mengupdate perangkat")
		return
	}

	h.logUpdate(c, "device", 0,
		map[string]interface{}{"id": id},
		map[string]interface{}{"name": name},
	)
	c.Redirect(http.StatusFound, "/devices")
}

// ─── Delete ───────────────────────────────────────────────────────

func (h *Handler) DeviceDelete(c *gin.Context) {
	id := c.Param("id")
	_, err := h.db.Exec("DELETE FROM devices WHERE id = ?", id)
	if err != nil {
		h.logDeleteError(c, "device", 0, map[string]interface{}{"id": id}, err.Error())
		h.redirectWithError(c, "/devices", "Gagal menghapus perangkat")
		return
	}
	h.logDelete(c, "device", 0, map[string]interface{}{"id": id})
	c.Redirect(http.StatusFound, "/devices")
}

// ─── Asset Code ───────────────────────────────────────────────────

func (h *Handler) GetNextAssetCode(c *gin.Context) {
	prefix := c.Query("prefix")
	var next int
	h.db.QueryRow(`SELECT COALESCE(MAX(CAST(SUBSTRING(asset_code, LENGTH(?) + 2) AS INTEGER)) + 1, 1) FROM devices WHERE asset_code LIKE ? || '-%'`, prefix, prefix).Scan(&next)
	c.JSON(http.StatusOK, gin.H{"next_code": fmt.Sprintf("%s-%03d", prefix, next)})
}

// ─── Export ───────────────────────────────────────────────────────

func (h *Handler) DeviceExport(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat export data"); return }

	pcs, _ := h.db.Query(`SELECT pc_number, "row", "column", status, device_type, serial_number, brand_model,
		processor, ram, storage, operating_system, accessories, purchase_date, last_checked, notes, action_notes
		FROM pcs ORDER BY pc_number`)
	if pcs != nil {
		defer pcs.Close()
	}
	devs, _ := h.db.Query(`SELECT d.asset_code, d.name, dt.category, d.brand, d.model, d.serial_number, d.quantity_total, d.quantity_available, d.condition, d.location FROM devices d JOIN device_types dt ON d.device_type_id = dt.id ORDER BY d.asset_code`)
	if devs != nil {
		defer devs.Close()
	}
	usages, _ := h.db.Query(`SELECT d.asset_code, d.name, u.user_name, u.user_type, u.usage_date, u.quantity, u.purpose FROM device_usages u JOIN devices d ON u.device_id = d.id ORDER BY u.usage_date DESC`)
	if usages != nil {
		defer usages.Close()
	}

	h.errHTML(c, "Fitur export akan diperbaiki")
}
