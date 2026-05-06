package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

// DeviceList renders list of all devices
func (h *Handler) DeviceList(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	rows, err := h.db.Query(`
		SELECT id, name, category, brand, condition, location, purchase_date, notes, created_at
		FROM devices
		ORDER BY category, name
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
		var device models.Device
		err := rows.Scan(&device.ID, &device.Name, &device.Category, &device.Brand,
			&device.Condition, &device.Location, &device.PurchaseDate, &device.Notes, &device.CreatedAt)
		if err != nil {
			continue
		}
		devices = append(devices, device)
	}

	c.HTML(http.StatusOK, "device/list.html", gin.H{
		"title":       "Daftar Perangkat - Sistem Inventaris Lab",
		"currentPage": "devices",
		"username":    username,
		"role":        role,
		"devices":     devices,
	})
}

// DeviceCreatePage renders device creation form
func (h *Handler) DeviceCreatePage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	c.HTML(http.StatusOK, "device/create.html", gin.H{
		"title":       "Tambah Perangkat Baru - Sistem Inventaris Lab",
		"currentPage": "devices",
		"username":    username,
		"role":        role,
	})
}

// DeviceCreate handles device creation
func (h *Handler) DeviceCreate(c *gin.Context) {
	name := c.PostForm("name")
	category := c.PostForm("category")
	brand := c.PostForm("brand")
	condition := c.PostForm("condition")
	location := c.PostForm("location")
	purchaseDate := c.PostForm("purchase_date")
	notes := c.PostForm("notes")

	if name == "" || category == "" {
		c.HTML(http.StatusBadRequest, "device/create.html", gin.H{
			"title": "Tambah Perangkat Baru",
			"error": "Nama dan kategori harus diisi",
		})
		return
	}

	var purchaseDatePtr *string
	if purchaseDate != "" {
		purchaseDatePtr = &purchaseDate
	}

	_, err := h.db.Exec(`
		INSERT INTO devices (name, category, brand, condition, location, purchase_date, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, name, category, brand, condition, location, purchaseDatePtr, notes)

	if err != nil {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogAuth(
				userID, username, role, "create", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to create device: %v", err),
			)
		}
		c.HTML(http.StatusInternalServerError, "device/create.html", gin.H{
			"title": "Tambah Perangkat Baru",
			"error": "Gagal menyimpan data perangkat",
		})
		return
	}

	// Get last insert ID and log
	var deviceID int
	err = h.db.QueryRow("SELECT last_insert_rowid()").Scan(&deviceID)
	if err == nil {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogCreate(
				userID, username, role,
				"device", deviceID,
				map[string]interface{}{
					"name":      name,
					"category":  category,
					"condition": condition,
				},
				ipAddress, userAgent,
			)
		}
	}

	c.Redirect(http.StatusFound, "/devices")
}

// DeviceEditPage renders device edit form
func (h *Handler) DeviceEditPage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	id := c.Param("id")
	var device models.Device
	var purchaseDateStr sql.NullString

	err := h.db.QueryRow(`
		SELECT id, name, category, brand, condition, location, purchase_date, notes
		FROM devices WHERE id = ?
	`, id).Scan(&device.ID, &device.Name, &device.Category, &device.Brand,
		&device.Condition, &device.Location, &purchaseDateStr, &device.Notes)

	if err == sql.ErrNoRows {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Perangkat Tidak Ditemukan",
			"message": "Perangkat yang Anda cari tidak ditemukan",
		})
		return
	}

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data perangkat",
		})
		return
	}

	var purchaseDateFormatted string
	if purchaseDateStr.Valid {
		// Try multiple date formats
		formats := []string{"2006-01-02", "2006-01-02T15:04:05Z", time.RFC3339}
		for _, format := range formats {
			if t, err := time.Parse(format, purchaseDateStr.String); err == nil {
				purchaseDateFormatted = t.Format("2006-01-02")
				break
			}
		}
		// If parsing fails, use the string as-is (fallback)
		if purchaseDateFormatted == "" {
			purchaseDateFormatted = purchaseDateStr.String
		}
	}

	c.HTML(http.StatusOK, "device/edit.html", gin.H{
		"title":        "Edit Perangkat - Sistem Inventaris Lab",
		"currentPage":  "devices",
		"username":     username,
		"role":         role,
		"device":       device,
		"purchaseDate": purchaseDateFormatted,
	})
}

