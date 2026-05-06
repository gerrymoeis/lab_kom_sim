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
	"github.com/xuri/excelize/v2"
)

// LogbookList renders list of logbook entries
func (h *Handler) LogbookList(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	rows, err := h.db.Query(`
		SELECT id, date, student_name, nim, time_in, time_out, notes, source_file, created_at
		FROM logbook_entries
		ORDER BY date DESC, time_in DESC
		LIMIT 100
	`)
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
			&entry.TimeIn, &entry.TimeOut, &entry.Notes, &entry.SourceFile, &entry.CreatedAt)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	c.HTML(http.StatusOK, "logbook/list.html", gin.H{
		"title":       "Logbook Absensi - Sistem Inventaris Lab",
		"currentPage": "logbook",
		"username":    username,
		"role":        role,
		"entries":     entries,
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
	notes := c.PostFormArray("notes[]")

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

	// Insert entries
	stmt, err := tx.Prepare(`
		INSERT INTO logbook_entries (date, student_name, nim, time_in, time_out, notes, source_file, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyiapkan statement"})
		return
	}
	defer stmt.Close()

	now := time.Now()
	savedCount := 0

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
		
		note := ""
		if i < len(notes) {
			note = notes[i]
		}

		_, err = stmt.Exec(dateValue, names[i], nims[i], timeIn, timeOut, note, sourceFile, now, now)
		if err != nil {
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
				"entries_count": savedCount,
				"source_file":   sourceFile,
			},
			ipAddress, userAgent,
		)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Berhasil menyimpan %d entry logbook", savedCount),
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
		SELECT id, date, student_name, nim, time_in, time_out, notes, source_file, created_at
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
			&entry.TimeIn, &entry.TimeOut, &entry.Notes, &entry.SourceFile, &entry.CreatedAt)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	// Create Excel file
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Logbook"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal membuat sheet Excel",
		})
		return
	}

	// Set headers
	headers := []string{"No", "Tanggal", "Nama Mahasiswa", "NIM", "Jam Masuk", "Jam Keluar", "Keterangan"}
	for i, header := range headers {
		cell := string(rune('A'+i)) + "1"
		f.SetCellValue(sheetName, cell, header)
	}

	// Set header style
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	f.SetCellStyle(sheetName, "A1", "G1", headerStyle)

	// Set data
	for i, entry := range entries {
		row := i + 2
		f.SetCellValue(sheetName, "A"+strconv.Itoa(row), i+1)
		f.SetCellValue(sheetName, "B"+strconv.Itoa(row), entry.Date.Format("2006-01-02"))
		f.SetCellValue(sheetName, "C"+strconv.Itoa(row), entry.StudentName)
		f.SetCellValue(sheetName, "D"+strconv.Itoa(row), entry.NIM)
		f.SetCellValue(sheetName, "E"+strconv.Itoa(row), entry.TimeIn)
		f.SetCellValue(sheetName, "F"+strconv.Itoa(row), entry.TimeOut)
		f.SetCellValue(sheetName, "G"+strconv.Itoa(row), entry.Notes)
	}

	// Auto-fit columns
	f.SetColWidth(sheetName, "A", "A", 5)
	f.SetColWidth(sheetName, "B", "B", 12)
	f.SetColWidth(sheetName, "C", "C", 25)
	f.SetColWidth(sheetName, "D", "D", 15)
	f.SetColWidth(sheetName, "E", "E", 12)
	f.SetColWidth(sheetName, "F", "F", 12)
	f.SetColWidth(sheetName, "G", "G", 30)

	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	// Generate filename
	filename := fmt.Sprintf("logbook_export_%s.xlsx", time.Now().Format("20060102_150405"))

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
	notes := c.QueryArray("notes[]")

	if len(dates) == 0 {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "Error",
			"message": "Tidak ada data untuk di-export",
		})
		return
	}

	// Create Excel file
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Logbook"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal membuat sheet Excel",
		})
		return
	}

	// Set headers
	headers := []string{"No", "Tanggal", "Nama Mahasiswa", "NIM", "Jam Masuk", "Jam Keluar", "Keterangan"}
	for i, header := range headers {
		cell := string(rune('A'+i)) + "1"
		f.SetCellValue(sheetName, cell, header)
	}

	// Set header style
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	f.SetCellStyle(sheetName, "A1", "G1", headerStyle)

	// Set data
	for i := 0; i < len(dates); i++ {
		row := i + 2
		f.SetCellValue(sheetName, "A"+strconv.Itoa(row), i+1)
		f.SetCellValue(sheetName, "B"+strconv.Itoa(row), dates[i])
		f.SetCellValue(sheetName, "C"+strconv.Itoa(row), names[i])
		f.SetCellValue(sheetName, "D"+strconv.Itoa(row), nims[i])
		f.SetCellValue(sheetName, "E"+strconv.Itoa(row), timeIns[i])
		
		timeOut := ""
		if i < len(timeOuts) {
			timeOut = timeOuts[i]
		}
		f.SetCellValue(sheetName, "F"+strconv.Itoa(row), timeOut)
		
		note := ""
		if i < len(notes) {
			note = notes[i]
		}
		f.SetCellValue(sheetName, "G"+strconv.Itoa(row), note)
	}

	// Auto-fit columns
	f.SetColWidth(sheetName, "A", "A", 5)
	f.SetColWidth(sheetName, "B", "B", 12)
	f.SetColWidth(sheetName, "C", "C", 25)
	f.SetColWidth(sheetName, "D", "D", 15)
	f.SetColWidth(sheetName, "E", "E", 12)
	f.SetColWidth(sheetName, "F", "F", 12)
	f.SetColWidth(sheetName, "G", "G", 30)

	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	// Generate filename
	filename := fmt.Sprintf("logbook_preview_%s.xlsx", time.Now().Format("20060102_150405"))

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
