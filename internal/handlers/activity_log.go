package handlers

import (
	"net/http"
	"strconv"
	"time"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

// ActivityLogList displays the activity log list page with filters
func (h *Handler) ActivityLogList(c *gin.Context) {
	// Get current user info
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Parse filters from query params
	filters := services.ActivityLogFilters{
		Limit:  50, // Default 50 entries per page
		Offset: 0,
	}

	// Parse page number
	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	filters.Offset = (page - 1) * filters.Limit

	// Parse date filters
	if dateFrom := c.Query("date_from"); dateFrom != "" {
		if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
			filters.DateFrom = &t
		}
	}

	if dateTo := c.Query("date_to"); dateTo != "" {
		if t, err := time.Parse("2006-01-02", dateTo); err == nil {
			// Set to end of day
			endOfDay := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			filters.DateTo = &endOfDay
		}
	}

	// Parse action filter
	if action := c.Query("action"); action != "" {
		filters.Action = action
	}

	// Parse entity type filter
	if entityType := c.Query("entity_type"); entityType != "" {
		filters.EntityType = entityType
	}

	// Parse username filter
	if username := c.Query("username"); username != "" {
		filters.Username = username
	}

	// Parse status filter
	if status := c.Query("status"); status != "" {
		filters.Status = status
	}

	// Parse search keyword
	keyword := c.Query("search")

	var logs []interface{}
	var totalCount int
	var err error

	// Use search if keyword provided, otherwise use filters
	if keyword != "" {
		logsResult, count, searchErr := h.activityLogService.SearchLogs(keyword, filters.Limit, filters.Offset)
		if searchErr != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title":   "Error",
				"message": "Gagal mencari activity logs: " + searchErr.Error(),
			})
			return
		}
		// Convert to []interface{} for template
		logs = make([]interface{}, len(logsResult))
		for i, log := range logsResult {
			logs[i] = log
		}
		totalCount = count
	} else {
		logsResult, count, filterErr := h.activityLogService.GetLogs(filters)
		if filterErr != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title":   "Error",
				"message": "Gagal mengambil activity logs: " + filterErr.Error(),
			})
			return
		}
		// Convert to []interface{} for template
		logs = make([]interface{}, len(logsResult))
		for i, log := range logsResult {
			logs[i] = log
		}
		totalCount = count
	}

	// Calculate pagination
	totalPages := (totalCount + filters.Limit - 1) / filters.Limit
	if totalPages == 0 {
		totalPages = 1
	}

	// Get unique usernames for filter dropdown
	usernames, err := h.getUniqueUsernames()
	if err != nil {
		// Log error but continue
		usernames = []string{}
	}

	c.HTML(http.StatusOK, "activity_log/list.html", gin.H{
		"title":        "Activity Logs",
		"currentPage":  "activity_logs",
		"username":     username,
		"role":         role,
		"logs":         logs,
		"totalCount":   totalCount,
		"page":         page,
		"totalPages":   totalPages,
		"filters": gin.H{
			"date_from":   c.Query("date_from"),
			"date_to":     c.Query("date_to"),
			"action":      c.Query("action"),
			"entity_type": c.Query("entity_type"),
			"username":    c.Query("username"),
			"status":      c.Query("status"),
			"search":      keyword,
		},
		"usernames": usernames,
	})
}

// getUniqueUsernames retrieves unique usernames from activity logs
func (h *Handler) getUniqueUsernames() ([]string, error) {
	query := "SELECT DISTINCT username FROM activity_logs ORDER BY username"
	rows, err := h.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	usernames := []string{}
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, err
		}
		usernames = append(usernames, username)
	}

	return usernames, nil
}

