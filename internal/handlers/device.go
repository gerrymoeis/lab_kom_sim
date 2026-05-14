package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

// Helper function to fetch device types for dropdown
func (h *Handler) fetchDeviceTypes() []models.DeviceType {
	rows, err := h.db.Query(`
		SELECT id, name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location
		FROM device_types ORDER BY category, name
	`)
	if err != nil {
		return []models.DeviceType{}
	}
	defer rows.Close()

	var deviceTypes []models.DeviceType
	for rows.Next() {
		var dt models.DeviceType
		var brand, model, assetCodePrefix, defaultLocation sql.NullString
		
		err := rows.Scan(&dt.ID, &dt.Name, &dt.Category, &brand, &model, &dt.ItemType,
			&dt.IsLoanable, &dt.IsConsumable, &assetCodePrefix, &defaultLocation)
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
	
	return deviceTypes
}

// Helper function to render device create page with error
func (h *Handler) renderDeviceCreateWithError(c *gin.Context, errorMsg string) {
	_, username, role, _ := middleware.GetCurrentUser(c)
	deviceTypes := h.fetchDeviceTypes()
	
	c.HTML(http.StatusBadRequest, "device/create.html", gin.H{
		"title":       "Tambah Perangkat Baru - Sistem Inventaris Lab",
		"currentPage": "devices",
		"username":    username,
		"role":        role,
		"deviceTypes": deviceTypes,
		"error":       errorMsg,
	})
}

// DeviceList renders list of all devices with tab navigation
func (h *Handler) DeviceList(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Get active tab (default: list)
	tab := c.DefaultQuery("tab", "list")

	// Parse filters
	search := c.Query("search")
	category := c.Query("category")
	itemType := c.Query("item_type")
	condition := c.Query("condition")
	status := c.Query("status")
	sortBy := c.DefaultQuery("sort_by", "name")
	sortOrder := c.DefaultQuery("sort_order", "ASC")

	// Prepare data based on active tab
	var devices []models.DeviceWithCategory
	var deviceTypes []models.DeviceType
	var deviceLoans []models.DeviceLoan
	var deviceUsages []models.DeviceUsage

	switch tab {
	case "list":
		// Daftar Perangkat (JOIN with device_types for category)
		query := `SELECT d.id, d.asset_code, d.name, dt.category, d.brand, d.model, d.item_type, 
		                 d.quantity_total, d.quantity_available, d.condition, d.location, d.created_at 
		          FROM devices d
		          JOIN device_types dt ON d.device_type_id = dt.id
		          WHERE 1=1`
		args := []interface{}{}

		if search != "" {
			query += ` AND (d.name LIKE ? OR d.asset_code LIKE ? OR d.brand LIKE ?)`
			searchTerm := "%" + search + "%"
			args = append(args, searchTerm, searchTerm, searchTerm)
		}

		if category != "" {
			query += ` AND dt.category = ?`
			args = append(args, category)
		}

		if itemType != "" {
			query += ` AND d.item_type = ?`
			args = append(args, itemType)
		}

		if condition != "" {
			query += ` AND d.condition = ?`
			args = append(args, condition)
		}

		// Sorting
		validSortColumns := map[string]bool{
			"name": true, "asset_code": true, "category": true, "condition": true, "created_at": true,
		}
		if !validSortColumns[sortBy] {
			sortBy = "name"
		}
		if sortOrder != "ASC" && sortOrder != "DESC" {
			sortOrder = "ASC"
		}
		// Prefix column names for sorting
		sortColumn := sortBy
		if sortBy == "name" || sortBy == "asset_code" || sortBy == "condition" || sortBy == "created_at" {
			sortColumn = "d." + sortBy
		} else if sortBy == "category" {
			sortColumn = "dt.category"
		}
		query += fmt.Sprintf(" ORDER BY %s %s", sortColumn, sortOrder)

		rows, err := h.db.Query(query, args...)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title":   "Error",
				"message": "Gagal mengambil data perangkat",
			})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var device models.DeviceWithCategory
			err := rows.Scan(&device.ID, &device.AssetCode, &device.Name, &device.Category, &device.Brand,
				&device.Model, &device.ItemType, &device.QuantityTotal, &device.QuantityAvailable,
				&device.Condition, &device.Location, &device.CreatedAt)
			if err != nil {
				continue
			}
			devices = append(devices, device)
		}

	case "types":
		// Jenis Barang
		query := `SELECT id, name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, created_at
		          FROM device_types WHERE 1=1`
		args := []interface{}{}

		if search != "" {
			query += ` AND (name LIKE ? OR category LIKE ?)`
			searchTerm := "%" + search + "%"
			args = append(args, searchTerm, searchTerm)
		}

		if category != "" {
			query += ` AND category = ?`
			args = append(args, category)
		}

		query += ` ORDER BY category, name`

		rows, err := h.db.Query(query, args...)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title":   "Error",
				"message": "Gagal mengambil data jenis barang",
			})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var dt models.DeviceType
			var brand, model, assetCodePrefix, defaultLocation sql.NullString
			
			err := rows.Scan(&dt.ID, &dt.Name, &dt.Category, &brand, &model, &dt.ItemType,
				&dt.IsLoanable, &dt.IsConsumable, &assetCodePrefix, &defaultLocation, &dt.CreatedAt)
			if err != nil {
				fmt.Printf("Error scanning device_type row: %v\n", err)
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

	case "loans":
		// Peminjaman with computed status
		query := `SELECT l.id, l.device_id, d.asset_code, d.name, l.borrower_name, l.borrower_type, 
		                 l.loan_date, l.expected_return_date, l.actual_return_date, l.quantity, l.status, l.purpose,
		                 CASE 
		                   WHEN l.actual_return_date IS NOT NULL THEN 'returned'
		                   WHEN l.expected_return_date IS NOT NULL AND CURRENT_DATE > l.expected_return_date THEN 'overdue'
		                   ELSE 'active'
		                 END as computed_status
		          FROM device_loans l
		          JOIN devices d ON l.device_id = d.id
		          WHERE 1=1`
		args := []interface{}{}

		if search != "" {
			query += ` AND (l.borrower_name LIKE ? OR d.name LIKE ? OR d.asset_code LIKE ?)`
			searchTerm := "%" + search + "%"
			args = append(args, searchTerm, searchTerm, searchTerm)
		}

		if status != "" {
			// Filter by computed status
			query += ` AND (CASE 
			              WHEN l.actual_return_date IS NOT NULL THEN 'returned'
			              WHEN l.expected_return_date IS NOT NULL AND CURRENT_DATE > l.expected_return_date THEN 'overdue'
			              ELSE 'active'
			            END) = ?`
			args = append(args, status)
		}

		query += ` ORDER BY l.loan_date DESC`

		rows, err := h.db.Query(query, args...)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title":   "Error",
				"message": "Gagal mengambil data peminjaman",
			})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var loan models.DeviceLoan
			var assetCode, deviceName, computedStatus string
			err := rows.Scan(&loan.ID, &loan.DeviceID, &assetCode, &deviceName, &loan.BorrowerName,
				&loan.BorrowerType, &loan.LoanDate, &loan.ExpectedReturnDate, &loan.ActualReturnDate,
				&loan.Quantity, &loan.Status, &loan.Purpose, &computedStatus)
			if err != nil {
				continue
			}
			// Store asset code, device name, and computed status for display
			loan.DeviceAssetCode = assetCode
			loan.DeviceName = deviceName
			loan.ComputedStatus = computedStatus
			deviceLoans = append(deviceLoans, loan)
		}

	case "usages":
		// Pemakaian
		query := `SELECT u.id, u.device_id, d.asset_code, d.name, u.user_name, u.user_type, 
		                 u.usage_date, u.quantity, u.is_available, u.purpose
		          FROM device_usages u
		          JOIN devices d ON u.device_id = d.id
		          WHERE 1=1`
		args := []interface{}{}

		if search != "" {
			query += ` AND (u.user_name LIKE ? OR d.name LIKE ? OR d.asset_code LIKE ?)`
			searchTerm := "%" + search + "%"
			args = append(args, searchTerm, searchTerm, searchTerm)
		}

		query += ` ORDER BY u.usage_date DESC`

		rows, err := h.db.Query(query, args...)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title":   "Error",
				"message": "Gagal mengambil data pemakaian",
			})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var usage models.DeviceUsage
			var assetCode, deviceName string
			err := rows.Scan(&usage.ID, &usage.DeviceID, &assetCode, &deviceName, &usage.UserName,
				&usage.UserType, &usage.UsageDate, &usage.Quantity, &usage.IsAvailable, &usage.Purpose)
			if err != nil {
				continue
			}
			// Store asset code and device name for display
			usage.DeviceAssetCode = assetCode
			usage.DeviceName = deviceName
			deviceUsages = append(deviceUsages, usage)
		}
	}

	c.HTML(http.StatusOK, "device/list.html", gin.H{
		"title":        "Manajemen Perangkat - Sistem Inventaris Lab",
		"currentPage":  "devices",
		"username":     username,
		"role":         role,
		"devices":      devices,
		"deviceTypes":  deviceTypes,
		"deviceLoans":  deviceLoans,
		"deviceUsages": deviceUsages,
		"activeTab":    tab,
		"filters": gin.H{
			"search":     search,
			"category":   category,
			"item_type":  itemType,
			"condition":  condition,
			"status":     status,
			"sort_by":    sortBy,
			"sort_order": sortOrder,
		},
	})
}

