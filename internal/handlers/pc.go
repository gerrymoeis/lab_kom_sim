package handlers

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// PCList renders list of all PCs
func (h *Handler) PCList(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	rows, err := h.db.Query(`
		SELECT id, pc_number, "row", "column", status, processor, ram, storage, 
		       purchase_date, notes, last_checked, 
		       asset_id, serial_number, brand, model, operating_system, physical_condition,
		       device_type, brand_model, accessories, action_notes, photo_serial, photo_front,
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
		var processor, ram, storage, notes sql.NullString
		var assetID, serialNumber, brand, model, operatingSystem, physicalCondition sql.NullString
		var deviceType, brandModel, accessories, actionNotes, photoSerial, photoFront sql.NullString
		var purchaseDate, lastChecked sql.NullTime
		
		err := rows.Scan(&pc.ID, &pc.PCNumber, &pc.Row, &pc.Column, &pc.Status,
			&processor, &ram, &storage, &purchaseDate, &notes,
			&lastChecked, 
			&assetID, &serialNumber, &brand, &model, &operatingSystem, &physicalCondition,
			&deviceType, &brandModel, &accessories, &actionNotes, &photoSerial, &photoFront,
			&pc.CreatedAt, &pc.UpdatedAt)
		if err != nil {
			continue
		}
		
		// Convert NullString to string
		if processor.Valid {
			pc.Processor = processor.String
		}
		if ram.Valid {
			pc.RAM = ram.String
		}
		if storage.Valid {
			pc.Storage = storage.String
		}
		if notes.Valid {
			pc.Notes = notes.String
		}
		if assetID.Valid {
			pc.AssetID = assetID.String
		}
		if serialNumber.Valid {
			pc.SerialNumber = serialNumber.String
		}
		if brand.Valid {
			pc.Brand = brand.String
		}
		if model.Valid {
			pc.Model = model.String
		}
		if operatingSystem.Valid {
			pc.OperatingSystem = operatingSystem.String
		}
		if physicalCondition.Valid {
			pc.PhysicalCondition = physicalCondition.String
		}
		if deviceType.Valid {
			pc.DeviceType = deviceType.String
		}
		if brandModel.Valid {
			pc.BrandModel = brandModel.String
		}
		if accessories.Valid {
			pc.Accessories = accessories.String
		}
		if actionNotes.Valid {
			pc.ActionNotes = actionNotes.String
		}
		if photoSerial.Valid {
			pc.PhotoSerial = photoSerial.String
		}
		if photoFront.Valid {
			pc.PhotoFront = photoFront.String
		}
		if purchaseDate.Valid {
			pc.PurchaseDate = &purchaseDate.Time
		}
		if lastChecked.Valid {
			pc.LastChecked = &lastChecked.Time
		}
		
		pcs = append(pcs, pc)
	}

	c.HTML(http.StatusOK, "pc/list.html", gin.H{
		"title":       "Daftar PC - Sistem Inventaris Lab",
		"username":    username,
		"role":        role,
		"currentPage": "pc",
		"pcs":         pcs,
	})
}

// PCDetail shows detail of a PC
func (h *Handler) PCDetail(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	pcNumber := c.Param("pc_number")
	var pc models.PC
	
	// Use sql.NullString for nullable fields
	var processor, ram, storage, assetID, serialNumber, brand, model, operatingSystem, physicalCondition, notes sql.NullString
	var deviceType, brandModel, accessories, actionNotes, photoSerial, photoFront sql.NullString
	var purchaseDate, lastChecked sql.NullTime
	
	err := h.db.QueryRow(`
		SELECT id, pc_number, "row", "column", status, processor, ram, storage,
		       purchase_date, notes, last_checked,
		       asset_id, serial_number, brand, model, operating_system, physical_condition,
		       device_type, brand_model, accessories, action_notes, photo_serial, photo_front,
		       created_at, updated_at
		FROM pcs WHERE pc_number = ?
	`, pcNumber).Scan(&pc.ID, &pc.PCNumber, &pc.Row, &pc.Column, &pc.Status,
		&processor, &ram, &storage, &purchaseDate, &notes,
		&lastChecked,
		&assetID, &serialNumber, &brand, &model, &operatingSystem, &physicalCondition,
		&deviceType, &brandModel, &accessories, &actionNotes, &photoSerial, &photoFront,
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
			"message": "Gagal mengambil data PC: " + err.Error(),
		})
		return
	}
	
	// Convert NullString to string
	if processor.Valid {
		pc.Processor = processor.String
	}
	if ram.Valid {
		pc.RAM = ram.String
	}
	if storage.Valid {
		pc.Storage = storage.String
	}
	if assetID.Valid {
		pc.AssetID = assetID.String
	}
	if serialNumber.Valid {
		pc.SerialNumber = serialNumber.String
	}
	if brand.Valid {
		pc.Brand = brand.String
	}
	if model.Valid {
		pc.Model = model.String
	}
	if operatingSystem.Valid {
		pc.OperatingSystem = operatingSystem.String
	}
	if physicalCondition.Valid {
		pc.PhysicalCondition = physicalCondition.String
	}
	if notes.Valid {
		pc.Notes = notes.String
	}
	if deviceType.Valid {
		pc.DeviceType = deviceType.String
	}
	if brandModel.Valid {
		pc.BrandModel = brandModel.String
	}
	if accessories.Valid {
		pc.Accessories = accessories.String
	}
	if actionNotes.Valid {
		pc.ActionNotes = actionNotes.String
	}
	if photoSerial.Valid {
		pc.PhotoSerial = photoSerial.String
	}
	if photoFront.Valid {
		pc.PhotoFront = photoFront.String
	}
	if purchaseDate.Valid {
		pc.PurchaseDate = &purchaseDate.Time
	}
	if lastChecked.Valid {
		pc.LastChecked = &lastChecked.Time
	}

	// Get software for this PC (via junction)
	swRows, err := h.db.Query(`
		SELECT sc.id, sc.name, sc.category, COALESCE(ps.installed, FALSE), sc.description
		FROM software_catalog sc
		LEFT JOIN pc_software ps ON sc.id = ps.software_id AND ps.pc_id = ?
		ORDER BY sc.category, sc.name
	`, pc.ID)

	var pcSoftware []models.PCSoftware
	if err == nil {
		defer swRows.Close()
		for swRows.Next() {
		var catID int
		var name, category, description string
		var installed bool

		if err := swRows.Scan(&catID, &name, &category, &installed, &description); err == nil {
			pcSoftware = append(pcSoftware, models.PCSoftware{
				PCID:         pc.ID,
				SoftwareID:   catID,
				Installed:    installed,
				SoftwareName: name,
				Category:     category,
				Description:  description,
			})
			}
		}
	}

	// Separate required and other
	var requiredSW, otherSW []models.PCSoftware
	for _, sw := range pcSoftware {
		if sw.Category == "required" {
			requiredSW = append(requiredSW, sw)
		} else {
			otherSW = append(otherSW, sw)
		}
	}

	// Format lastChecked for display
	var lastCheckedFormatted string
	if pc.LastChecked != nil {
		lastCheckedFormatted = pc.LastChecked.Format("02/01/2006 15:04")
	}

	c.HTML(http.StatusOK, "pc/detail.html", gin.H{
		"title":                 "Detail PC - Sistem Inventaris Lab",
		"username":              username,
		"role":                  role,
		"currentPage":           "pc",
		"pc":                    pc,
		"requiredSW":            requiredSW,
		"otherSW":               otherSW,
		"lastCheckedFormatted":  lastCheckedFormatted,
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
		"title":       "Tambah PC Baru - Sistem Inventaris Lab",
		"username":    username,
		"role":        role,
		"currentPage": "pc",
	})
}

// PCCreate handles PC creation
func (h *Handler) PCCreate(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Parse form data
	pcNumber, _ := strconv.Atoi(c.PostForm("pc_number"))
	row, _ := strconv.Atoi(c.PostForm("row"))
	column, _ := strconv.Atoi(c.PostForm("column"))
	status := c.PostForm("status")
	
	// New fields
	deviceType := c.PostForm("device_type")
	serialNumber := c.PostForm("serial_number")
	brandModel := c.PostForm("brand_model")
	accessories := c.PostForm("accessories")
	
	// Specs
	processor := c.PostForm("processor")
	ram := c.PostForm("ram")
	storage := c.PostForm("storage")
	operatingSystem := c.PostForm("operating_system")
	
	// Additional info
	purchaseDate := c.PostForm("purchase_date")
	lastChecked := c.PostForm("last_checked")
	notes := c.PostForm("notes")
	actionNotes := c.PostForm("action_notes")

	// Validate
	if pcNumber < 1 || pcNumber > 40 {
		c.HTML(http.StatusBadRequest, "pc/create.html", gin.H{
			"title":       "Tambah PC Baru - Sistem Inventaris Lab",
			"username":    username,
			"role":        role,
			"currentPage": "pc",
			"error":       "Nomor PC harus antara 1-40",
		})
		return
	}

	if row < 1 || row > 5 || column < 1 || column > 8 {
		c.HTML(http.StatusBadRequest, "pc/create.html", gin.H{
			"title":       "Tambah PC Baru - Sistem Inventaris Lab",
			"username":    username,
			"role":        role,
			"currentPage": "pc",
			"error":       "Posisi baris (1-5) dan kolom (1-8) tidak valid",
		})
		return
	}

	// Validate required fields
	if serialNumber == "" {
		c.HTML(http.StatusBadRequest, "pc/create.html", gin.H{
			"title":       "Tambah PC Baru - Sistem Inventaris Lab",
			"username":    username,
			"role":        role,
			"currentPage": "pc",
			"error":       "Serial Number wajib diisi",
		})
		return
	}

	if operatingSystem == "" {
		c.HTML(http.StatusBadRequest, "pc/create.html", gin.H{
			"title":       "Tambah PC Baru - Sistem Inventaris Lab",
			"username":    username,
			"role":        role,
			"currentPage": "pc",
			"error":       "Sistem Operasi wajib diisi",
		})
		return
	}

	var purchaseDatePtr *string
	if purchaseDate != "" {
		purchaseDatePtr = &purchaseDate
	}

	var lastCheckedPtr *string
	if lastChecked != "" {
		// Convert datetime-local format (2006-01-02T15:04) to ISO 8601 for database
		if t, err := time.Parse("2006-01-02T15:04", lastChecked); err == nil {
			formatted := t.Format(time.RFC3339)
			lastCheckedPtr = &formatted
		} else {
			// Fallback: use as-is if parsing fails
			lastCheckedPtr = &lastChecked
		}
	}

	// Handle file uploads using file references (no processing needed)
	var photoSerialFilename, photoFrontFilename string

	// Get file references from form (uploaded via /api/upload-image)
	serialFileRef := c.PostForm("serial_file_ref")
	frontFileRef := c.PostForm("front_file_ref")

	// Move files from temp to final location (no processing needed)
	if serialFileRef != "" {
		tempPath := filepath.Join("uploads", "temp", serialFileRef)
		finalPath := filepath.Join("uploads", "pc", serialFileRef)
		
		// Ensure pc directory exists
		pcDir := filepath.Join("uploads", "pc")
		if err := os.MkdirAll(pcDir, 0755); err != nil {
			c.HTML(http.StatusInternalServerError, "pc/create.html", gin.H{
				"title":       "Tambah PC Baru - Sistem Inventaris Lab",
				"username":    username,
				"role":        role,
				"currentPage": "pc",
				"error":       "Gagal membuat direktori upload",
			})
			return
		}
		
		if err := os.Rename(tempPath, finalPath); err == nil {
			photoSerialFilename = serialFileRef
		} else {
			// If rename fails, try copy and delete
			if err := copyFile(tempPath, finalPath); err == nil {
				os.Remove(tempPath)
				photoSerialFilename = serialFileRef
			}
		}
	}

	if frontFileRef != "" {
		tempPath := filepath.Join("uploads", "temp", frontFileRef)
		finalPath := filepath.Join("uploads", "pc", frontFileRef)
		
		// Ensure pc directory exists
		pcDir := filepath.Join("uploads", "pc")
		if err := os.MkdirAll(pcDir, 0755); err != nil {
			c.HTML(http.StatusInternalServerError, "pc/create.html", gin.H{
				"title":       "Tambah PC Baru - Sistem Inventaris Lab",
				"username":    username,
				"role":        role,
				"currentPage": "pc",
				"error":       "Gagal membuat direktori upload",
			})
			return
		}
		
		if err := os.Rename(tempPath, finalPath); err == nil {
			photoFrontFilename = frontFileRef
		} else {
			// If rename fails, try copy and delete
			if err := copyFile(tempPath, finalPath); err == nil {
				os.Remove(tempPath)
				photoFrontFilename = frontFileRef
			}
		}
	}

	// Cleanup any remaining temp files for this session
	defer func() {
		if serialFileRef != "" {
			tempPath := filepath.Join("uploads", "temp", serialFileRef)
			os.Remove(tempPath) // Remove if still exists
		}
		if frontFileRef != "" {
			tempPath := filepath.Join("uploads", "temp", frontFileRef)
			os.Remove(tempPath) // Remove if still exists
		}
	}()

	// Insert to database
	_, err := h.db.Exec(`
		INSERT INTO pcs (
			pc_number, "row", "column", status, 
			device_type, serial_number, brand_model, accessories,
			processor, ram, storage, operating_system, 
			purchase_date, last_checked, notes, action_notes,
			photo_serial, photo_front
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, pcNumber, row, column, status,
		deviceType, serialNumber, brandModel, accessories,
		processor, ram, storage, operatingSystem,
		purchaseDatePtr, lastCheckedPtr, notes, actionNotes,
		photoSerialFilename, photoFrontFilename)

	if err != nil {
		// Cleanup uploaded photos on database error
		if photoSerialFilename != "" {
			os.Remove(filepath.Join("uploads", "pc", photoSerialFilename))
		}
		if photoFrontFilename != "" {
			os.Remove(filepath.Join("uploads", "pc", photoFrontFilename))
		}
		
		// Log failed create
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogAuth(
				userID, username, role, "create", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to create PC #%d: %v", pcNumber, err),
			)
		}
		
		c.HTML(http.StatusInternalServerError, "pc/create.html", gin.H{
			"title":       "Tambah PC Baru - Sistem Inventaris Lab",
			"username":    username,
			"role":        role,
			"currentPage": "pc",
			"error":       "Gagal menyimpan data PC. Mungkin nomor PC sudah digunakan.",
		})
		return
	}

	// Get the created PC ID
	var pcID int
	err = h.db.QueryRow("SELECT id FROM pcs WHERE pc_number = ?", pcNumber).Scan(&pcID)
	if err == nil {
		// Log successful create
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogCreate(
				userID, username, role,
				"pc", pcID,
				map[string]interface{}{
					"pc_number":     pcNumber,
					"status":        status,
					"serial_number": serialNumber,
					"brand_model":   brandModel,
					"device_type":   deviceType,
				},
				ipAddress, userAgent,
			)
			
			// Log photo uploads separately (if uploaded)
			if photoSerialFilename != "" {
				h.activityLogService.LogUpload(
					userID, username, role,
					"pc", pcID,
					photoSerialFilename, "photo_serial",
					ipAddress, userAgent,
				)
			}
			
			if photoFrontFilename != "" {
				h.activityLogService.LogUpload(
					userID, username, role,
					"pc", pcID,
					photoFrontFilename, "photo_front",
					ipAddress, userAgent,
				)
			}
		}
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

	pcNumber := c.Param("pc_number")
	var pc models.PC
	var purchaseDateStr, lastCheckedStr sql.NullString
	var processor, ram, storage, operatingSystem, notes sql.NullString
	var deviceType, serialNumber, brandModel, accessories, actionNotes, photoSerial, photoFront sql.NullString

	err := h.db.QueryRow(`
		SELECT id, pc_number, "row", "column", status, processor, ram, storage,
		       purchase_date, last_checked, operating_system, notes,
		       device_type, serial_number, brand_model, accessories, action_notes,
		       photo_serial, photo_front
		FROM pcs WHERE pc_number = ?
	`, pcNumber).Scan(&pc.ID, &pc.PCNumber, &pc.Row, &pc.Column, &pc.Status,
		&processor, &ram, &storage, &purchaseDateStr, &lastCheckedStr, &operatingSystem, &notes,
		&deviceType, &serialNumber, &brandModel, &accessories, &actionNotes,
		&photoSerial, &photoFront)

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
	
	// Convert NullString to string
	if processor.Valid {
		pc.Processor = processor.String
	}
	if ram.Valid {
		pc.RAM = ram.String
	}
	if storage.Valid {
		pc.Storage = storage.String
	}
	if operatingSystem.Valid {
		pc.OperatingSystem = operatingSystem.String
	}
	if notes.Valid {
		pc.Notes = notes.String
	}
	if deviceType.Valid {
		pc.DeviceType = deviceType.String
	}
	if serialNumber.Valid {
		pc.SerialNumber = serialNumber.String
	}
	if brandModel.Valid {
		pc.BrandModel = brandModel.String
	}
	if accessories.Valid {
		pc.Accessories = accessories.String
	}
	if actionNotes.Valid {
		pc.ActionNotes = actionNotes.String
	}
	if photoSerial.Valid {
		pc.PhotoSerial = photoSerial.String
	}
	if photoFront.Valid {
		pc.PhotoFront = photoFront.String
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
	}

	var lastCheckedFormatted string
	var lastCheckedDisplay string
	if lastCheckedStr.Valid {
		// Try multiple datetime formats
		formats := []string{"2006-01-02T15:04:05Z", time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04"}
		var parsedTime time.Time
		var parseSuccess bool
		for _, format := range formats {
			if t, err := time.Parse(format, lastCheckedStr.String); err == nil {
				parsedTime = t
				parseSuccess = true
				break
			}
		}
		if parseSuccess {
			// Format for datetime-local input (YYYY-MM-DDTHH:MM)
			lastCheckedFormatted = parsedTime.Format("2006-01-02T15:04")
			// Format for display (DD/MM/YYYY HH:MM)
			lastCheckedDisplay = parsedTime.Format("02/01/2006 15:04")
		}
	}

	// Get software catalog with installed status for this PC
	swRows, err := h.db.Query(`
		SELECT sc.id, sc.name, sc.category, COALESCE(ps.installed, FALSE), sc.description
		FROM software_catalog sc
		LEFT JOIN pc_software ps ON sc.id = ps.software_id AND ps.pc_id = ?
		ORDER BY sc.category, sc.name
	`, pc.ID)

	var requiredSW, otherSW []models.PCSoftware
	if err == nil {
		defer swRows.Close()
		for swRows.Next() {
			var catID int
			var name, category, description string
			var installed bool
			if err := swRows.Scan(&catID, &name, &category, &installed, &description); err == nil {
				sw := models.PCSoftware{
					PCID: pc.ID, SoftwareID: catID, Installed: installed,
					SoftwareName: name, Category: category, Description: description,
				}
				if category == "required" {
					requiredSW = append(requiredSW, sw)
				} else {
					otherSW = append(otherSW, sw)
				}
			}
		}
	}

	c.HTML(http.StatusOK, "pc/edit.html", gin.H{
		"title":              "Edit PC - Sistem Inventaris Lab",
		"username":           username,
		"role":               role,
		"currentPage":        "pc",
		"pc":                 pc,
		"purchaseDate":       purchaseDateFormatted,
		"lastChecked":        lastCheckedFormatted,
		"lastCheckedDisplay": lastCheckedDisplay,
		"requiredSW":         requiredSW,
		"otherSW":            otherSW,
	})
}

// PCEdit handles PC update
func (h *Handler) PCEdit(c *gin.Context) {
	pcNumber := c.Param("pc_number")
	status := c.PostForm("status")
	
	// New fields
	deviceType := c.PostForm("device_type")
	serialNumber := c.PostForm("serial_number")
	brandModel := c.PostForm("brand_model")
	accessories := c.PostForm("accessories")
	
	// Specs
	processor := c.PostForm("processor")
	ram := c.PostForm("ram")
	storage := c.PostForm("storage")
	operatingSystem := c.PostForm("operating_system")
	
	// Additional info
	purchaseDateForm := c.PostForm("purchase_date")
	lastCheckedForm := c.PostForm("last_checked")
	notes := c.PostForm("notes")
	actionNotes := c.PostForm("action_notes")

	// Validate required fields
	if serialNumber == "" || operatingSystem == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "Error",
			"message": "Serial Number dan Sistem Operasi wajib diisi",
		})
		return
	}

	// Get current PC data to retrieve existing photos, date values, AND old values for logging
	var currentPhotoSerial, currentPhotoFront sql.NullString
	var currentPurchaseDate, currentLastChecked sql.NullString
	var currentPCNumber, pcID int
	var oldStatus, oldDeviceType, oldSerialNumber, oldBrandModel sql.NullString
	err := h.db.QueryRow(`
		SELECT id, pc_number, status, device_type, serial_number, brand_model, 
		       photo_serial, photo_front, purchase_date, last_checked
		FROM pcs WHERE pc_number = ?
	`, pcNumber).Scan(&pcID, &currentPCNumber, &oldStatus, &oldDeviceType, &oldSerialNumber, &oldBrandModel, 
		&currentPhotoSerial, &currentPhotoFront, &currentPurchaseDate, &currentLastChecked)
	if err != nil {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogAuth(
				userID, username, role, "update", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to get PC data for update: %v", err),
			)
		}
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data PC",
		})
		return
	}

	// Preserve existing date values if form fields are empty
	var purchaseDatePtr *string
	if purchaseDateForm != "" {
		// User provided new value
		purchaseDatePtr = &purchaseDateForm
	} else if currentPurchaseDate.Valid {
		// Preserve existing value
		purchaseDatePtr = &currentPurchaseDate.String
	}
	// If both empty, purchaseDatePtr = nil (set to NULL)

	var lastCheckedPtr *string
	if lastCheckedForm != "" {
		// User provided new value from datetime-local input (format: 2006-01-02T15:04)
		// Convert to ISO 8601 format for database
		if t, err := time.Parse("2006-01-02T15:04", lastCheckedForm); err == nil {
			formatted := t.Format(time.RFC3339)
			lastCheckedPtr = &formatted
		} else {
			// Fallback: use as-is if parsing fails
			lastCheckedPtr = &lastCheckedForm
		}
	} else if currentLastChecked.Valid {
		// Preserve existing value
		lastCheckedPtr = &currentLastChecked.String
	}
	// If both empty, lastCheckedPtr = nil (set to NULL)

	// Handle file uploads using file references (no processing needed)
	photoSerialFilename := ""
	if currentPhotoSerial.Valid {
		photoSerialFilename = currentPhotoSerial.String
	}
	
	photoFrontFilename := ""
	if currentPhotoFront.Valid {
		photoFrontFilename = currentPhotoFront.String
	}

	imageService := services.NewImageService()

	// Get file references from form (uploaded via /api/upload-image)
	serialFileRef := c.PostForm("serial_file_ref")
	frontFileRef := c.PostForm("front_file_ref")

	// Handle serial photo update
	if serialFileRef != "" {
		// Delete old photo if exists
		if photoSerialFilename != "" {
			oldPath := filepath.Join("uploads", "pc", photoSerialFilename)
			imageService.DeleteImage(oldPath)
		}

		// Move file from temp to final location
		tempPath := filepath.Join("uploads", "temp", serialFileRef)
		finalPath := filepath.Join("uploads", "pc", serialFileRef)
		
		// Ensure pc directory exists
		pcDir := filepath.Join("uploads", "pc")
		if err := os.MkdirAll(pcDir, 0755); err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title":   "Error",
				"message": "Gagal membuat direktori upload",
			})
			return
		}
		
		if err := os.Rename(tempPath, finalPath); err == nil {
			photoSerialFilename = serialFileRef
		} else {
			// If rename fails, try copy and delete
			if err := copyFile(tempPath, finalPath); err == nil {
				os.Remove(tempPath)
				photoSerialFilename = serialFileRef
			}
		}
	}

	// Handle front photo update
	if frontFileRef != "" {
		// Delete old photo if exists
		if photoFrontFilename != "" {
			oldPath := filepath.Join("uploads", "pc", photoFrontFilename)
			imageService.DeleteImage(oldPath)
		}

		// Move file from temp to final location
		tempPath := filepath.Join("uploads", "temp", frontFileRef)
		finalPath := filepath.Join("uploads", "pc", frontFileRef)
		
		// Ensure pc directory exists
		pcDir := filepath.Join("uploads", "pc")
		if err := os.MkdirAll(pcDir, 0755); err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title":   "Error",
				"message": "Gagal membuat direktori upload",
			})
			return
		}
		
		if err := os.Rename(tempPath, finalPath); err == nil {
			photoFrontFilename = frontFileRef
		} else {
			// If rename fails, try copy and delete
			if err := copyFile(tempPath, finalPath); err == nil {
				os.Remove(tempPath)
				photoFrontFilename = frontFileRef
			}
		}
	}

	// Cleanup any remaining temp files for this session
	defer func() {
		if serialFileRef != "" {
			tempPath := filepath.Join("uploads", "temp", serialFileRef)
			os.Remove(tempPath) // Remove if still exists
		}
		if frontFileRef != "" {
			tempPath := filepath.Join("uploads", "temp", frontFileRef)
			os.Remove(tempPath) // Remove if still exists
		}
	}()

	// Update database
	_, err = h.db.Exec(`
		UPDATE pcs 
		SET status = ?, 
		    device_type = ?, serial_number = ?, brand_model = ?, accessories = ?,
		    processor = ?, ram = ?, storage = ?, operating_system = ?,
		    purchase_date = ?, last_checked = ?, notes = ?, action_notes = ?,
		    photo_serial = ?, photo_front = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE pc_number = ?
	`, status,
		deviceType, serialNumber, brandModel, accessories,
		processor, ram, storage, operatingSystem,
		purchaseDatePtr, lastCheckedPtr, notes, actionNotes,
		photoSerialFilename, photoFrontFilename,
		pcNumber)

	if err != nil {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogAuth(
				userID, username, role, "update", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to update PC #%d: %v", currentPCNumber, err),
			)
		}
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengupdate data PC",
		})
		return
	}

	// Sync software assignments (all or nothing via transaction)
	requiredIDs := c.PostFormArray("required_sw[]")
	otherNames := c.PostFormArray("other_name[]")
	otherDescs := c.PostFormArray("other_desc[]")

	if err := syncPCSoftware(h.db, pcID, requiredIDs, otherNames, otherDescs); err != nil {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogAuth(
				userID, username, role, "update", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to sync software for PC #%d: %v", pcID, err),
			)
		}
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal menyimpan software PC",
		})
		return
	}

	// Log successful update
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		
		// Prepare old and new values
		oldValues := map[string]interface{}{}
		if oldStatus.Valid {
			oldValues["status"] = oldStatus.String
		}
		if oldDeviceType.Valid {
			oldValues["device_type"] = oldDeviceType.String
		}
		if oldSerialNumber.Valid {
			oldValues["serial_number"] = oldSerialNumber.String
		}
		if oldBrandModel.Valid {
			oldValues["brand_model"] = oldBrandModel.String
		}
		
		newValues := map[string]interface{}{
			"status":        status,
			"device_type":   deviceType,
			"serial_number": serialNumber,
			"brand_model":   brandModel,
		}
		
		h.activityLogService.LogUpdate(
			userID, username, role,
			"pc", pcID,
			oldValues,
			newValues,
			ipAddress, userAgent,
		)
		
		// Log photo uploads separately (if new photos uploaded)
		if serialFileRef != "" {
			h.activityLogService.LogUpload(
				userID, username, role,
				"pc", pcID,
				photoSerialFilename, "photo_serial",
				ipAddress, userAgent,
			)
		}
		
		if frontFileRef != "" {
			h.activityLogService.LogUpload(
				userID, username, role,
				"pc", pcID,
				photoFrontFilename, "photo_front",
				ipAddress, userAgent,
			)
		}
	}

	c.Redirect(http.StatusFound, "/pc/"+pcNumber)
}

