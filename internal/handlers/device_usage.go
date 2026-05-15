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

func (h *Handler) DeviceUsageList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	search := c.Query("search")
	sortBy := c.DefaultQuery("sort_by", "usage_date")
	sortOrder := c.DefaultQuery("sort_order", "DESC")

	query := `SELECT u.id, u.device_id, d.name, d.asset_code, u.user_name, u.user_type,
		u.usage_date, u.quantity, u.is_available, u.purpose
		FROM device_usages u JOIN devices d ON u.device_id = d.id WHERE 1=1`
	var args []interface{}

	if search != "" {
		query += ` AND (u.user_name LIKE ? OR d.name LIKE ? OR d.asset_code LIKE ?)`
		s := "%" + search + "%"
		args = append(args, s, s, s)
	}

	validSort := map[string]bool{"usage_date": true, "user_name": true, "quantity": true}
	if !validSort[sortBy] { sortBy = "usage_date" }
	if sortOrder != "ASC" { sortOrder = "DESC" }

	type UsageWithDevice struct {
		models.DeviceUsage
		DeviceName, AssetCode string
	}

	rows, err := h.db.Query(query+fmt.Sprintf(" ORDER BY u.%s %s", sortBy, sortOrder), args...)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data pemakaian")
		return
	}
	defer rows.Close()

	var usages []UsageWithDevice
	for rows.Next() {
		var u UsageWithDevice
		if rows.Scan(&u.ID, &u.DeviceID, &u.DeviceName, &u.AssetCode, &u.UserName, &u.UserType,
			&u.UsageDate, &u.Quantity, &u.IsAvailable, &u.Purpose) == nil {
			usages = append(usages, u)
		}
	}

	c.HTML(http.StatusOK, "device_usage/list.html", gin.H{
		"title": "Pemakaian Perangkat", "currentPage": "devices",
		"username": username, "role": role, "usages": usages,
		"filters": gin.H{"search": search, "sort_by": sortBy, "sort_order": sortOrder},
	})
}

func (h *Handler) DeviceUsageCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	rows, err := h.db.Query(`SELECT id, asset_code, name, item_type, quantity_available, is_consumable FROM devices WHERE is_consumable = TRUE AND quantity_available > 0 ORDER BY name`)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data perangkat")
		return
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var d models.Device
		if rows.Scan(&d.ID, &d.AssetCode, &d.Name, &d.ItemType, &d.QuantityAvailable, &d.IsConsumable) == nil {
			devices = append(devices, d)
		}
	}

	c.HTML(http.StatusOK, "device_usage/create.html", gin.H{
		"title": "Catat Pemakaian", "currentPage": "devices",
		"username": username, "role": role, "devices": devices,
		"deviceID": c.Query("device_id"),
	})
}

