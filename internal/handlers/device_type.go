package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"

	"github.com/gin-gonic/gin"
)

// DeviceTypeList renders list of device types with filter/sort/search
func (h *Handler) DeviceTypeList(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Parse filters
	search := c.Query("search")
	category := c.Query("category")
	itemType := c.Query("item_type")
	sortBy := c.DefaultQuery("sort_by", "name")
	sortOrder := c.DefaultQuery("sort_order", "ASC")

	// Build query
	query := `SELECT id, name, category, brand, model, item_type, is_loanable, is_consumable, 
	          asset_code_prefix, default_location, created_at 
	          FROM device_types WHERE 1=1`
	args := []interface{}{}

	if search != "" {
		query += ` AND (name LIKE ? OR brand LIKE ? OR model LIKE ?)`
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	if category != "" {
		query += ` AND category = ?`
		args = append(args, category)
	}

	if itemType != "" {
		query += ` AND item_type = ?`
		args = append(args, itemType)
	}

	// Sorting
	validSortColumns := map[string]bool{
		"name": true, "category": true, "item_type": true, "created_at": true,
	}
	if !validSortColumns[sortBy] {
		sortBy = "name"
	}
	if sortOrder != "ASC" && sortOrder != "DESC" {
		sortOrder = "ASC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

	rows, err := h.db.Query(query, args...)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data jenis barang",
		})
		return
	}
	defer rows.Close()

	var deviceTypes []models.DeviceType
	for rows.Next() {
		var dt models.DeviceType
		var brand, model, assetCodePrefix, defaultLocation sql.NullString
		
		err := rows.Scan(&dt.ID, &dt.Name, &dt.Category, &brand, &model, &dt.ItemType,
			&dt.IsLoanable, &dt.IsConsumable, &assetCodePrefix, &defaultLocation, &dt.CreatedAt)
		if err != nil {
			continue
		}
		
		// Convert NullString to string
		if brand.Valid {
			dt.Brand = brand.String
		}
		if model.Valid {
			dt.Model = model.String
		}
		if assetCodePrefix.Valid {
			dt.AssetCodePrefix = assetCodePrefix.String
		}
		if defaultLocation.Valid {
			dt.DefaultLocation = defaultLocation.String
		}
		
		deviceTypes = append(deviceTypes, dt)
	}

	c.HTML(http.StatusOK, "device_type/list.html", gin.H{
		"title":       "Jenis Barang - Sistem Inventaris Lab",
		"currentPage": "devices",
		"username":    username,
		"role":        role,
		"deviceTypes": deviceTypes,
		"filters": gin.H{
			"search":     search,
			"category":   category,
			"item_type":  itemType,
			"sort_by":    sortBy,
			"sort_order": sortOrder,
		},
	})
}

// DeviceTypeDetail renders device type detail page
func (h *Handler) DeviceTypeDetail(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	id := c.Param("id")
	var dt models.DeviceType
	var brand, model, assetCodePrefix, defaultLocation, notesTemplate sql.NullString

	err := h.db.QueryRow(`
		SELECT id, name, category, brand, model, item_type, is_loanable, is_consumable,
		       asset_code_prefix, default_location, notes_template, created_at, updated_at
		FROM device_types WHERE id = ?
	`, id).Scan(&dt.ID, &dt.Name, &dt.Category, &brand, &model, &dt.ItemType,
		&dt.IsLoanable, &dt.IsConsumable, &assetCodePrefix, &defaultLocation,
		&notesTemplate, &dt.CreatedAt, &dt.UpdatedAt)

	if err == sql.ErrNoRows {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Jenis Barang Tidak Ditemukan",
			"message": "Jenis barang yang Anda cari tidak ditemukan",
		})
		return
	}

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data jenis barang",
		})
		return
	}

	// Convert NullString to string
	if brand.Valid {
		dt.Brand = brand.String
	}
	if model.Valid {
		dt.Model = model.String
	}
	if assetCodePrefix.Valid {
		dt.AssetCodePrefix = assetCodePrefix.String
	}
	if defaultLocation.Valid {
		dt.DefaultLocation = defaultLocation.String
	}
	if notesTemplate.Valid {
		dt.NotesTemplate = notesTemplate.String
	}

	// Get count of devices using this type
	var deviceCount int
	h.db.QueryRow(`SELECT COUNT(*) FROM devices WHERE device_type_id = ?`, id).Scan(&deviceCount)

	c.HTML(http.StatusOK, "device_type/detail.html", gin.H{
		"title":       "Detail Jenis Barang - Sistem Inventaris Lab",
		"currentPage": "devices",
		"username":    username,
		"role":        role,
		"deviceType":  dt,
		"deviceCount": deviceCount,
	})
}

