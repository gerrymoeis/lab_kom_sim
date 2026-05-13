package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

// LogbookFilters represents filter options for logbook queries
type LogbookFilters struct {
	DateFrom  *time.Time
	DateTo    *time.Time
	Search    string
	SortBy    string
	SortOrder string
	Limit     int
	Offset    int
}

// LogbookList renders list of logbook entries with filters, search, and sort
func (h *Handler) LogbookList(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Parse pagination parameters
	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 25 // Default
	if sizeStr := c.Query("size"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 && s <= 100 {
			pageSize = s
		}
	}

	// Build filters
	filters := LogbookFilters{
		Limit:     pageSize,
		Offset:    (page - 1) * pageSize,
		SortBy:    c.DefaultQuery("sort_by", "date"),
		SortOrder: c.DefaultQuery("sort_order", "DESC"),
		Search:    c.Query("search"),
	}

	// Parse date filters
	if dateFrom := c.Query("date_from"); dateFrom != "" {
		if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
			filters.DateFrom = &t
		}
	}

	if dateTo := c.Query("date_to"); dateTo != "" {
		if t, err := time.Parse("2006-01-02", dateTo); err == nil {
			filters.DateTo = &t
		}
	}

	// Build query
	baseQuery := `
		SELECT id, date, student_name, nim, time_in, time_out, purpose, source_file, created_at
		FROM logbook_entries WHERE 1=1
	`
	countQuery := `SELECT COUNT(*) FROM logbook_entries WHERE 1=1`
	args := []interface{}{}
	conditions := ""

	// Date filtering
	if filters.DateFrom != nil {
		conditions += " AND date >= ?"
		args = append(args, filters.DateFrom)
	}
	if filters.DateTo != nil {
		conditions += " AND date <= ?"
		args = append(args, filters.DateTo)
	}

	// Search functionality
	if filters.Search != "" {
		conditions += ` AND (
			student_name LIKE ? OR 
			nim LIKE ? OR 
			purpose LIKE ?
		)`
		searchTerm := "%" + filters.Search + "%"
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	// Get total count
	var totalCount int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	err := h.db.QueryRow(countQuery+conditions, countArgs...).Scan(&totalCount)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal menghitung data logbook",
		})
		return
	}

	// Sorting
	orderBy := " ORDER BY "
	switch filters.SortBy {
	case "student_name":
		orderBy += "student_name"
	case "nim":
		orderBy += "nim"
	case "time_in":
		orderBy += "time_in"
	case "created_at":
		orderBy += "created_at"
	default:
		orderBy += "date"
	}

	if filters.SortOrder == "ASC" {
		orderBy += " ASC"
	} else {
		orderBy += " DESC"
	}

	// Add secondary sort untuk consistency
	if filters.SortBy != "date" {
		orderBy += ", date DESC"
	}
	if filters.SortBy != "time_in" && filters.SortBy != "date" {
		orderBy += ", time_in DESC"
	}

	// Final query dengan pagination
	finalQuery := baseQuery + conditions + orderBy + " LIMIT ? OFFSET ?"
	args = append(args, filters.Limit, filters.Offset)

	rows, err := h.db.Query(finalQuery, args...)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data logbook",
		})
		return
	}
	defer rows.Close()

	var entries []models.LogbookEntry
	for rows.Next() {
		var entry models.LogbookEntry
		err := rows.Scan(&entry.ID, &entry.Date, &entry.StudentName, &entry.NIM,
			&entry.TimeIn, &entry.TimeOut, &entry.Purpose, &entry.SourceFile, &entry.CreatedAt)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	// Calculate pagination
	totalPages := (totalCount + filters.Limit - 1) / filters.Limit
	if totalPages == 0 {
		totalPages = 1
	}

	c.HTML(http.StatusOK, "logbook/list.html", gin.H{
		"title":       "Logbook Absensi - Sistem Inventaris Lab",
		"currentPage": "logbook",
		"username":    username,
		"role":        role,
		"entries":     entries,
		"totalCount":  totalCount,
		"page":        page,
		"totalPages":  totalPages,
		"pageSize":    pageSize,
		"filters": gin.H{
			"date_from":  c.Query("date_from"),
			"date_to":    c.Query("date_to"),
			"search":     filters.Search,
			"sort_by":    filters.SortBy,
			"sort_order": filters.SortOrder,
		},
	})
}