// DeviceEdit handles device update
func (h *Handler) DeviceEdit(c *gin.Context) {
	id := c.Param("id")
	name := c.PostForm("name")
	category := c.PostForm("category")
	brand := c.PostForm("brand")
	condition := c.PostForm("condition")
	location := c.PostForm("location")
	purchaseDateForm := c.PostForm("purchase_date")
	notes := c.PostForm("notes")

	// Get current device data including purchase_date and old values for logging
	var currentPurchaseDate sql.NullString
	var oldName, oldCategory, oldCondition string
	err := h.db.QueryRow(`
		SELECT name, category, condition, purchase_date
		FROM devices WHERE id = ?
	`, id).Scan(&oldName, &oldCategory, &oldCondition, &currentPurchaseDate)
	
	if err != nil {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogAuth(
				userID, username, role, "update", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to get device data for update: %v", err),
			)
		}
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data perangkat",
		})
		return
	}

	// Preserve existing purchase_date if form field is empty
	var purchaseDatePtr *string
	if purchaseDateForm != "" {
		// User provided new value
		purchaseDatePtr = &purchaseDateForm
	} else if currentPurchaseDate.Valid {
		// Preserve existing value
		purchaseDatePtr = &currentPurchaseDate.String
	}
	// If both empty, purchaseDatePtr = nil (set to NULL)

	_, err = h.db.Exec(`
		UPDATE devices 
		SET name = ?, category = ?, brand = ?, condition = ?, location = ?,
		    purchase_date = ?, notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, name, category, brand, condition, location, purchaseDatePtr, notes, id)

	if err != nil {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			deviceIDInt, _ := strconv.Atoi(id)
			h.activityLogService.LogAuth(
				userID, username, role, "update", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to update device #%d: %v", deviceIDInt, err),
			)
		}
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengupdate data perangkat",
		})
		return
	}

	// Log successful update
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		deviceIDInt, _ := strconv.Atoi(id)
		
		oldValues := map[string]interface{}{
			"name":      oldName,
			"category":  oldCategory,
			"condition": oldCondition,
		}
		
		newValues := map[string]interface{}{
			"name":      name,
			"category":  category,
			"condition": condition,
		}
		
		h.activityLogService.LogUpdate(
			userID, username, role,
			"device", deviceIDInt,
			oldValues,
			newValues,
			ipAddress, userAgent,
		)
	}

	c.Redirect(http.StatusFound, "/devices")
}

// DeviceDelete handles device deletion
func (h *Handler) DeviceDelete(c *gin.Context) {
	id := c.Param("id")

	// Get device data before delete
	var deviceID int
	var name, category, condition string
	err := h.db.QueryRow(`
		SELECT id, name, category, condition 
		FROM devices WHERE id = ?
	`, id).Scan(&deviceID, &name, &category, &condition)
	
	if err != nil {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogAuth(
				userID, username, role, "delete", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to get device data for delete: %v", err),
			)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengambil data perangkat",
		})
		return
	}

	_, err = h.db.Exec("DELETE FROM devices WHERE id = ?", id)
	if err != nil {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogAuth(
				userID, username, role, "delete", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to delete device #%d: %v", deviceID, err),
			)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menghapus perangkat",
		})
		return
	}

	// Log successful delete
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		
		oldValues := map[string]interface{}{
			"name":      name,
			"category":  category,
			"condition": condition,
		}
		
		h.activityLogService.LogDelete(
			userID, username, role,
			"device", deviceID,
			oldValues,
			ipAddress, userAgent,
		)
	}

	c.Redirect(http.StatusFound, "/devices")
}

// DeviceExport exports device list to Excel
func (h *Handler) DeviceExport(c *gin.Context) {
	_, _, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	if role != "admin" {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"title":   "Akses Ditolak",
			"message": "Hanya admin yang dapat export data perangkat",
		})
		return
	}

	// Query all devices
	rows, err := h.db.Query(`
		SELECT id, name, category, brand, condition, location, purchase_date, notes
		FROM devices
		ORDER BY category, name
	`)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data perangkat",
		})
		return
	}
	defer rows.Close()

	// Collect data
	type DeviceExportData struct {
		ID           int
		Name         string
		Category     string
		Brand        string
		Condition    string
		Location     string
		PurchaseDate sql.NullString
		Notes        string
	}

	var devices []DeviceExportData
	for rows.Next() {
		var device DeviceExportData
		err := rows.Scan(&device.ID, &device.Name, &device.Category, &device.Brand,
			&device.Condition, &device.Location, &device.PurchaseDate, &device.Notes)
		if err != nil {
			continue
		}
		devices = append(devices, device)
	}

	// Category translation map
	categoryMap := map[string]string{
		"printer":         "Printer",
		"router":          "Router",
		"speaker":         "Speaker",
		"pc_cadangan":     "PC Cadangan",
		"komputer_labor":  "Komputer Labor",
		"komputer_dosen":  "Komputer Dosen",
		"lainnya":         "Lainnya",
	}

	// Condition translation map
	conditionMap := map[string]string{
		"baik":        "Baik",
		"rusak":       "Rusak",
		"maintenance": "Maintenance",
	}

	// Transform data to [][]interface{}
	data := [][]interface{}{}
	for i, device := range devices {
		// Translate category
		category := categoryMap[device.Category]
		if category == "" {
			category = device.Category
		}

		// Translate condition
		condition := conditionMap[device.Condition]
		if condition == "" {
			condition = device.Condition
		}

		// Format purchase date
		purchaseDate := "-"
		if device.PurchaseDate.Valid && device.PurchaseDate.String != "" {
			formats := []string{"2006-01-02", "2006-01-02T15:04:05Z", time.RFC3339}
			for _, format := range formats {
				if t, err := time.Parse(format, device.PurchaseDate.String); err == nil {
					purchaseDate = t.Format("02/01/2006")
					break
				}
			}
			if purchaseDate == "-" {
				purchaseDate = device.PurchaseDate.String
			}
		}

		// Handle empty values
		brand := device.Brand
		if brand == "" {
			brand = "-"
		}
		location := device.Location
		if location == "" {
			location = "-"
		}
		notes := device.Notes
		if notes == "" {
			notes = "-"
		}

		row := []interface{}{
			i + 1,       // No
			device.Name, // Name
			category,    // Category
			brand,       // Brand
			condition,   // Condition
			location,    // Location
			purchaseDate, // Purchase Date
			notes,       // Notes
		}
		data = append(data, row)
	}

	// Prepare conditional formatting for condition column (column E, index 4)
	conditionalFormats := []services.ConditionalFormat{}
	if len(data) > 0 {
		conditionalFormats = []services.ConditionalFormat{
			{
				Column:    "E",
				RowStart:  2,
				RowEnd:    len(data) + 1,
				Condition: "Baik",
				Color:     "#92D050", // Green
			},
			{
				Column:    "E",
				RowStart:  2,
				RowEnd:    len(data) + 1,
				Condition: "Rusak",
				Color:     "#FFC7CE", // Red
			},
			{
				Column:    "E",
				RowStart:  2,
				RowEnd:    len(data) + 1,
				Condition: "Maintenance",
				Color:     "#FFEB9C", // Yellow
			},
		}
	}

	// Configure Excel export
	excelService := services.NewExcelService()
	config := services.ExcelExportConfig{
		SheetName: "Daftar Perangkat",
		Headers: []string{
			"No", "Nama Perangkat", "Kategori", "Merk", "Kondisi",
			"Lokasi", "Tanggal Pembelian", "Catatan",
		},
		Data: data,
		ColumnWidths: map[string]float64{
			"A": 5,   // No
			"B": 25,  // Nama Perangkat
			"C": 18,  // Kategori
			"D": 15,  // Merk
			"E": 12,  // Kondisi
			"F": 20,  // Lokasi
			"G": 16,  // Tanggal Pembelian
			"H": 30,  // Catatan
		},
		ConditionalFormats: conditionalFormats,
	}

	// Generate Excel file
	f, err := excelService.GenerateExcel(config)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal generate file Excel: " + err.Error(),
		})
		return
	}
	defer f.Close()

	// Generate filename: devices_export_HHMM_DDMMYYYY.xlsx
	filename := excelService.GenerateFilename("devices_export")

	// Set headers for download
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Transfer-Encoding", "binary")

	// Write to response
	if err := f.Write(c.Writer); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal generate file Excel",
		})
		return
	}
}