// DeviceTypeCreatePage renders device type creation form
func (h *Handler) DeviceTypeCreatePage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	c.HTML(http.StatusOK, "device_type/create.html", gin.H{
		"title":       "Tambah Jenis Barang - Sistem Inventaris Lab",
		"currentPage": "devices",
		"username":    username,
		"role":        role,
	})
}

// DeviceTypeCreate handles device type creation
func (h *Handler) DeviceTypeCreate(c *gin.Context) {
	name := c.PostForm("name")
	category := c.PostForm("category")
	brand := c.PostForm("brand")
	model := c.PostForm("model")
	itemType := c.PostForm("item_type")
	itemMode := c.PostForm("item_mode")
	isLoanable := itemMode == "loanable"
	isConsumable := itemMode == "consumable"
	assetCodePrefix := c.PostForm("asset_code_prefix")
	defaultLocation := c.PostForm("default_location")
	notesTemplate := c.PostForm("notes_template")

	if name == "" || category == "" || itemType == "" {
		c.HTML(http.StatusBadRequest, "device_type/create.html", gin.H{
			"title": "Tambah Jenis Barang",
			"error": "Nama, kategori, dan tipe item harus diisi",
		})
		return
	}

	result, err := h.db.Exec(`
		INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable,
		                          asset_code_prefix, default_location, notes_template)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, name, category, brand, model, itemType, isLoanable, isConsumable, assetCodePrefix, defaultLocation, notesTemplate)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			c.HTML(http.StatusBadRequest, "device_type/create.html", gin.H{
				"title": "Tambah Jenis Barang",
				"error": "Nama jenis barang sudah ada",
			})
			return
		}

		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogCreate(
				userID, username, role, "device_type", 0,
				map[string]interface{}{"error": "Failed to create device type"},
				ipAddress, userAgent, fmt.Sprintf("Failed to create device type: %v", err),
			)
		}
		c.HTML(http.StatusInternalServerError, "device_type/create.html", gin.H{
			"title": "Tambah Jenis Barang",
			"error": "Gagal menyimpan data jenis barang",
		})
		return
	}

	// Log successful create
	deviceTypeID, _ := result.LastInsertId()
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		h.activityLogService.LogCreate(
			userID, username, role,
			"device_type", int(deviceTypeID),
			map[string]interface{}{
				"name":     name,
				"category": category,
			},
			ipAddress, userAgent,
		)
	}

	c.Redirect(http.StatusFound, "/devices?tab=types")
}

// DeviceTypeEditPage renders device type edit form
func (h *Handler) DeviceTypeEditPage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	id := c.Param("id")
	var dt models.DeviceType
	var brand, model, assetCodePrefix, defaultLocation, notesTemplate sql.NullString

	err := h.db.QueryRow(`
		SELECT id, name, category, brand, model, item_type, is_loanable, is_consumable,
		       asset_code_prefix, default_location, notes_template
		FROM device_types WHERE id = ?
	`, id).Scan(&dt.ID, &dt.Name, &dt.Category, &brand, &model, &dt.ItemType,
		&dt.IsLoanable, &dt.IsConsumable, &assetCodePrefix, &defaultLocation, &notesTemplate)

	if err == sql.ErrNoRows {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Jenis Barang Tidak Ditemukan",
			"message": "Jenis barang yang Anda cari tidak ditemukan",
		})
		return
	}

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data jenis barang",
		})
		return
	}

	// Convert NullString to string
	if brand.Valid {
		dt.Brand = brand.String
	}
	if model.Valid {
		dt.Model = model.String
	}
	if assetCodePrefix.Valid {
		dt.AssetCodePrefix = assetCodePrefix.String
	}
	if defaultLocation.Valid {
		dt.DefaultLocation = defaultLocation.String
	}
	if notesTemplate.Valid {
		dt.NotesTemplate = notesTemplate.String
	}

	c.HTML(http.StatusOK, "device_type/edit.html", gin.H{
		"title":       "Edit Jenis Barang - Sistem Inventaris Lab",
		"currentPage": "devices",
		"username":    username,
		"role":        role,
		"deviceType":  dt,
	})
}

// DeviceTypeEdit handles device type update
func (h *Handler) DeviceTypeEdit(c *gin.Context) {
	id := c.Param("id")
	name := c.PostForm("name")
	category := c.PostForm("category")
	brand := c.PostForm("brand")
	model := c.PostForm("model")
	itemType := c.PostForm("item_type")
	itemMode := c.PostForm("item_mode")
	isLoanable := itemMode == "loanable"
	isConsumable := itemMode == "consumable"
	assetCodePrefix := c.PostForm("asset_code_prefix")
	defaultLocation := c.PostForm("default_location")
	notesTemplate := c.PostForm("notes_template")

	// Get old values for logging
	var oldName, oldCategory string
	err := h.db.QueryRow(`SELECT name, category FROM device_types WHERE id = ?`, id).Scan(&oldName, &oldCategory)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data jenis barang",
		})
		return
	}

	_, err = h.db.Exec(`
		UPDATE device_types 
		SET name = ?, category = ?, brand = ?, model = ?, item_type = ?, is_loanable = ?, is_consumable = ?,
		    asset_code_prefix = ?, default_location = ?, notes_template = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, name, category, brand, model, itemType, isLoanable, isConsumable, assetCodePrefix, defaultLocation, notesTemplate, id)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"title":   "Error",
				"message": "Nama jenis barang sudah ada",
			})
			return
		}

		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			deviceTypeIDInt, _ := strconv.Atoi(id)
			h.activityLogService.LogUpdate(
				userID, username, role, "device_type", deviceTypeIDInt,
				map[string]interface{}{"id": deviceTypeIDInt}, map[string]interface{}{"error": err.Error()},
				ipAddress, userAgent, fmt.Sprintf("Failed to update device type #%d: %v", deviceTypeIDInt, err),
			)
		}
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengupdate data jenis barang",
		})
		return
	}

	// Log successful update
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		deviceTypeIDInt, _ := strconv.Atoi(id)

		h.activityLogService.LogUpdate(
			userID, username, role,
			"device_type", deviceTypeIDInt,
			map[string]interface{}{"name": oldName, "category": oldCategory},
			map[string]interface{}{"name": name, "category": category},
			ipAddress, userAgent,
		)
	}

	c.Redirect(http.StatusFound, "/devices?tab=types")
}