// DeviceDetail renders device detail page with loan/usage tabs
func (h *Handler) DeviceDetail(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	id := c.Param("id")
	var device models.DeviceWithCategory

	err := h.db.QueryRow(`
		SELECT d.id, d.device_type_id, d.asset_code, d.name, dt.category, d.brand, d.model, d.serial_number,
		       d.item_type, d.is_loanable, d.is_consumable, d.quantity_total, d.quantity_available,
		       d.condition, d.location, d.purchase_date, d.notes, d.created_at, d.updated_at
		FROM devices d
		JOIN device_types dt ON d.device_type_id = dt.id
		WHERE d.id = ?
	`, id).Scan(&device.ID, &device.DeviceTypeID, &device.AssetCode, &device.Name, &device.Category,
		&device.Brand, &device.Model, &device.SerialNumber, &device.ItemType, &device.IsLoanable,
		&device.IsConsumable, &device.QuantityTotal, &device.QuantityAvailable, &device.Condition,
		&device.Location, &device.PurchaseDate, &device.Notes, &device.CreatedAt, &device.UpdatedAt)

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

	// Get device type name
	var deviceTypeName string
	h.db.QueryRow(`SELECT name FROM device_types WHERE id = ?`, device.DeviceTypeID).Scan(&deviceTypeName)

	// Get loans history
	loanRows, _ := h.db.Query(`
		SELECT id, borrower_name, loan_date, expected_return_date, actual_return_date, quantity, status
		FROM device_loans WHERE device_id = ? ORDER BY loan_date DESC LIMIT 10
	`, id)
	defer loanRows.Close()

	var loans []models.DeviceLoan
	for loanRows.Next() {
		var loan models.DeviceLoan
		loanRows.Scan(&loan.ID, &loan.BorrowerName, &loan.LoanDate, &loan.ExpectedReturnDate,
			&loan.ActualReturnDate, &loan.Quantity, &loan.Status)
		loans = append(loans, loan)
	}

	// Get usages history (only if consumable)
	var usages []models.DeviceUsage
	if device.IsConsumable {
		usageRows, _ := h.db.Query(`
			SELECT id, user_name, usage_date, quantity, purpose, is_available
			FROM device_usages WHERE device_id = ? ORDER BY usage_date DESC LIMIT 10
		`, id)
		defer usageRows.Close()

		for usageRows.Next() {
			var usage models.DeviceUsage
			usageRows.Scan(&usage.ID, &usage.UserName, &usage.UsageDate, &usage.Quantity, &usage.Purpose, &usage.IsAvailable)
			usages = append(usages, usage)
		}
	}

	c.HTML(http.StatusOK, "device/detail.html", gin.H{
		"title":          "Detail Perangkat - Sistem Inventaris Lab",
		"currentPage":    "devices",
		"username":       username,
		"role":           role,
		"device":         device,
		"deviceTypeName": deviceTypeName,
		"loans":          loans,
		"usages":         usages,
	})
}