// LogbookUploadPage renders logbook upload page
func (h *Handler) LogbookUploadPage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	c.HTML(http.StatusOK, "logbook/upload.html", gin.H{
		"title":       "Upload Logbook - Sistem Inventaris Lab",
		"currentPage": "logbook",
		"username":    username,
		"role":        role,
	})
}

// LogbookUpload handles logbook file upload and OCR processing
func (h *Handler) LogbookUpload(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Only admin can upload
	if role != "admin" {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"title":   "Akses Ditolak",
			"message": "Hanya admin yang dapat mengupload logbook",
		})
		return
	}

	// Get uploaded file
	file, err := c.FormFile("logbook_image")
	if err != nil {
		c.HTML(http.StatusBadRequest, "logbook/upload.html", gin.H{
			"title":    "Upload Logbook - Sistem Inventaris Lab",
			"username": username,
			"role":     role,
			"error":    "Gagal mengambil file. Pastikan Anda memilih file gambar.",
		})
		return
	}

	// Validate file type
	ext := filepath.Ext(file.Filename)
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		c.HTML(http.StatusBadRequest, "logbook/upload.html", gin.H{
			"title":    "Upload Logbook - Sistem Inventaris Lab",
			"username": username,
			"role":     role,
			"error":    "Format file tidak didukung. Gunakan JPG atau PNG.",
		})
		return
	}

	// Create upload directory if not exists
	uploadDir := filepath.Join("uploads", "logbook")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal membuat direktori upload",
		})
		return
	}

	// Save file with timestamp
	filename := fmt.Sprintf("logbook_%d%s", time.Now().Unix(), ext)
	filepath := filepath.Join(uploadDir, filename)
	
	if err := c.SaveUploadedFile(file, filepath); err != nil {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogAuth(
				userID, username, role, "upload", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to save logbook file: %v", err),
			)
		}
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal menyimpan file",
		})
		return
	}

	// Log successful upload
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		h.activityLogService.LogUpload(
			userID, username, role,
			"logbook", 0, // No specific entity ID for upload
			filename, "logbook_image",
			ipAddress, userAgent,
		)
	}

	// Get Gemini API key from config
	apiKey := h.cfg.GeminiAPIKey
	if apiKey == "" {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gemini API key tidak dikonfigurasi. Silakan tambahkan GEMINI_API_KEY di file .env",
		})
		return
	}

	ocrService := services.NewOCRService(apiKey)
	result, err := ocrService.ExtractLogbookFromImage(filepath)

	success := true
	errorMsg := ""
	var entries []services.LogbookEntry
	rawText := ""

	if err != nil {
		success = false
		errorMsg = fmt.Sprintf("Gagal memproses OCR: %v. File tetap tersimpan, Anda bisa coba upload ulang.", err)
		entries = []services.LogbookEntry{}
	} else {
		success = result.Success
		errorMsg = result.Error
		entries = result.Entries
		rawText = result.RawText
	}

	c.HTML(http.StatusOK, "logbook/preview.html", gin.H{
		"title":       "Preview Hasil OCR - Sistem Inventaris Lab",
		"currentPage": "logbook",
		"username":    username,
		"role":        role,
		"entries":     entries,
		"raw_text":    rawText,
		"success":     success,
		"error":       errorMsg,
		"source_file": filename,
	})
}