// DeviceTypeDelete handles device type deletion
func (h *Handler) DeviceTypeDelete(c *gin.Context) {
	id := c.Param("id")

	// Check if any devices use this type
	var deviceCount int
	h.db.QueryRow(`SELECT COUNT(*) FROM devices WHERE device_type_id = ?`, id).Scan(&deviceCount)

	if deviceCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Tidak dapat menghapus. Ada %d perangkat yang menggunakan jenis ini", deviceCount),
		})
		return
	}

	// Get data before delete
	var deviceTypeID int
	var name, category string
	err := h.db.QueryRow(`SELECT id, name, category FROM device_types WHERE id = ?`, id).Scan(&deviceTypeID, &name, &category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengambil data jenis barang",
		})
		return
	}

	_, err = h.db.Exec("DELETE FROM device_types WHERE id = ?", id)
	if err != nil {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogDelete(
				userID, username, role, "device_type", deviceTypeID,
				map[string]interface{}{"id": deviceTypeID},
				ipAddress, userAgent, fmt.Sprintf("Failed to delete device type #%d: %v", deviceTypeID, err),
			)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menghapus jenis barang",
		})
		return
	}

	// Log successful delete
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		h.activityLogService.LogDelete(
			userID, username, role,
			"device_type", deviceTypeID,
			map[string]interface{}{"name": name, "category": category},
			ipAddress, userAgent,
		)
	}

	c.Redirect(http.StatusFound, "/devices?tab=types")
}
