package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

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

	// Process OCR
	ocrService := services.NewOCRService(apiKey)
	result, err := ocrService.ExtractLogbookFromImage(filepath)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": fmt.Sprintf("Gagal memproses OCR: %v", err),
		})
		return
	}

	// Redirect to preview page with extracted data
	c.HTML(http.StatusOK, "logbook/preview.html", gin.H{
		"title":       "Preview Hasil OCR - Sistem Inventaris Lab",
		"currentPage": "logbook",
		"username":    username,
		"role":        role,
		"entries":     result.Entries,
		"raw_text":    result.RawText,
		"success":     result.Success,
		"error":       result.Error,
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

	// Begin transaction
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memulai transaksi"})
		return
	}
	defer tx.Rollback()

	// Use INSERT OR IGNORE untuk handle duplicates
	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO logbook_entries (date, student_name, nim, time_in, time_out, purpose, source_file, created_at, updated_at)
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

		result, err := stmt.Exec(dateValue, names[i], nims[i], timeIn, timeOut, purpose, sourceFile, now, now)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Gagal menyimpan entry: %v", err)})
			return
		}
		
		// Check if row was actually inserted (not ignored due to duplicate)
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			savedCount++
		} else {
			duplicateCount++
		}
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