// LogbookSave saves logbook entries from preview page to database
func (h *Handler) LogbookSave(c *gin.Context) {
	_, _, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
		return
	}

	// Parse form data
	if err := c.Request.ParseForm(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal parsing form"})
		return
	}

	sourceFile := c.PostForm("source_file")
	
	// Get array of entries
	dates := c.PostFormArray("date[]")
	names := c.PostFormArray("student_name[]")
	nims := c.PostFormArray("nim[]")
	timeIns := c.PostFormArray("time_in[]")
	timeOuts := c.PostFormArray("time_out[]")
	purposes := c.PostFormArray("purpose[]")

	if len(dates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tidak ada data untuk disimpan"})
		return
	}

	// Load duplicate check config
	dupConfig := config.DefaultDuplicateConfig

	// Begin transaction
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memulai transaksi"})
		return
	}
	defer tx.Rollback()

	// Prepare insert statement
	stmt, err := tx.Prepare(`
		INSERT INTO logbook_entries (date, student_name, nim, time_in, time_out, purpose, source_file, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyiapkan statement"})
		return
	}
	defer stmt.Close()

	now := time.Now()
	savedCount := 0
	duplicateCount := 0

	for i := 0; i < len(dates); i++ {
		// Skip empty entries
		if dates[i] == "" && names[i] == "" && nims[i] == "" {
			continue
		}

		// Parse date
		var dateValue time.Time
		if dates[i] != "" {
			parsedDate, err := time.Parse("2006-01-02", dates[i])
			if err != nil {
				// Try alternative format
				parsedDate, err = time.Parse("02/01/2006", dates[i])
				if err != nil {
					dateValue = now
				} else {
					dateValue = parsedDate
				}
			} else {
				dateValue = parsedDate
			}
		} else {
			dateValue = now
		}

		timeIn := timeIns[i]
		timeOut := ""
		if i < len(timeOuts) {
			timeOut = timeOuts[i]
		}
		
		purpose := ""
		if i < len(purposes) {
			purpose = purposes[i]
		}

		// Check for similarity-based duplicates
		isDuplicate := false
		rows, err := h.db.Query(`
			SELECT date, student_name, nim, time_in 
			FROM logbook_entries 
			WHERE date = ? AND time_in = ?
		`, dateValue, timeIn)
		
		if err == nil {
			for rows.Next() {
				var existingDate time.Time
				var existingName, existingNIM, existingTimeIn string
				if err := rows.Scan(&existingDate, &existingName, &existingNIM, &existingTimeIn); err == nil {
					if isDuplicateEntry(dateValue, existingDate, timeIn, existingTimeIn, 
						names[i], existingName, nims[i], existingNIM, dupConfig) {
						isDuplicate = true
						rows.Close()
						break
					}
				}
			}
			rows.Close()
		}

		if isDuplicate {
			duplicateCount++
			continue
		}

		// Insert if not duplicate
		_, err = stmt.Exec(dateValue, names[i], nims[i], timeIn, timeOut, purpose, sourceFile, now, now)
		if err != nil {
			// Check if database-level unique constraint triggered
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				duplicateCount++
				continue
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Gagal menyimpan entry: %v", err)})
			return
		}
		
		savedCount++
	}

	if err := tx.Commit(); err != nil {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogAuth(
				userID, username, role, "create", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to commit logbook entries: %v", err),
			)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data"})
		return
	}

	// Log successful bulk create
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		h.activityLogService.LogCreate(
			userID, username, role,
			"logbook", 0, // Bulk operation, no specific ID
			map[string]interface{}{
				"entries_saved":     savedCount,
				"entries_duplicate": duplicateCount,
				"source_file":       sourceFile,
			},
			ipAddress, userAgent,
		)
	}

	// Return result dengan info tentang duplicates
	message := fmt.Sprintf("Berhasil menyimpan %d entry logbook", savedCount)
	if duplicateCount > 0 {
		message += fmt.Sprintf(" (%d duplikat dilewati)", duplicateCount)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        message,
		"saved":          savedCount,
		"duplicates":     duplicateCount,
		"total_processed": len(dates),
	})
}

// LogbookExport exports logbook to Excel
func (h *Handler) LogbookExport(c *gin.Context) {
	_, _, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	if role != "admin" {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"title":   "Akses Ditolak",
			"message": "Hanya admin yang dapat export logbook",
		})
		return
	}

	// Get filter parameters
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	// Build query
	query := `
		SELECT id, date, student_name, nim, time_in, time_out, purpose, source_file, created_at
		FROM logbook_entries
		WHERE 1=1
	`
	args := []interface{}{}

	if startDate != "" {
		query += " AND date >= ?"
		args = append(args, startDate)
	}
	if endDate != "" {
		query += " AND date <= ?"
		args = append(args, endDate)
	}

	query += " ORDER BY date DESC, time_in DESC"

	rows, err := h.db.Query(query, args...)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data logbook",
		})
		return
	}
	defer rows.Close()

	var entries []models.LogbookEntry
	for rows.Next() {
		var entry models.LogbookEntry
		err := rows.Scan(&entry.ID, &entry.Date, &entry.StudentName, &entry.NIM,
			&entry.TimeIn, &entry.TimeOut, &entry.Purpose, &entry.SourceFile, &entry.CreatedAt)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	// Transform data to [][]interface{}
	data := [][]interface{}{}
	for i, entry := range entries {
		row := []interface{}{
			i + 1,                            // No
			entry.Date.Format("2006-01-02"),  // Tanggal
			entry.StudentName,                // Nama Mahasiswa
			entry.NIM,                        // NIM
			entry.Purpose,                    // Keperluan
			entry.TimeIn,                     // Jam Masuk
			entry.TimeOut,                    // Jam Keluar
		}
		data = append(data, row)
	}

	// Configure Excel export
	excelService := services.NewExcelService()
	config := services.ExcelExportConfig{
		SheetName: "Logbook",
		Headers:   []string{"No", "Tanggal", "Nama Mahasiswa", "NIM", "Keperluan", "Jam Masuk", "Jam Keluar"},
		Data:      data,
		ColumnWidths: map[string]float64{
			"A": 5,   // No
			"B": 12,  // Tanggal
			"C": 25,  // Nama Mahasiswa
			"D": 13,  // NIM
			"E": 30,  // Keperluan
			"F": 11,  // Jam Masuk
			"G": 11,  // Jam Keluar
		},
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

	// Generate filename with new format: logbook_export_HHMM_DDMMYYYY.xlsx
	filename := excelService.GenerateFilename("logbook_export")

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

// LogbookExportPreview exports logbook from preview page to Excel
func (h *Handler) LogbookExportPreview(c *gin.Context) {
	_, _, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	if role != "admin" {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"title":   "Akses Ditolak",
			"message": "Hanya admin yang dapat export logbook",
		})
		return
	}

	// Get data from query params
	dates := c.QueryArray("date[]")
	names := c.QueryArray("student_name[]")
	nims := c.QueryArray("nim[]")
	timeIns := c.QueryArray("time_in[]")
	timeOuts := c.QueryArray("time_out[]")
	purposes := c.QueryArray("purpose[]")

	if len(dates) == 0 {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "Error",
			"message": "Tidak ada data untuk di-export",
		})
		return
	}

	// Transform data to [][]interface{}
	data := [][]interface{}{}
	for i := 0; i < len(dates); i++ {
		purpose := ""
		if i < len(purposes) {
			purpose = purposes[i]
		}
		
		timeOut := ""
		if i < len(timeOuts) {
			timeOut = timeOuts[i]
		}
		
		row := []interface{}{
			i + 1,      // No
			dates[i],   // Tanggal
			names[i],   // Nama Mahasiswa
			nims[i],    // NIM
			purpose,    // Keperluan
			timeIns[i], // Jam Masuk
			timeOut,    // Jam Keluar
		}
		data = append(data, row)
	}

	// Configure Excel export
	excelService := services.NewExcelService()
	config := services.ExcelExportConfig{
		SheetName: "Logbook",
		Headers:   []string{"No", "Tanggal", "Nama Mahasiswa", "NIM", "Keperluan", "Jam Masuk", "Jam Keluar"},
		Data:      data,
		ColumnWidths: map[string]float64{
			"A": 5,   // No
			"B": 12,  // Tanggal
			"C": 25,  // Nama Mahasiswa
			"D": 13,  // NIM
			"E": 30,  // Keperluan
			"F": 11,  // Jam Masuk
			"G": 11,  // Jam Keluar
		},
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

	// Generate filename with new format: logbook_preview_HHMM_DDMMYYYY.xlsx
	filename := excelService.GenerateFilename("logbook_preview")

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

// LogbookCreatePage renders manual logbook entry creation form
func (h *Handler) LogbookCreatePage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	c.HTML(http.StatusOK, "logbook/create.html", gin.H{
		"title":       "Tambah Entry Logbook - Sistem Inventaris Lab",
		"currentPage": "logbook",
		"username":    username,
		"role":        role,
	})
}

// LogbookCreate handles manual logbook entry creation
func (h *Handler) LogbookCreate(c *gin.Context) {
	_, _, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Parse form data
	dateStr := c.PostForm("date")
	studentName := c.PostForm("student_name")
	nim := c.PostForm("nim")
	timeIn := c.PostForm("time_in")
	timeOut := c.PostForm("time_out")
	purpose := c.PostForm("purpose")

	// Validate required fields
	if dateStr == "" || studentName == "" || nim == "" || timeIn == "" {
		c.HTML(http.StatusBadRequest, "logbook/create.html", gin.H{
			"title":       "Tambah Entry Logbook",
			"currentPage": "logbook",
			"error":       "Tanggal, Nama Mahasiswa, NIM, dan Jam Masuk harus diisi",
		})
		return
	}

	// Parse date
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.HTML(http.StatusBadRequest, "logbook/create.html", gin.H{
			"title":       "Tambah Entry Logbook",
			"currentPage": "logbook",
			"error":       "Format tanggal tidak valid",
		})
		return
	}

	// Apply normalization (same as OCR)
	studentName = normalizeStudentName(studentName)
	nim = normalizeNIM(nim)
	purpose = normalizePurpose(purpose)

	// Load duplicate check config
	dupConfig := config.DefaultDuplicateConfig

	// Check for similarity-based duplicates
	isDuplicate := false
	rows, err := h.db.Query(`
		SELECT date, student_name, nim, time_in 
		FROM logbook_entries 
		WHERE date = ? AND time_in = ?
	`, date, timeIn)
	
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var existingDate time.Time
			var existingName, existingNIM, existingTimeIn string
			if err := rows.Scan(&existingDate, &existingName, &existingNIM, &existingTimeIn); err == nil {
				// Check similarity
				if isDuplicateEntry(date, existingDate, timeIn, existingTimeIn, 
					studentName, existingName, nim, existingNIM, dupConfig) {
					isDuplicate = true
					break
				}
			}
		}
	}

	if isDuplicate {
		c.HTML(http.StatusBadRequest, "logbook/create.html", gin.H{
			"title":       "Tambah Entry Logbook",
			"currentPage": "logbook",
			"error":       "Entry duplikat: Mahasiswa dengan nama/NIM serupa sudah tercatat di tanggal dan jam yang sama",
		})
		return
	}

	// Insert to database
	result, err := h.db.Exec(`
		INSERT INTO logbook_entries (date, student_name, nim, time_in, time_out, purpose, source_file, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, date, studentName, nim, timeIn, timeOut, purpose, "manual_entry", time.Now(), time.Now())

	if err != nil {
		// Check if duplicate
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			c.HTML(http.StatusBadRequest, "logbook/create.html", gin.H{
				"title":       "Tambah Entry Logbook",
				"currentPage": "logbook",
				"error":       "Entry duplikat: Mahasiswa dengan NIM ini sudah tercatat di tanggal dan jam yang sama",
			})
			return
		}

		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogAuth(
				userID, username, role, "create", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to create logbook entry: %v", err),
			)
		}
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal menyimpan entry logbook",
		})
		return
	}

	// Get last insert ID and log
	entryID, _ := result.LastInsertId()
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		h.activityLogService.LogCreate(
			userID, username, role,
			"logbook", int(entryID),
			map[string]interface{}{
				"date":         dateStr,
				"student_name": studentName,
				"nim":          nim,
			},
			ipAddress, userAgent,
		)
	}

	c.Redirect(http.StatusFound, "/logbook")
}