// PCDelete handles PC deletion
func (h *Handler) PCDelete(c *gin.Context) {
	pcNumber := c.Param("pc_number")

	// Get PC data before deleting (for logging)
	var pcID, pcNumberInt int
	var status, deviceType, serialNumber, brandModel sql.NullString
	var photoSerial, photoFront sql.NullString
	err := h.db.QueryRow(`
		SELECT id, pc_number, status, device_type, serial_number, brand_model, photo_serial, photo_front 
		FROM pcs WHERE pc_number = ?
	`, pcNumber).Scan(&pcID, &pcNumberInt, &status, &deviceType, &serialNumber, &brandModel, &photoSerial, &photoFront)
	
	if err != nil && err != sql.ErrNoRows {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogAuth(
				userID, username, role, "delete", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to get PC data for delete: %v", err),
			)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengambil data PC",
		})
		return
	}

	// Delete PC from database
	_, err = h.db.Exec("DELETE FROM pcs WHERE pc_number = ?", pcNumber)
	if err != nil {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogAuth(
				userID, username, role, "delete", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to delete PC #%d: %v", pcNumberInt, err),
			)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menghapus PC",
		})
		return
	}

	// Log successful delete
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		
		oldValues := map[string]interface{}{
			"pc_number": pcNumberInt,
		}
		if status.Valid {
			oldValues["status"] = status.String
		}
		if deviceType.Valid {
			oldValues["device_type"] = deviceType.String
		}
		if serialNumber.Valid {
			oldValues["serial_number"] = serialNumber.String
		}
		if brandModel.Valid {
			oldValues["brand_model"] = brandModel.String
		}
		
		h.activityLogService.LogDelete(
			userID, username, role,
			"pc", pcID,
			oldValues,
			ipAddress, userAgent,
		)
	}

	// Delete photos if exist
	imageService := services.NewImageService()
	if photoSerial.Valid && photoSerial.String != "" {
		photoPath := filepath.Join("uploads", "pc", photoSerial.String)
		imageService.DeleteImage(photoPath)
	}
	if photoFront.Valid && photoFront.String != "" {
		photoPath := filepath.Join("uploads", "pc", photoFront.String)
		imageService.DeleteImage(photoPath)
	}

	c.Redirect(http.StatusFound, "/pc")
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