// DeviceCreatePage renders device creation form
func (h *Handler) DeviceCreatePage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	deviceTypes := h.fetchDeviceTypes()

	c.HTML(http.StatusOK, "device/create.html", gin.H{
		"title":       "Tambah Perangkat Baru - Sistem Inventaris Lab",
		"currentPage": "devices",
		"username":    username,
		"role":        role,
		"deviceTypes": deviceTypes,
	})
}

// DeviceCreate handles device creation
func (h *Handler) DeviceCreate(c *gin.Context) {
	deviceTypeIDStr := c.PostForm("device_type_id")
	assetCode := c.PostForm("asset_code")
	name := c.PostForm("name")
	brand := c.PostForm("brand")
	model := c.PostForm("model")
	serialNumber := c.PostForm("serial_number")
	itemType := c.PostForm("item_type")
	itemMode := c.PostForm("item_mode")
	isLoanable := itemMode == "loanable"
	isConsumable := itemMode == "consumable"
	quantityTotalStr := c.PostForm("quantity_total")
	condition := c.PostForm("condition")
	location := c.PostForm("location")
	purchaseDate := c.PostForm("purchase_date")
	notes := c.PostForm("notes")

	// Validation: Required fields (device_type_id is now REQUIRED)
	if deviceTypeIDStr == "" || name == "" || itemType == "" {
		h.renderDeviceCreateWithError(c, "Template jenis barang, nama, dan tipe item harus diisi")
		return
	}

	// Parse device_type_id (REQUIRED)
	deviceTypeID, err := strconv.Atoi(deviceTypeIDStr)
	if err != nil {
		h.renderDeviceCreateWithError(c, "Template jenis barang tidak valid")
		return
	}

	// Validation: Item type whitelist
	if itemType != "individual" && itemType != "consumable" {
		h.renderDeviceCreateWithError(c, "Tipe item tidak valid")
		return
	}

	// Validation: Condition whitelist
	validConditions := map[string]bool{
		"baik":        true,
		"rusak":       true,
		"maintenance": true,
	}
	if condition != "" && !validConditions[condition] {
		h.renderDeviceCreateWithError(c, "Kondisi tidak valid")
		return
	}

	// Parse quantity with validation
	quantityTotal := 1
	quantityAvailable := 1

	if quantityTotalStr != "" {
		parsed, err := strconv.Atoi(quantityTotalStr)
		if err == nil && parsed > 0 {
			quantityTotal = parsed
		}
	}

	quantityAvailableStr := c.PostForm("quantity_available")
	if quantityAvailableStr != "" {
		parsed, err := strconv.Atoi(quantityAvailableStr)
		if err == nil && parsed >= 0 {
			quantityAvailable = parsed
		}
	}

	// Validation: Individual items must have quantity >= 1
	if itemType == "individual" && quantityTotal < 1 {
		quantityTotal = 1
		quantityAvailable = 1
	}

	// Validation: Available cannot exceed total
	if quantityAvailable > quantityTotal {
		quantityAvailable = quantityTotal
	}

	var purchaseDatePtr *string
	if purchaseDate != "" {
		purchaseDatePtr = &purchaseDate
	}

	result, err := h.db.Exec(`
		INSERT INTO devices (device_type_id, asset_code, name, brand, model, serial_number,
		                     item_type, is_loanable, is_consumable, quantity_total, quantity_available,
		                     condition, location, purchase_date, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, deviceTypeID, assetCode, name, brand, model, serialNumber, itemType, isLoanable,
		isConsumable, quantityTotal, quantityAvailable, condition, location, purchaseDatePtr, notes)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			h.renderDeviceCreateWithError(c, "Asset code sudah digunakan")
			return
		}

		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogAuth(
				userID, username, role, "create", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to create device: %v", err),
			)
		}
		h.renderDeviceCreateWithError(c, "Gagal menyimpan data perangkat")
		return
	}

	// Log successful create
	deviceID, _ := result.LastInsertId()
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		h.activityLogService.LogCreate(
			userID, username, role,
			"device", int(deviceID),
			map[string]interface{}{
				"name":       name,
				"asset_code": assetCode,
			},
			ipAddress, userAgent,
		)
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
	var device models.DeviceWithCategory
	var purchaseDateStr sql.NullString

	err := h.db.QueryRow(`
		SELECT d.id, d.device_type_id, d.asset_code, d.name, dt.category, d.brand, d.model, d.serial_number,
		       d.item_type, d.is_loanable, d.is_consumable, d.quantity_total, d.quantity_available,
		       d.condition, d.location, d.purchase_date, d.notes
		FROM devices d
		JOIN device_types dt ON d.device_type_id = dt.id
		WHERE d.id = ?
	`, id).Scan(&device.ID, &device.DeviceTypeID, &device.AssetCode, &device.Name, &device.Category,
		&device.Brand, &device.Model, &device.SerialNumber, &device.ItemType, &device.IsLoanable,
		&device.IsConsumable, &device.QuantityTotal, &device.QuantityAvailable, &device.Condition,
		&device.Location, &purchaseDateStr, &device.Notes)

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
		formats := []string{"2006-01-02", "2006-01-02T15:04:05Z", time.RFC3339}
		for _, format := range formats {
			if t, err := time.Parse(format, purchaseDateStr.String); err == nil {
				purchaseDateFormatted = t.Format("2006-01-02")
				break
			}
		}
		if purchaseDateFormatted == "" {
			purchaseDateFormatted = purchaseDateStr.String
		}
	}

	// Get device types
	rows, _ := h.db.Query(`SELECT id, name FROM device_types ORDER BY name`)
	defer rows.Close()

	var deviceTypes []models.DeviceType
	for rows.Next() {
		var dt models.DeviceType
		rows.Scan(&dt.ID, &dt.Name)
		deviceTypes = append(deviceTypes, dt)
	}

	c.HTML(http.StatusOK, "device/edit.html", gin.H{
		"title":        "Edit Perangkat - Sistem Inventaris Lab",
		"currentPage":  "devices",
		"username":     username,
		"role":         role,
		"device":       device,
		"purchaseDate": purchaseDateFormatted,
		"deviceTypes":  deviceTypes,
	})
}

// DeviceEdit handles device update
func (h *Handler) DeviceEdit(c *gin.Context) {
	id := c.Param("id")
	deviceTypeIDStr := c.PostForm("device_type_id")
	assetCode := c.PostForm("asset_code")
	name := c.PostForm("name")
	brand := c.PostForm("brand")
	model := c.PostForm("model")
	serialNumber := c.PostForm("serial_number")
	itemType := c.PostForm("item_type")
	itemMode := c.PostForm("item_mode")
	isLoanable := itemMode == "loanable"
	isConsumable := itemMode == "consumable"
	quantityTotalStr := c.PostForm("quantity_total")
	quantityAvailableStr := c.PostForm("quantity_available")
	condition := c.PostForm("condition")
	location := c.PostForm("location")
	purchaseDateForm := c.PostForm("purchase_date")
	notes := c.PostForm("notes")

	// Get old values for logging
	var oldName, oldAssetCode, oldCondition string
	var currentPurchaseDate sql.NullString
	err := h.db.QueryRow(`
		SELECT name, asset_code, condition, purchase_date FROM devices WHERE id = ?
	`, id).Scan(&oldName, &oldAssetCode, &oldCondition, &currentPurchaseDate)

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data perangkat",
		})
		return
	}

	// Parse device_type_id (REQUIRED)
	if deviceTypeIDStr == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "Error",
			"message": "Template jenis barang harus diisi",
		})
		return
	}
	deviceTypeID, _ := strconv.Atoi(deviceTypeIDStr)

	// Parse quantities
	quantityTotal, _ := strconv.Atoi(quantityTotalStr)
	quantityAvailable, _ := strconv.Atoi(quantityAvailableStr)

	// Preserve existing purchase_date if form field is empty
	var purchaseDatePtr *string
	if purchaseDateForm != "" {
		purchaseDatePtr = &purchaseDateForm
	} else if currentPurchaseDate.Valid {
		purchaseDatePtr = &currentPurchaseDate.String
	}

	_, err = h.db.Exec(`
		UPDATE devices 
		SET device_type_id = ?, asset_code = ?, name = ?, brand = ?, model = ?, serial_number = ?,
		    item_type = ?, is_loanable = ?, is_consumable = ?, quantity_total = ?, quantity_available = ?,
		    condition = ?, location = ?, purchase_date = ?, notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, deviceTypeID, assetCode, name, brand, model, serialNumber, itemType, isLoanable,
		isConsumable, quantityTotal, quantityAvailable, condition, location, purchaseDatePtr, notes, id)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"title":   "Error",
				"message": "Asset code sudah digunakan",
			})
			return
		}

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

		h.activityLogService.LogUpdate(
			userID, username, role,
			"device", deviceIDInt,
			map[string]interface{}{"name": oldName, "asset_code": oldAssetCode, "condition": oldCondition},
			map[string]interface{}{"name": name, "asset_code": assetCode, "condition": condition},
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
	var name, assetCode, condition string
	err := h.db.QueryRow(`
		SELECT id, name, asset_code, condition FROM devices WHERE id = ?
	`, id).Scan(&deviceID, &name, &assetCode, &condition)

	if err != nil {
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

		h.activityLogService.LogDelete(
			userID, username, role,
			"device", deviceID,
			map[string]interface{}{"name": name, "asset_code": assetCode, "condition": condition},
			ipAddress, userAgent,
		)
	}

	c.Redirect(http.StatusFound, "/devices")
}