// LogbookEditPage renders logbook entry edit form
func (h *Handler) LogbookEditPage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	id := c.Param("id")
	var entry models.LogbookEntry

	err := h.db.QueryRow(`
		SELECT id, date, student_name, nim, time_in, time_out, purpose, source_file
		FROM logbook_entries WHERE id = ?
	`, id).Scan(&entry.ID, &entry.Date, &entry.StudentName, &entry.NIM,
		&entry.TimeIn, &entry.TimeOut, &entry.Purpose, &entry.SourceFile)

	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Entry Tidak Ditemukan",
			"message": "Entry logbook yang Anda cari tidak ditemukan",
		})
		return
	}

	c.HTML(http.StatusOK, "logbook/edit.html", gin.H{
		"title":       "Edit Entry Logbook - Sistem Inventaris Lab",
		"currentPage": "logbook",
		"username":    username,
		"role":        role,
		"entry":       entry,
		"dateStr":     entry.Date.Format("2006-01-02"),
	})
}

// LogbookEdit handles logbook entry update
func (h *Handler) LogbookEdit(c *gin.Context) {
	_, _, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	id := c.Param("id")
	dateStr := c.PostForm("date")
	studentName := c.PostForm("student_name")
	nim := c.PostForm("nim")
	timeIn := c.PostForm("time_in")
	timeOut := c.PostForm("time_out")
	purpose := c.PostForm("purpose")

	// Validate required fields
	if dateStr == "" || studentName == "" || nim == "" || timeIn == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "Error",
			"message": "Tanggal, Nama Mahasiswa, NIM, dan Jam Masuk harus diisi",
		})
		return
	}

	// Parse date
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "Error",
			"message": "Format tanggal tidak valid",
		})
		return
	}

	// Get old values for logging
	var oldDate time.Time
	var oldName, oldNIM string
	err = h.db.QueryRow(`
		SELECT date, student_name, nim FROM logbook_entries WHERE id = ?
	`, id).Scan(&oldDate, &oldName, &oldNIM)

	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Error",
			"message": "Entry logbook tidak ditemukan",
		})
		return
	}

	// Apply normalization
	studentName = normalizeStudentName(studentName)
	nim = normalizeNIM(nim)
	purpose = normalizePurpose(purpose)

	// Load duplicate check config
	dupConfig := config.DefaultDuplicateConfig

	// Check for similarity-based duplicates (excluding current entry)
	isDuplicate := false
	rows, err := h.db.Query(`
		SELECT id, date, student_name, nim, time_in 
		FROM logbook_entries 
		WHERE date = ? AND time_in = ? AND id != ?
	`, date, timeIn, id)
	
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var existingID int
			var existingDate time.Time
			var existingName, existingNIM, existingTimeIn string
			if err := rows.Scan(&existingID, &existingDate, &existingName, &existingNIM, &existingTimeIn); err == nil {
				// Check similarity
				if isDuplicateEntry(date, existingDate, timeIn, existingTimeIn, 
					studentName, existingName, nim, existingNIM, dupConfig) {
					isDuplicate = true
					break
				}
			}
		}
	}

	if isDuplicate {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "Error",
			"message": "Entry duplikat: Mahasiswa dengan nama/NIM serupa sudah tercatat di tanggal dan jam yang sama",
		})
		return
	}

	// Update database
	_, err = h.db.Exec(`
		UPDATE logbook_entries 
		SET date = ?, student_name = ?, nim = ?, time_in = ?, time_out = ?, purpose = ?, updated_at = ?
		WHERE id = ?
	`, date, studentName, nim, timeIn, timeOut, purpose, time.Now(), id)

	if err != nil {
		// Check if duplicate
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"title":   "Error",
				"message": "Entry duplikat: Mahasiswa dengan NIM ini sudah tercatat di tanggal dan jam yang sama",
			})
			return
		}

		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			entryIDInt, _ := strconv.Atoi(id)
			h.activityLogService.LogAuth(
				userID, username, role, "update", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to update logbook entry #%d: %v", entryIDInt, err),
			)
		}
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengupdate entry logbook",
		})
		return
	}

	// Log successful update
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		entryIDInt, _ := strconv.Atoi(id)

		oldValues := map[string]interface{}{
			"date":         oldDate.Format("2006-01-02"),
			"student_name": oldName,
			"nim":          oldNIM,
		}

		newValues := map[string]interface{}{
			"date":         dateStr,
			"student_name": studentName,
			"nim":          nim,
		}

		h.activityLogService.LogUpdate(
			userID, username, role,
			"logbook", entryIDInt,
			oldValues,
			newValues,
			ipAddress, userAgent,
		)
	}

	c.Redirect(http.StatusFound, "/logbook")
}