func (h *Handler) DeviceUsageCreate(c *gin.Context) {
	deviceID := c.PostForm("device_id")
	userName := c.PostForm("user_name")
	userType := c.PostForm("user_type")
	usageDateStr := c.PostForm("usage_date")
	quantityStr := c.PostForm("quantity")
	isAvailable := c.PostForm("is_available")
	purpose := c.PostForm("purpose")

	quantity, _ := strconv.Atoi(quantityStr)
	if deviceID == "" || userName == "" || usageDateStr == "" || quantity <= 0 {
		h.errHTML(c, "Perangkat, nama pengguna, tanggal, dan jumlah harus diisi")
		return
	}
	if isAvailable != "no" { isAvailable = "yes" }

	usageDate, _ := time.Parse("2006-01-02", usageDateStr)

	tx, _ := h.db.Begin()
	defer tx.Rollback()

	result, err := tx.Exec(`INSERT INTO device_usages (device_id, user_name, user_type, usage_date, quantity, is_available, purpose) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		deviceID, userName, userType, usageDate, quantity, isAvailable, purpose)
	if err != nil {
		h.errHTML(c, "Gagal menyimpan data pemakaian")
		return
	}

	if isAvailable == "no" {
		var qtyAvail int
		tx.QueryRow(`SELECT quantity_available FROM devices WHERE id = ?`, deviceID).Scan(&qtyAvail)
		if qtyAvail < quantity {
			h.errHTML(c, fmt.Sprintf("Stok tidak cukup. Tersedia: %d", qtyAvail))
			return
		}
		tx.Exec(`UPDATE devices SET quantity_available = quantity_available - ? WHERE id = ?`, quantity, deviceID)
	}
	tx.Commit()

	id, _ := result.LastInsertId()
	h.logCreate(c, "device_usage", int(id), map[string]interface{}{
		"device_id": deviceID, "user_name": userName, "quantity": quantity,
	})
	c.Redirect(http.StatusFound, "/devices?tab=usages")
}

func (h *Handler) DeviceUsageEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id := c.Param("id")
	var usage models.DeviceUsage
	var deviceName, assetCode string
	var purpose, notes sql.NullString
	err := h.db.QueryRow(`SELECT u.id, u.device_id, d.name, d.asset_code, u.user_name, u.user_type,
		u.usage_date, u.quantity, u.is_available, u.purpose, u.notes
		FROM device_usages u JOIN devices d ON u.device_id = d.id WHERE u.id = ?`, id).
		Scan(&usage.ID, &usage.DeviceID, &deviceName, &assetCode, &usage.UserName, &usage.UserType,
			&usage.UsageDate, &usage.Quantity, &usage.IsAvailable, &purpose, &notes)
	if err != nil {
		h.errHTML(c, "Data pemakaian tidak ditemukan")
		return
	}
	if purpose.Valid { usage.Purpose = purpose.String }
	if notes.Valid { usage.Notes = notes.String }

	c.HTML(http.StatusOK, "device_usage/edit.html", gin.H{
		"title": "Edit Pemakaian", "currentPage": "devices",
		"username": username, "role": role,
		"usage": usage, "deviceName": deviceName, "assetCode": assetCode,
	})
}

func (h *Handler) DeviceUsageEdit(c *gin.Context) {
	id := c.Param("id")
	userName := c.PostForm("user_name")
	userType := c.PostForm("user_type")
	usageDateStr := c.PostForm("usage_date")
	quantityStr := c.PostForm("quantity")
	isAvailable := c.PostForm("is_available")
	purpose := c.PostForm("purpose")
	notes := c.PostForm("notes")
	if isAvailable != "no" { isAvailable = "yes" }

	var oldID, oldQty int
	var oldAvail string
	h.db.QueryRow(`SELECT id, quantity, is_available FROM device_usages WHERE id = ?`, id).Scan(&oldID, &oldQty, &oldAvail)
	qty, _ := strconv.Atoi(quantityStr)
	date, _ := time.Parse("2006-01-02", usageDateStr)

	tx, _ := h.db.Begin()
	defer tx.Rollback()

	tx.Exec(`UPDATE device_usages SET user_name=?, user_type=?, usage_date=?, quantity=?, is_available=?, purpose=?, notes=? WHERE id=?`,
		userName, userType, date, qty, isAvailable, purpose, notes, id)

	if oldAvail != isAvailable {
		if isAvailable == "yes" {
			tx.Exec(`UPDATE devices SET quantity_available = quantity_available + ? WHERE id = ?`, qty, oldID)
		} else {
			tx.Exec(`UPDATE devices SET quantity_available = quantity_available - ? WHERE id = ?`, qty, oldID)
		}
	} else if qty != oldQty {
		tx.Exec(`UPDATE devices SET quantity_available = quantity_available + ? WHERE id = ?`, oldQty-qty, oldID)
	}
	tx.Commit()

	h.logUpdate(c, "device_usage", 0,
		map[string]interface{}{"id": id, "quantity": oldQty},
		map[string]interface{}{"quantity": qty, "is_available": isAvailable},
	)
	c.Redirect(http.StatusFound, "/devices?tab=usages")
}

func (h *Handler) DeviceUsageDelete(c *gin.Context) {
	id := c.Param("id")

	var deviceID, quantity int
	var isAvailable string
	h.db.QueryRow(`SELECT device_id, quantity, is_available FROM device_usages WHERE id = ?`, id).Scan(&deviceID, &quantity, &isAvailable)

	tx, _ := h.db.Begin()
	defer tx.Rollback()

	if isAvailable == "no" {
		tx.Exec(`UPDATE devices SET quantity_available = quantity_available + ? WHERE id = ?`, quantity, deviceID)
	}
	tx.Exec(`DELETE FROM device_usages WHERE id = ?`, id)
	tx.Commit()

	h.logDelete(c, "device_usage", 0, map[string]interface{}{"id": id})
	c.Redirect(http.StatusFound, "/devices?tab=usages")
}

func (h *Handler) DeviceUsageUpdateAvailability(c *gin.Context) {
	id := c.Param("id")
	isAvailable := c.PostForm("is_available")
	if isAvailable != "yes" && isAvailable != "no" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Nilai tidak valid"})
		return
	}

	var deviceID, quantity int
	var oldAvail string
	if err := h.db.QueryRow(`SELECT device_id, quantity, is_available FROM device_usages WHERE id = ?`, id).Scan(&deviceID, &quantity, &oldAvail); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Data tidak ditemukan"})
		return
	}
	if oldAvail == isAvailable {
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	tx, _ := h.db.Begin()
	defer tx.Rollback()
	tx.Exec(`UPDATE device_usages SET is_available = ? WHERE id = ?`, isAvailable, id)
	if isAvailable == "yes" {
		tx.Exec(`UPDATE devices SET quantity_available = quantity_available + ? WHERE id = ?`, quantity, deviceID)
	} else {
		tx.Exec(`UPDATE devices SET quantity_available = quantity_available - ? WHERE id = ?`, quantity, deviceID)
	}
	tx.Commit()

	if uid, u, r, ok := middleware.GetCurrentUser(c); ok {
		ip, ua := getRequestContext(c)
		h.activityLogService.LogUpdate(uid, u, r, "device_usage", 0,
			map[string]interface{}{"id": id},
			map[string]interface{}{"is_available": isAvailable}, ip, ua)
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