// ActivityLogExport exports activity logs to Excel
func (h *Handler) ActivityLogExport(c *gin.Context) {
	_, _, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	if role != "admin" {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"title":   "Akses Ditolak",
			"message": "Hanya admin yang dapat export activity logs",
		})
		return
	}

	// Parse filters from query params (same as ActivityLogList)
	filters := services.ActivityLogFilters{
		Limit:  1000, // Max 1000 rows for export
		Offset: 0,
	}

	// Parse date filters
	if dateFrom := c.Query("date_from"); dateFrom != "" {
		if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
			filters.DateFrom = &t
		}
	}

	if dateTo := c.Query("date_to"); dateTo != "" {
		if t, err := time.Parse("2006-01-02", dateTo); err == nil {
			// Set to end of day
			endOfDay := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			filters.DateTo = &endOfDay
		}
	}

	// Parse action filter
	if action := c.Query("action"); action != "" {
		filters.Action = action
	}

	// Parse entity type filter
	if entityType := c.Query("entity_type"); entityType != "" {
		filters.EntityType = entityType
	}

	// Parse username filter
	if username := c.Query("username"); username != "" {
		filters.Username = username
	}

	// Parse status filter
	if status := c.Query("status"); status != "" {
		filters.Status = status
	}

	// Get logs with filters
	logs, totalCount, err := h.activityLogService.GetLogs(filters)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil activity logs: " + err.Error(),
		})
		return
	}

	// Check if data exceeds limit
	isLimited := false
	if totalCount > 1000 {
		isLimited = true
		// logs already limited to 1000 by filters.Limit
	}

	// Action translation map
	actionMap := map[string]string{
		"create": "Create",
		"update": "Update",
		"delete": "Delete",
		"upload": "Upload",
		"login":  "Login",
		"logout": "Logout",
		"view":   "View",
		"export": "Export",
	}

	// Entity type translation map
	entityMap := map[string]string{
		"pc":       "PC",
		"device":   "Device",
		"software": "Software",
		"logbook":  "Logbook",
		"user":     "User",
		"auth":     "Auth",
	}

	// Status translation map
	statusMap := map[string]string{
		"success": "Success",
		"failed":  "Failed",
		"error":   "Error",
	}

	// Transform data to [][]interface{}
	data := [][]interface{}{}
	for i, log := range logs {
		// Format datetime
		createdAt := log.CreatedAt.Format("02/01/2006 15:04:05")

		// Translate action
		action := actionMap[log.Action]
		if action == "" {
			action = log.Action
		}

		// Translate entity type
		entityType := entityMap[log.EntityType]
		if entityType == "" {
			entityType = log.EntityType
		}

		// Translate status
		status := statusMap[log.Status]
		if status == "" {
			status = log.Status
		}

		// Format entity ID
		entityID := "-"
		if log.EntityID != nil {
			entityID = strconv.Itoa(*log.EntityID)
		}

		// Handle empty values
		ipAddress := log.IPAddress
		if ipAddress == "" {
			ipAddress = "-"
		}

		row := []interface{}{
			i + 1,           // No
			createdAt,       // Date/Time
			log.Username,    // Username
			log.UserRole,    // Role
			action,          // Action
			entityType,      // Entity Type
			entityID,        // Entity ID
			log.Description, // Description
			status,          // Status
			ipAddress,       // IP Address
		}
		data = append(data, row)
	}

	// Prepare conditional formatting for status column (column I, index 8)
	conditionalFormats := []services.ConditionalFormat{}
	if len(data) > 0 {
		conditionalFormats = []services.ConditionalFormat{
			{
				Column:    "I",
				RowStart:  2,
				RowEnd:    len(data) + 1,
				Condition: "Success",
				Color:     "#92D050", // Green
			},
			{
				Column:    "I",
				RowStart:  2,
				RowEnd:    len(data) + 1,
				Condition: "Failed",
				Color:     "#FFC7CE", // Red
			},
			{
				Column:    "I",
				RowStart:  2,
				RowEnd:    len(data) + 1,
				Condition: "Error",
				Color:     "#FFEB9C", // Yellow
			},
		}
	}

	// Configure Excel export
	excelService := services.NewExcelService()
	config := services.ExcelExportConfig{
		SheetName: "Activity Logs",
		Headers: []string{
			"No", "Date/Time", "Username", "Role", "Action",
			"Entity Type", "Entity ID", "Description", "Status", "IP Address",
		},
		Data: data,
		ColumnWidths: map[string]float64{
			"A": 5,   // No
			"B": 18,  // Date/Time
			"C": 15,  // Username
			"D": 10,  // Role
			"E": 10,  // Action
			"F": 12,  // Entity Type
			"G": 10,  // Entity ID
			"H": 40,  // Description
			"I": 10,  // Status
			"J": 15,  // IP Address
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

	// Add warning note if data is limited
	if isLimited {
		sheetName := "Activity Logs"
		warningRow := len(data) + 3 // 2 rows after data
		warningMsg := "PERHATIAN: Data dibatasi 1,000 baris terakhir. Total data: " + strconv.Itoa(totalCount) + ". Gunakan filter untuk mempersempit data."
		f.SetCellValue(sheetName, "A"+strconv.Itoa(warningRow), warningMsg)
		
		// Style warning (bold, red)
		warningStyle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{
				Bold:  true,
				Color: "#FF0000",
			},
		})
		f.SetCellStyle(sheetName, "A"+strconv.Itoa(warningRow), "J"+strconv.Itoa(warningRow), warningStyle)
		f.MergeCell(sheetName, "A"+strconv.Itoa(warningRow), "J"+strconv.Itoa(warningRow))
	}

	// Generate filename: activity_log_export_HHMM_DDMMYYYY.xlsx
	filename := excelService.GenerateFilename("activity_log_export")

	// Set headers for download
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+filename)
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