// PCExport exports PC list to Excel
func (h *Handler) PCExport(c *gin.Context) {
	_, _, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	if role != "admin" {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"title":   "Akses Ditolak",
			"message": "Hanya admin yang dapat export data PC",
		})
		return
	}

	// Query all PCs
	rows, err := h.db.Query(`
		SELECT pc_number, "row", "column", status, device_type, serial_number, brand_model,
		       processor, ram, storage, operating_system, accessories,
		       purchase_date, last_checked, notes, action_notes
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

	// Collect data
	type PCExportData struct {
		PCNumber        int
		Row             int
		Column          int
		Status          string
		DeviceType      sql.NullString
		SerialNumber    sql.NullString
		BrandModel      sql.NullString
		Processor       sql.NullString
		RAM             sql.NullString
		Storage         sql.NullString
		OperatingSystem sql.NullString
		Accessories     sql.NullString
		PurchaseDate    sql.NullString
		LastChecked     sql.NullString
		Notes           sql.NullString
		ActionNotes     sql.NullString
	}

	var pcs []PCExportData
	for rows.Next() {
		var pc PCExportData
		err := rows.Scan(&pc.PCNumber, &pc.Row, &pc.Column, &pc.Status,
			&pc.DeviceType, &pc.SerialNumber, &pc.BrandModel,
			&pc.Processor, &pc.RAM, &pc.Storage, &pc.OperatingSystem, &pc.Accessories,
			&pc.PurchaseDate, &pc.LastChecked, &pc.Notes, &pc.ActionNotes)
		if err != nil {
			continue
		}
		pcs = append(pcs, pc)
	}

	// Transform data to [][]interface{}
	data := [][]interface{}{}
	for i, pc := range pcs {
		// Format position as "Baris X - Kolom Y"
		position := fmt.Sprintf("Baris %d - Kolom %d", pc.Row, pc.Column)

		// Format purchase date
		purchaseDate := "-"
		if pc.PurchaseDate.Valid && pc.PurchaseDate.String != "" {
			if t, err := time.Parse("2006-01-02", pc.PurchaseDate.String); err == nil {
				purchaseDate = t.Format("02/01/2006")
			} else {
				purchaseDate = pc.PurchaseDate.String
			}
		}

		// Format last checked datetime
		lastChecked := "-"
		if pc.LastChecked.Valid && pc.LastChecked.String != "" {
			formats := []string{"2006-01-02T15:04:05Z", time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04"}
			for _, format := range formats {
				if t, err := time.Parse(format, pc.LastChecked.String); err == nil {
					lastChecked = t.Format("02/01/2006 15:04")
					break
				}
			}
			if lastChecked == "-" {
				lastChecked = pc.LastChecked.String
			}
		}

		// Helper function to get string value or "-"
		getValue := func(ns sql.NullString) string {
			if ns.Valid && ns.String != "" {
				return ns.String
			}
			return "-"
		}

		row := []interface{}{
			i + 1,                           // No
			fmt.Sprintf("PC-%02d", pc.PCNumber), // PC Number
			position,                        // Posisi
			pc.Status,                       // Status
			getValue(pc.DeviceType),         // Device Type
			getValue(pc.SerialNumber),       // Serial Number
			getValue(pc.BrandModel),         // Brand & Model
			getValue(pc.Processor),          // Processor
			getValue(pc.RAM),                // RAM
			getValue(pc.Storage),            // Storage
			getValue(pc.OperatingSystem),    // OS
			getValue(pc.Accessories),        // Accessories
			purchaseDate,                    // Purchase Date
			lastChecked,                     // Last Checked
			getValue(pc.Notes),              // Notes
			getValue(pc.ActionNotes),        // Action Notes
		}
		data = append(data, row)
	}

	// Prepare conditional formatting for status column (column D, index 3)
	conditionalFormats := []services.ConditionalFormat{}
	if len(data) > 0 {
		conditionalFormats = []services.ConditionalFormat{
			{
				Column:    "D",
				RowStart:  2,
				RowEnd:    len(data) + 1,
				Condition: "normal",
				Color:     "#92D050", // Green
			},
			{
				Column:    "D",
				RowStart:  2,
				RowEnd:    len(data) + 1,
				Condition: "warning",
				Color:     "#FFEB9C", // Yellow
			},
			{
				Column:    "D",
				RowStart:  2,
				RowEnd:    len(data) + 1,
				Condition: "broken",
				Color:     "#FFC7CE", // Red
			},
			{
				Column:    "D",
				RowStart:  2,
				RowEnd:    len(data) + 1,
				Condition: "inactive",
				Color:     "#D9D9D9", // Gray
			},
		}
	}

	// Configure Excel export
	excelService := services.NewExcelService()
	config := services.ExcelExportConfig{
		SheetName: "Daftar PC",
		Headers: []string{
			"No", "PC Number", "Posisi", "Status", "Device Type", "Serial Number",
			"Brand & Model", "Processor", "RAM", "Storage", "OS", "Accessories",
			"Purchase Date", "Last Checked", "Notes", "Action Notes",
		},
		Data: data,
		ColumnWidths: map[string]float64{
			"A": 5,   // No
			"B": 10,  // PC Number
			"C": 15,  // Posisi
			"D": 10,  // Status
			"E": 22,  // Device Type
			"F": 18,  // Serial Number
			"G": 28,  // Brand & Model
			"H": 22,  // Processor
			"I": 12,  // RAM
			"J": 12,  // Storage
			"K": 22,  // OS
			"L": 28,  // Accessories
			"M": 14,  // Purchase Date
			"N": 18,  // Last Checked
			"O": 30,  // Notes
			"P": 30,  // Action Notes
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

	// Generate filename: pc_export_HHMM_DDMMYYYY.xlsx
	filename := excelService.GenerateFilename("pc_export")

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

// syncPCSoftware synchronizes software assignments for a PC
// Deletes existing entries and inserts new ones based on form data
func syncPCSoftware(db *database.DB, pcID int, requiredIDs []string, otherNames []string, otherDescs []string) error {
	checked := make(map[int]bool)
	for _, idStr := range requiredIDs {
		id, err := strconv.Atoi(idStr)
		if err == nil {
			checked[id] = true
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`DELETE FROM pc_software WHERE pc_id = ?`, pcID)
	if err != nil {
		return fmt.Errorf("failed to delete existing software: %w", err)
	}

	for swID := range checked {
		_, err = tx.Exec(`INSERT INTO pc_software (pc_id, software_id, installed) VALUES (?, ?, TRUE)`, pcID, swID)
		if err != nil {
			return fmt.Errorf("failed to insert required software %d: %w", swID, err)
		}
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

		var swID int
		err := tx.QueryRow(`SELECT id FROM software_catalog WHERE name = ?`, name).Scan(&swID)
		if err != nil {
			pgErr := tx.QueryRow(`INSERT INTO software_catalog (name, category, description) VALUES (?, 'other', ?) RETURNING id`, name, desc).Scan(&swID)
			if pgErr != nil {
				_, execErr := tx.Exec(`INSERT INTO software_catalog (name, category, description) VALUES (?, 'other', ?)`, name, desc)
				if execErr != nil {
					return fmt.Errorf("failed to create catalog entry for %s: %w", name, execErr)
				}
				tx.QueryRow(`SELECT MAX(id) FROM software_catalog WHERE name = ?`, name).Scan(&swID)
				if swID == 0 {
					return fmt.Errorf("failed to get ID for created software: %s", name)
				}
			} else if desc != "" {
				// Update description if already exists but new description provided
				tx.Exec(`UPDATE software_catalog SET description = ? WHERE id = ?`, desc, swID)
			}
		} else if desc != "" {
			tx.Exec(`UPDATE software_catalog SET description = ? WHERE id = ?`, desc, swID)
		}

		_, err = tx.Exec(`INSERT INTO pc_software (pc_id, software_id, installed) VALUES (?, ?, TRUE)`, pcID, swID)
		if err != nil {
			return fmt.Errorf("failed to insert other software %s: %w", name, err)
		}
	}

	return tx.Commit()
}