// LogbookDelete handles logbook entry deletion
func (h *Handler) LogbookDelete(c *gin.Context) {
	_, _, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id := c.Param("id")

	// Get entry data before delete
	var entryID int
	var date time.Time
	var studentName, nim string
	err := h.db.QueryRow(`
		SELECT id, date, student_name, nim FROM logbook_entries WHERE id = ?
	`, id).Scan(&entryID, &date, &studentName, &nim)

	if err != nil {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogAuth(
				userID, username, role, "delete", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to get logbook entry data for delete: %v", err),
			)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengambil data entry logbook",
		})
		return
	}

	// Delete entry
	_, err = h.db.Exec("DELETE FROM logbook_entries WHERE id = ?", id)
	if err != nil {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogAuth(
				userID, username, role, "delete", false,
				ipAddress, userAgent, fmt.Sprintf("Failed to delete logbook entry #%d: %v", entryID, err),
			)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menghapus entry logbook",
		})
		return
	}

	// Log successful delete
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)

		oldValues := map[string]interface{}{
			"date":         date.Format("2006-01-02"),
			"student_name": studentName,
			"nim":          nim,
		}

		h.activityLogService.LogDelete(
			userID, username, role,
			"logbook", entryID,
			oldValues,
			ipAddress, userAgent,
		)
	}

	c.Redirect(http.StatusFound, "/logbook")
}

// Helper functions for normalization (same logic as OCR)
func normalizeStudentName(name string) string {
	// Apply title case with abbreviation normalization
	return toTitleCaseWithAbbr(name)
}

func normalizeNIM(nim string) string {
	// Uppercase and remove all spaces
	nim = strings.ToUpper(strings.TrimSpace(nim))
	nim = strings.ReplaceAll(nim, " ", "")
	return nim
}

func normalizePurpose(purpose string) string {
	// Apply title case
	return toTitleCaseWithAbbr(purpose)
}

func toTitleCaseWithAbbr(text string) string {
	if text == "" {
		return ""
	}
	
	// Trim and remove double spaces
	text = strings.TrimSpace(text)
	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")
	
	// Split by space and capitalize each word
	words := strings.Fields(text)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	
	result := strings.Join(words, " ")
	
	// Normalize abbreviations
	// Pattern: single uppercase letter followed by another uppercase letter
	reAbbr := regexp.MustCompile(`\b([A-Z])([A-Z])\b`)
	result = reAbbr.ReplaceAllString(result, "$1.$2")
	
	// Remove trailing dot at the end
	result = strings.TrimSuffix(result, ".")
	
	return result
}

