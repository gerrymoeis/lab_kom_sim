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

// DeviceUsageList renders list of device usages with filter/sort/search
func (h *Handler) DeviceUsageList(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Parse filters
	search := c.Query("search")
	sortBy := c.DefaultQuery("sort_by", "usage_date")
	sortOrder := c.DefaultQuery("sort_order", "DESC")

	// Build query with JOIN to get device name
	query := `
		SELECT u.id, u.device_id, d.name as device_name, d.asset_code, u.user_name, u.user_type,
		       u.usage_date, u.quantity, u.purpose
		FROM device_usages u
		JOIN devices d ON u.device_id = d.id
		WHERE 1=1
	`
	args := []interface{}{}

	if search != "" {
		query += ` AND (u.user_name LIKE ? OR d.name LIKE ? OR d.asset_code LIKE ?)`
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	// Sorting
	validSortColumns := map[string]bool{
		"usage_date": true, "user_name": true, "quantity": true,
	}
	if !validSortColumns[sortBy] {
		sortBy = "usage_date"
	}
	if sortOrder != "ASC" && sortOrder != "DESC" {
		sortOrder = "DESC"
	}
	query += fmt.Sprintf(" ORDER BY u.%s %s", sortBy, sortOrder)

	rows, err := h.db.Query(query, args...)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data pemakaian",
		})
		return
	}
	defer rows.Close()

	type UsageWithDevice struct {
		models.DeviceUsage
		DeviceName string
		AssetCode  string
	}

	var usages []UsageWithDevice
	for rows.Next() {
		var usage UsageWithDevice
		err := rows.Scan(&usage.ID, &usage.DeviceID, &usage.DeviceName, &usage.AssetCode, &usage.UserName,
			&usage.UserType, &usage.UsageDate, &usage.Quantity, &usage.Purpose)
		if err != nil {
			continue
		}
		usages = append(usages, usage)
	}

	c.HTML(http.StatusOK, "device_usage/list.html", gin.H{
		"title":       "Pemakaian Perangkat - Sistem Inventaris Lab",
		"currentPage": "devices",
		"username":    username,
		"role":        role,
		"usages":      usages,
		"filters": gin.H{
			"search":     search,
			"sort_by":    sortBy,
			"sort_order": sortOrder,
		},
	})
}

// DeviceUsageCreatePage renders device usage creation form
func (h *Handler) DeviceUsageCreatePage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	deviceID := c.Query("device_id")

	// Get consumable devices
	rows, err := h.db.Query(`
		SELECT id, asset_code, name, item_type, quantity_available, is_consumable
		FROM devices
		WHERE is_consumable = TRUE AND quantity_available > 0
		ORDER BY name
	`)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data perangkat",
		})
		return
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var d models.Device
		rows.Scan(&d.ID, &d.AssetCode, &d.Name, &d.ItemType, &d.QuantityAvailable, &d.IsConsumable)
		devices = append(devices, d)
	}

	c.HTML(http.StatusOK, "device_usage/create.html", gin.H{
		"title":       "Catat Pemakaian - Sistem Inventaris Lab",
		"currentPage": "devices",
		"username":    username,
		"role":        role,
		"devices":     devices,
		"deviceID":    deviceID,
	})
}

// DeviceUsageCreate handles device usage creation
func (h *Handler) DeviceUsageCreate(c *gin.Context) {
	deviceID := c.PostForm("device_id")
	userName := c.PostForm("user_name")
	userType := c.PostForm("user_type")
	usageDateStr := c.PostForm("usage_date")
	quantityStr := c.PostForm("quantity")
	purpose := c.PostForm("purpose")

	// Validate
	if deviceID == "" || userName == "" || usageDateStr == "" || quantityStr == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "Error",
			"message": "Perangkat, nama pengguna, tanggal, dan jumlah harus diisi",
		})
		return
	}

	quantity, _ := strconv.Atoi(quantityStr)
	if quantity <= 0 {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "Error",
			"message": "Jumlah harus lebih dari 0",
		})
		return
	}

	// Check available quantity
	var quantityAvailable int
	err := h.db.QueryRow(`SELECT quantity_available FROM devices WHERE id = ?`, deviceID).Scan(&quantityAvailable)
	if err != nil || quantityAvailable < quantity {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "Error",
			"message": fmt.Sprintf("Stok tidak cukup. Tersedia: %d", quantityAvailable),
		})
		return
	}

	// Parse date
	usageDate, _ := time.Parse("2006-01-02", usageDateStr)

	// Begin transaction
	tx, err := h.db.Begin()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal memulai transaksi",
		})
		return
	}
	defer tx.Rollback()

	// Insert usage
	result, err := tx.Exec(`
		INSERT INTO device_usages (device_id, user_name, user_type, usage_date, quantity, purpose)
		VALUES (?, ?, ?, ?, ?, ?)
	`, deviceID, userName, userType, usageDate, quantity, purpose)

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal menyimpan data pemakaian",
		})
		return
	}

	// Update device quantity (permanent reduction)
	_, err = tx.Exec(`UPDATE devices SET quantity_available = quantity_available - ? WHERE id = ?`, quantity, deviceID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengupdate stok perangkat",
		})
		return
	}

	if err := tx.Commit(); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal menyimpan data",
		})
		return
	}

	// Log
	usageID, _ := result.LastInsertId()
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		h.activityLogService.LogCreate(
			userID, username, role,
			"device_usage", int(usageID),
			map[string]interface{}{
				"device_id": deviceID,
				"user_name": userName,
				"quantity":  quantity,
			},
			ipAddress, userAgent,
		)
	}

	c.Redirect(http.StatusFound, "/devices?tab=usages")
}

