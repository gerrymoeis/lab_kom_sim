package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/services"

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
		SELECT id, pc_number, row, column, status, processor, ram, storage,
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

	// Get software installed on this PC
	softwareRows, err := h.db.Query(`
		SELECT id, name, version, license, install_date, notes
		FROM software WHERE pc_id = ?
		ORDER BY name
	`, pc.ID)

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
		"title":       "Detail PC - Sistem Inventaris Lab",
		"username":    username,
		"role":        role,
		"currentPage": "pc",
		"pc":          pc,
		"software":    software,
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
		lastCheckedPtr = &lastChecked
	}

	// Handle file uploads (optional)
	var photoSerialFilename, photoFrontFilename string
	imageService := services.NewImageService()

	// Process photo_serial
	photoSerial, err := c.FormFile("photo_serial")
	if err == nil && photoSerial != nil {
		// Validate file size (max 5MB)
		if photoSerial.Size > 5*1024*1024 {
			c.HTML(http.StatusBadRequest, "pc/create.html", gin.H{
				"title":       "Tambah PC Baru - Sistem Inventaris Lab",
				"username":    username,
				"role":        role,
				"currentPage": "pc",
				"error":       "Ukuran foto serial number terlalu besar (max 5MB)",
			})
			return
		}

		// Validate file extension
		ext := strings.ToLower(filepath.Ext(photoSerial.Filename))
		allowedExts := []string{".jpg", ".jpeg", ".png", ".heic", ".heif"}
		isAllowed := false
		for _, allowed := range allowedExts {
			if ext == allowed {
				isAllowed = true
				break
			}
		}
		if !isAllowed {
			c.HTML(http.StatusBadRequest, "pc/create.html", gin.H{
				"title":       "Tambah PC Baru - Sistem Inventaris Lab",
				"username":    username,
				"role":        role,
				"currentPage": "pc",
				"error":       "Format foto serial number tidak didukung. Gunakan JPEG, PNG, atau HEIC",
			})
			return
		}

		// Generate unique filename - always use .jpeg extension (output is always JPEG)
		now := time.Now()
		photoSerialFilename = fmt.Sprintf("pc_%d_serial_%s.jpeg", pcNumber, now.Format("1504_02012006"))
		tempPath := filepath.Join("uploads", "temp", photoSerialFilename)
		finalPath := filepath.Join("uploads", "pc", photoSerialFilename)

		// Save temporary file with original extension for processing
		tempOriginal := tempPath + ext
		if err := c.SaveUploadedFile(photoSerial, tempOriginal); err != nil {
			c.HTML(http.StatusInternalServerError, "pc/create.html", gin.H{
				"title":       "Tambah PC Baru - Sistem Inventaris Lab",
				"username":    username,
				"role":        role,
				"currentPage": "pc",
				"error":       "Gagal menyimpan foto serial number",
			})
			return
		}

		// Compress and save (converts to JPEG)
		if err := imageService.CompressAndSave(tempOriginal, finalPath, 1280); err != nil {
			os.Remove(tempOriginal)
			c.HTML(http.StatusInternalServerError, "pc/create.html", gin.H{
				"title":       "Tambah PC Baru - Sistem Inventaris Lab",
				"username":    username,
				"role":        role,
				"currentPage": "pc",
				"error":       "Gagal mengkompresi foto serial number: " + err.Error(),
			})
			return
		}

		// Delete temp file
		os.Remove(tempOriginal)
	}

	// Process photo_front
	photoFront, err := c.FormFile("photo_front")
	if err == nil && photoFront != nil {
		// Validate file size (max 5MB)
		if photoFront.Size > 5*1024*1024 {
			// Cleanup photo_serial if already uploaded
			if photoSerialFilename != "" {
				os.Remove(filepath.Join("uploads", "pc", photoSerialFilename))
			}
			c.HTML(http.StatusBadRequest, "pc/create.html", gin.H{
				"title":       "Tambah PC Baru - Sistem Inventaris Lab",
				"username":    username,
				"role":        role,
				"currentPage": "pc",
				"error":       "Ukuran foto tampilan depan terlalu besar (max 5MB)",
			})
			return
		}

		// Validate file extension
		ext := strings.ToLower(filepath.Ext(photoFront.Filename))
		allowedExts := []string{".jpg", ".jpeg", ".png", ".heic", ".heif"}
		isAllowed := false
		for _, allowed := range allowedExts {
			if ext == allowed {
				isAllowed = true
				break
			}
		}
		if !isAllowed {
			// Cleanup photo_serial if already uploaded
			if photoSerialFilename != "" {
				os.Remove(filepath.Join("uploads", "pc", photoSerialFilename))
			}
			c.HTML(http.StatusBadRequest, "pc/create.html", gin.H{
				"title":       "Tambah PC Baru - Sistem Inventaris Lab",
				"username":    username,
				"role":        role,
				"currentPage": "pc",
				"error":       "Format foto tampilan depan tidak didukung. Gunakan JPEG, PNG, atau HEIC",
			})
			return
		}

		// Generate unique filename - always use .jpeg extension (output is always JPEG)
		now := time.Now()
		photoFrontFilename = fmt.Sprintf("pc_%d_front_%s.jpeg", pcNumber, now.Format("1504_02012006"))
		tempPath := filepath.Join("uploads", "temp", photoFrontFilename)
		finalPath := filepath.Join("uploads", "pc", photoFrontFilename)

		// Save temporary file with original extension for processing
		tempOriginal := tempPath + ext
		if err := c.SaveUploadedFile(photoFront, tempOriginal); err != nil {
			// Cleanup photo_serial if already uploaded
			if photoSerialFilename != "" {
				os.Remove(filepath.Join("uploads", "pc", photoSerialFilename))
			}
			c.HTML(http.StatusInternalServerError, "pc/create.html", gin.H{
				"title":       "Tambah PC Baru - Sistem Inventaris Lab",
				"username":    username,
				"role":        role,
				"currentPage": "pc",
				"error":       "Gagal menyimpan foto tampilan depan",
			})
			return
		}

		// Compress and save (converts to JPEG)
		if err := imageService.CompressAndSave(tempOriginal, finalPath, 1920); err != nil {
			os.Remove(tempOriginal)
			// Cleanup photo_serial if already uploaded
			if photoSerialFilename != "" {
				os.Remove(filepath.Join("uploads", "pc", photoSerialFilename))
			}
			c.HTML(http.StatusInternalServerError, "pc/create.html", gin.H{
				"title":       "Tambah PC Baru - Sistem Inventaris Lab",
				"username":    username,
				"role":        role,
				"currentPage": "pc",
				"error":       "Gagal mengkompresi foto tampilan depan: " + err.Error(),
			})
			return
		}

		// Delete temp file
		os.Remove(tempOriginal)
	}

	// Insert to database
	_, err = h.db.Exec(`
		INSERT INTO pcs (
			pc_number, row, column, status, 
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
		SELECT id, pc_number, row, column, status, processor, ram, storage,
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
		if t, err := time.Parse("2006-01-02", purchaseDateStr.String); err == nil {
			purchaseDateFormatted = t.Format("2006-01-02")
		}
	}

	var lastCheckedFormatted string
	if lastCheckedStr.Valid {
		if t, err := time.Parse("2006-01-02", lastCheckedStr.String); err == nil {
			lastCheckedFormatted = t.Format("2006-01-02")
		}
	}

	c.HTML(http.StatusOK, "pc/edit.html", gin.H{
		"title":        "Edit PC - Sistem Inventaris Lab",
		"username":     username,
		"role":         role,
		"currentPage":  "pc",
		"pc":           pc,
		"purchaseDate": purchaseDateFormatted,
		"lastChecked":  lastCheckedFormatted,
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
		// User provided new value
		lastCheckedPtr = &lastCheckedForm
	} else if currentLastChecked.Valid {
		// Preserve existing value
		lastCheckedPtr = &currentLastChecked.String
	}
	// If both empty, lastCheckedPtr = nil (set to NULL)

	// Handle file uploads (optional - keep existing if not uploaded)
	photoSerialFilename := ""
	if currentPhotoSerial.Valid {
		photoSerialFilename = currentPhotoSerial.String
	}
	
	photoFrontFilename := ""
	if currentPhotoFront.Valid {
		photoFrontFilename = currentPhotoFront.String
	}

	imageService := services.NewImageService()

	// Process photo_serial (if uploaded)
	photoSerial, err := c.FormFile("photo_serial")
	if err == nil && photoSerial != nil {
		// Validate file size (max 5MB)
		if photoSerial.Size > 5*1024*1024 {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"title":   "Error",
				"message": "Ukuran foto serial number terlalu besar (max 5MB)",
			})
			return
		}

		// Validate file extension
		ext := strings.ToLower(filepath.Ext(photoSerial.Filename))
		allowedExts := []string{".jpg", ".jpeg", ".png", ".heic", ".heif"}
		isAllowed := false
		for _, allowed := range allowedExts {
			if ext == allowed {
				isAllowed = true
				break
			}
		}
		if !isAllowed {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"title":   "Error",
				"message": "Format foto serial number tidak didukung. Gunakan JPEG, PNG, atau HEIC",
			})
			return
		}

		// Delete old photo if exists
		if photoSerialFilename != "" {
			oldPath := filepath.Join("uploads", "pc", photoSerialFilename)
			imageService.DeleteImage(oldPath)
		}

		// Generate unique filename - always use .jpeg extension (output is always JPEG)
		now := time.Now()
		photoSerialFilename = fmt.Sprintf("pc_%d_serial_%s.jpeg", currentPCNumber, now.Format("1504_02012006"))
		tempPath := filepath.Join("uploads", "temp", photoSerialFilename)
		finalPath := filepath.Join("uploads", "pc", photoSerialFilename)

		// Save temporary file with original extension for processing
		tempOriginal := tempPath + ext
		if err := c.SaveUploadedFile(photoSerial, tempOriginal); err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title":   "Error",
				"message": "Gagal menyimpan foto serial number",
			})
			return
		}

		// Compress and save (converts to JPEG)
		if err := imageService.CompressAndSave(tempOriginal, finalPath, 1280); err != nil {
			os.Remove(tempOriginal)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title":   "Error",
				"message": "Gagal mengkompresi foto serial number: " + err.Error(),
			})
			return
		}

		// Delete temp file
		os.Remove(tempOriginal)
	}

	// Process photo_front (if uploaded)
	photoFront, err := c.FormFile("photo_front")
	if err == nil && photoFront != nil {
		// Validate file size (max 5MB)
		if photoFront.Size > 5*1024*1024 {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"title":   "Error",
				"message": "Ukuran foto tampilan depan terlalu besar (max 5MB)",
			})
			return
		}

		// Validate file extension
		ext := strings.ToLower(filepath.Ext(photoFront.Filename))
		allowedExts := []string{".jpg", ".jpeg", ".png", ".heic", ".heif"}
		isAllowed := false
		for _, allowed := range allowedExts {
			if ext == allowed {
				isAllowed = true
				break
			}
		}
		if !isAllowed {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"title":   "Error",
				"message": "Format foto tampilan depan tidak didukung. Gunakan JPEG, PNG, atau HEIC",
			})
			return
		}

		// Delete old photo if exists
		if photoFrontFilename != "" {
			oldPath := filepath.Join("uploads", "pc", photoFrontFilename)
			imageService.DeleteImage(oldPath)
		}

		// Generate unique filename - always use .jpeg extension (output is always JPEG)
		now := time.Now()
		photoFrontFilename = fmt.Sprintf("pc_%d_front_%s.jpeg", currentPCNumber, now.Format("1504_02012006"))
		tempPath := filepath.Join("uploads", "temp", photoFrontFilename)
		finalPath := filepath.Join("uploads", "pc", photoFrontFilename)

		// Save temporary file with original extension for processing
		tempOriginal := tempPath + ext
		if err := c.SaveUploadedFile(photoFront, tempOriginal); err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title":   "Error",
				"message": "Gagal menyimpan foto tampilan depan",
			})
			return
		}

		// Compress and save (converts to JPEG)
		if err := imageService.CompressAndSave(tempOriginal, finalPath, 1920); err != nil {
			os.Remove(tempOriginal)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title":   "Error",
				"message": "Gagal mengkompresi foto tampilan depan: " + err.Error(),
			})
			return
		}

		// Delete temp file
		os.Remove(tempOriginal)
	}

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
		photoSerialUploaded, _ := c.FormFile("photo_serial")
		if photoSerialUploaded != nil {
			h.activityLogService.LogUpload(
				userID, username, role,
				"pc", pcID,
				photoSerialFilename, "photo_serial",
				ipAddress, userAgent,
			)
		}
		
		photoFrontUploaded, _ := c.FormFile("photo_front")
		if photoFrontUploaded != nil {
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