// GetNextAssetCode API endpoint to get next available asset code
func (h *Handler) GetNextAssetCode(c *gin.Context) {
	prefix := c.Query("prefix")

	if prefix == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prefix required"})
		return
	}

	// Query last asset code with this prefix
	var lastCode string
	err := h.db.QueryRow(`
		SELECT asset_code FROM devices 
		WHERE asset_code LIKE ? 
		ORDER BY asset_code DESC LIMIT 1
	`, prefix+"-%").Scan(&lastCode)

	var nextNumber int
	if err == sql.ErrNoRows {
		nextNumber = 1
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	} else {
		// Extract number from "PENTAB-012" → 12
		parts := strings.Split(lastCode, "-")
		if len(parts) == 2 {
			num, _ := strconv.Atoi(parts[1])
			nextNumber = num + 1
		} else {
			nextNumber = 1
		}
	}

	nextCode := fmt.Sprintf("%s-%03d", prefix, nextNumber)

	c.JSON(http.StatusOK, gin.H{
		"next_code": nextCode,
		"number":    nextNumber,
	})
}

// DeviceExport exports all device management data to Excel (4 sheets in 1 file)
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

	// ===== SHEET 1: Daftar Perangkat =====
	devicesData := [][]interface{}{}
	devicesRows, err := h.db.Query(`
		SELECT d.asset_code, d.name, dt.category, d.brand, d.model, d.item_type, d.quantity_total, d.quantity_available,
		       d.condition, d.location, d.purchase_date, d.notes
		FROM devices d
		JOIN device_types dt ON d.device_type_id = dt.id
		ORDER BY dt.category, d.name
	`)
	if err == nil {
		defer devicesRows.Close()
		i := 1
		for devicesRows.Next() {
			var assetCode, name, category, brand, model, itemType, condition, location, notes string
			var quantityTotal, quantityAvailable int
			var purchaseDate sql.NullString
			
			devicesRows.Scan(&assetCode, &name, &category, &brand, &model, &itemType, &quantityTotal,
				&quantityAvailable, &condition, &location, &purchaseDate, &notes)
			
			purchaseDateStr := "-"
			if purchaseDate.Valid && purchaseDate.String != "" {
				if t, err := time.Parse("2006-01-02", purchaseDate.String); err == nil {
					purchaseDateStr = t.Format("02/01/2006")
				}
			}
			
			devicesData = append(devicesData, []interface{}{
				i, assetCode, name, category, brand, model, itemType,
				quantityTotal, quantityAvailable, condition, location, purchaseDateStr, notes,
			})
			i++
		}
	}

	// ===== SHEET 2: Jenis Barang =====
	typesData := [][]interface{}{}
	typesRows, err := h.db.Query(`
		SELECT name, category, brand, model, item_type, asset_code_prefix, default_location
		FROM device_types ORDER BY category, name
	`)
	if err == nil {
		defer typesRows.Close()
		i := 1
		for typesRows.Next() {
			var name, category, itemType string
			var brand, model, prefix, location sql.NullString
			
			typesRows.Scan(&name, &category, &brand, &model, &itemType, &prefix, &location)
			
			brandStr := ""
			if brand.Valid {
				brandStr = brand.String
			}
			modelStr := ""
			if model.Valid {
				modelStr = model.String
			}
			prefixStr := ""
			if prefix.Valid {
				prefixStr = prefix.String
			}
			locationStr := ""
			if location.Valid {
				locationStr = location.String
			}
			
			typesData = append(typesData, []interface{}{
				i, name, category, brandStr, modelStr, itemType, prefixStr, locationStr,
			})
			i++
		}
	}

	// ===== SHEET 3: Peminjaman =====
	loansData := [][]interface{}{}
	loansRows, err := h.db.Query(`
		SELECT d.asset_code, d.name, l.borrower_name, l.borrower_type, l.loan_date,
		       l.expected_return_date, l.actual_return_date, l.quantity, l.status, l.purpose
		FROM device_loans l
		JOIN devices d ON l.device_id = d.id
		ORDER BY l.loan_date DESC
	`)
	if err == nil {
		defer loansRows.Close()
		i := 1
		for loansRows.Next() {
			var assetCode, deviceName, borrowerName, borrowerType, status, purpose string
			var loanDate time.Time
			var expectedReturn, actualReturn sql.NullTime
			var quantity int
			
			loansRows.Scan(&assetCode, &deviceName, &borrowerName, &borrowerType, &loanDate,
				&expectedReturn, &actualReturn, &quantity, &status, &purpose)
			
			expectedReturnStr := "-"
			if expectedReturn.Valid {
				expectedReturnStr = expectedReturn.Time.Format("02/01/2006")
			}
			
			actualReturnStr := "-"
			if actualReturn.Valid {
				actualReturnStr = actualReturn.Time.Format("02/01/2006")
			}
			
			loansData = append(loansData, []interface{}{
				i, assetCode, deviceName, borrowerName, borrowerType,
				loanDate.Format("02/01/2006"), expectedReturnStr, actualReturnStr,
				quantity, status, purpose,
			})
			i++
		}
	}

	// ===== SHEET 4: Pemakaian =====
	usagesData := [][]interface{}{}
	usagesRows, err := h.db.Query(`
		SELECT d.asset_code, d.name, u.user_name, u.user_type, u.usage_date, u.quantity, u.purpose
		FROM device_usages u
		JOIN devices d ON u.device_id = d.id
		ORDER BY u.usage_date DESC
	`)
	if err == nil {
		defer usagesRows.Close()
		i := 1
		for usagesRows.Next() {
			var assetCode, deviceName, userName, userType, purpose string
			var usageDate time.Time
			var quantity int
			
			usagesRows.Scan(&assetCode, &deviceName, &userName, &userType, &usageDate, &quantity, &purpose)
			
			usagesData = append(usagesData, []interface{}{
				i, assetCode, deviceName, userName, userType,
				usageDate.Format("02/01/2006"), quantity, purpose,
			})
			i++
		}
	}

	// ===== Configure Multi-Sheet Excel =====
	excelService := services.NewExcelService()
	configs := []services.ExcelExportConfig{
		// Sheet 1: Daftar Perangkat
		{
			SheetName: "Daftar Perangkat",
			Headers: []string{
				"No", "Asset Code", "Nama", "Kategori", "Merk", "Model", "Tipe",
				"Qty Total", "Qty Tersedia", "Kondisi", "Lokasi", "Tgl Beli", "Catatan",
			},
			Data: devicesData,
			ColumnWidths: map[string]float64{
				"A": 5, "B": 15, "C": 25, "D": 15, "E": 15, "F": 15, "G": 12,
				"H": 10, "I": 10, "J": 12, "K": 20, "L": 12, "M": 30,
			},
		},
		// Sheet 2: Jenis Barang
		{
			SheetName: "Jenis Barang",
			Headers: []string{
				"No", "Nama", "Kategori", "Merk", "Model", "Tipe", "Prefix", "Lokasi Default",
			},
			Data: typesData,
			ColumnWidths: map[string]float64{
				"A": 5, "B": 25, "C": 15, "D": 15, "E": 15, "F": 12, "G": 12, "H": 20,
			},
		},
		// Sheet 3: Peminjaman
		{
			SheetName: "Peminjaman",
			Headers: []string{
				"No", "Asset Code", "Perangkat", "Peminjam", "Tipe", "Tgl Pinjam",
				"Target Kembali", "Tgl Kembali", "Jumlah", "Status", "Keperluan",
			},
			Data: loansData,
			ColumnWidths: map[string]float64{
				"A": 5, "B": 15, "C": 25, "D": 20, "E": 12, "F": 12,
				"G": 12, "H": 12, "I": 8, "J": 10, "K": 30,
			},
		},
		// Sheet 4: Pemakaian
		{
			SheetName: "Pemakaian",
			Headers: []string{
				"No", "Asset Code", "Perangkat", "Pengguna", "Tipe", "Tgl Pemakaian", "Jumlah", "Keperluan",
			},
			Data: usagesData,
			ColumnWidths: map[string]float64{
				"A": 5, "B": 15, "C": 25, "D": 20, "E": 12, "F": 12, "G": 8, "H": 30,
			},
		},
	}

	// Generate multi-sheet Excel file
	f, err := excelService.GenerateMultiSheetExcel(configs)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal generate file Excel: " + err.Error(),
		})
		return
	}
	defer f.Close()

	filename := excelService.GenerateFilename("devices_export")

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Transfer-Encoding", "binary")

	if err := f.Write(c.Writer); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal generate file Excel",
		})
		return
	}
}