// DeviceUsageEditPage renders device usage edit form
func (h *Handler) DeviceUsageEditPage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	id := c.Param("id")

	var usage models.DeviceUsage
	var deviceName, assetCode string
	var purpose, notes sql.NullString
	err := h.db.QueryRow(`
		SELECT u.id, u.device_id, d.name, d.asset_code, u.user_name, u.user_type,
		       u.usage_date, u.quantity, u.purpose, u.notes
		FROM device_usages u
		JOIN devices d ON u.device_id = d.id
		WHERE u.id = ?
	`, id).Scan(&usage.ID, &usage.DeviceID, &deviceName, &assetCode, &usage.UserName, &usage.UserType,
		&usage.UsageDate, &usage.Quantity, &purpose, &notes)
	
	// Convert NullString to string
	if purpose.Valid {
		usage.Purpose = purpose.String
	}
	if notes.Valid {
		usage.Notes = notes.String
	}

	if err == sql.ErrNoRows {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Pemakaian Tidak Ditemukan",
			"message": "Data pemakaian tidak ditemukan",
		})
		return
	}

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data pemakaian",
		})
		return
	}

	c.HTML(http.StatusOK, "device_usage/edit.html", gin.H{
		"title":       "Edit Pemakaian - Sistem Inventaris Lab",
		"currentPage": "devices",
		"username":    username,
		"role":        role,
		"usage":       usage,
		"deviceName":  deviceName,
		"assetCode":   assetCode,
	})
}

// DeviceUsageEdit handles device usage update
func (h *Handler) DeviceUsageEdit(c *gin.Context) {
	id := c.Param("id")
	userName := c.PostForm("user_name")
	userType := c.PostForm("user_type")
	usageDateStr := c.PostForm("usage_date")
	quantityStr := c.PostForm("quantity")
	purpose := c.PostForm("purpose")
	notes := c.PostForm("notes")

	// Get current usage data
	var oldQuantity int
	var deviceID int
	err := h.db.QueryRow(`SELECT device_id, quantity FROM device_usages WHERE id = ?`, id).Scan(&deviceID, &oldQuantity)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data pemakaian",
		})
		return
	}

	newQuantity, _ := strconv.Atoi(quantityStr)
	usageDate, _ := time.Parse("2006-01-02", usageDateStr)

	// Begin transaction
	tx, err := h.db.Begin()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal memulai transaksi",
		})
		return
	}
	defer tx.Rollback()

	// Update usage
	_, err = tx.Exec(`
		UPDATE device_usages 
		SET user_name = ?, user_type = ?, usage_date = ?, quantity = ?, purpose = ?, notes = ?
		WHERE id = ?
	`, userName, userType, usageDate, newQuantity, purpose, notes, id)

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengupdate data pemakaian",
		})
		return
	}

	// Adjust device quantity if quantity changed
	if newQuantity != oldQuantity {
		quantityDiff := oldQuantity - newQuantity // If old=5, new=3, diff=2 (restore 2)
		_, err = tx.Exec(`UPDATE devices SET quantity_available = quantity_available + ? WHERE id = ?`, quantityDiff, deviceID)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title":   "Error",
				"message": "Gagal mengupdate stok perangkat",
			})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal menyimpan data",
		})
		return
	}

	// Log
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		usageIDInt, _ := strconv.Atoi(id)
		h.activityLogService.LogUpdate(
			userID, username, role,
			"device_usage", usageIDInt,
			map[string]interface{}{"quantity": oldQuantity},
			map[string]interface{}{"quantity": newQuantity},
			ipAddress, userAgent,
		)
	}

	c.Redirect(http.StatusFound, "/devices?tab=usages")
}

// DeviceUsageDelete handles device usage deletion
func (h *Handler) DeviceUsageDelete(c *gin.Context) {
	id := c.Param("id")

	// Get usage data
	var usageID, deviceID, quantity int
	var userName string
	err := h.db.QueryRow(`SELECT id, device_id, quantity, user_name FROM device_usages WHERE id = ?`, id).Scan(&usageID, &deviceID, &quantity, &userName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengambil data pemakaian",
		})
		return
	}

	// Begin transaction
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal memulai transaksi",
		})
		return
	}
	defer tx.Rollback()

	// Restore quantity
	_, err = tx.Exec(`UPDATE devices SET quantity_available = quantity_available + ? WHERE id = ?`, quantity, deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengupdate stok perangkat",
		})
		return
	}

	// Delete usage
	_, err = tx.Exec("DELETE FROM device_usages WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menghapus data pemakaian",
		})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menyimpan data",
		})
		return
	}

	// Log
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		h.activityLogService.LogDelete(
			userID, username, role,
			"device_usage", usageID,
			map[string]interface{}{
				"user_name": userName,
				"quantity":  quantity,
			},
			ipAddress, userAgent,
		)
	}

	c.Redirect(http.StatusFound, "/devices?tab=usages")
}
