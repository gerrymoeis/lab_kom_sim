package handlers

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

func (h *Handler) ActivityLogList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 { page = 1 }
	pageSize := 50

	values, _ := url.ParseQuery(c.Request.URL.RawQuery)
	delete(values, "page")
	query := ""
	if len(values) > 0 { query = "&" + values.Encode() }

	filters := services.ActivityLogFilters{Limit: pageSize, Offset: (page - 1) * pageSize}

	if d := c.Query("date_from"); d != "" { if t, err := services.ParseDate(d); err == nil { filters.DateFrom = &t } }
	if d := c.Query("date_to"); d != "" { if t, err := services.ParseDate(d); err == nil { eod := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second); filters.DateTo = &eod } }
	if a := c.Query("action"); a != "" { filters.Action = a }
	if e := c.Query("entity_type"); e != "" { filters.EntityType = e }
	if u := c.Query("username"); u != "" { filters.Username = u }
	if s := c.Query("status"); s != "" { filters.Status = s }

	keyword := c.Query("search")

	var alogs []models.ActivityLog
	var totalCount int

	if keyword != "" {
		r, total, err := h.activityLogService.SearchLogs(keyword, pageSize, (page-1)*pageSize)
		if err != nil { h.errHTML(c, "Gagal mencari activity logs"); return }
		alogs = r
		totalCount = total
	} else {
		r, total, err := h.activityLogService.GetLogs(filters)
		if err != nil { h.errHTML(c, "Gagal mengambil activity logs"); return }
		alogs = r
		totalCount = total
	}

	totalPages := (totalCount + pageSize - 1) / pageSize
	if totalPages < 1 { totalPages = 1 }

	usernames, _ := h.activityLogService.GetUsernames()

	filterMap := gin.H{
		"date_from": c.Query("date_from"), "date_to": c.Query("date_to"),
		"action": c.Query("action"), "entity_type": c.Query("entity_type"),
		"username": c.Query("username"), "status": c.Query("status"),
		"search": keyword,
	}

	c.HTML(http.StatusOK, "activity_log/list.html", gin.H{
		"title": "Activity Logs", "currentPage": "activity_logs",
		"username": username, "role": role,
		"logs":       alogs,
		"page":       page,
		"totalPages": totalPages,
		"totalItems": totalCount,
		"query":      query,
		"filters":    filterMap,
		"usernames":  usernames,
	})
}

func (h *Handler) ActivityLogExport(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat export"); return }

	filters := services.ActivityLogFilters{Limit: 1000, Offset: 0}
	if d := c.Query("date_from"); d != "" { if t, err := services.ParseDate(d); err == nil { filters.DateFrom = &t } }
	if d := c.Query("date_to"); d != "" { if t, err := services.ParseDate(d); err == nil { eod := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second); filters.DateTo = &eod } }
	if a := c.Query("action"); a != "" { filters.Action = a }
	if e := c.Query("entity_type"); e != "" { filters.EntityType = e }
	if u := c.Query("username"); u != "" { filters.Username = u }
	if s := c.Query("status"); s != "" { filters.Status = s }

	logs, totalCount, err := h.activityLogService.GetLogs(filters)
	if err != nil { h.errHTML(c, "Gagal mengambil activity logs"); return }

	actionMap := map[string]string{"create": "Create", "update": "Update", "delete": "Delete", "upload": "Upload", "login": "Login", "logout": "Logout", "view": "View", "export": "Export"}
	entityMap := map[string]string{"pc": "PC", "device": "Device", "software": "Software", "logbook": "Logbook", "user": "User", "auth": "Auth", "device_loan": "Device Loan", "device_usage": "Device Usage", "schedule": "Schedule"}
	statusMap := map[string]string{"success": "Success", "failed": "Failed", "error": "Error"}

	data := [][]any{}
	for i, l := range logs {
		action := actionMap[l.Action]; if action == "" { action = l.Action }
		entity := entityMap[l.EntityType]; if entity == "" { entity = l.EntityType }
		status := statusMap[l.Status]; if status == "" { status = l.Status }
		eid := "-"
		if l.EntityID != nil { eid = strconv.Itoa(*l.EntityID) }
		ip := l.IPAddress; if ip == "" { ip = "-" }

		data = append(data, []any{
			i + 1, l.CreatedAt.Format("02/01/2006 15:04:05"),
			l.Username, l.UserRole, action, entity, eid, l.Description, status, ip,
		})
	}

	svc := services.NewExcelService()
	cf := []services.ConditionalFormat{}
	if len(data) > 0 {
		cf = []services.ConditionalFormat{
			{Column: "I", RowStart: 2, RowEnd: len(data) + 1, Condition: "Success", Color: "#92D050"},
			{Column: "I", RowStart: 2, RowEnd: len(data) + 1, Condition: "Failed", Color: "#FFC7CE"},
			{Column: "I", RowStart: 2, RowEnd: len(data) + 1, Condition: "Error", Color: "#FFEB9C"},
		}
	}

	f, _ := svc.GenerateExcel(services.ExcelExportConfig{
		SheetName: "Activity Logs",
		Headers:   []string{"No", "Date/Time", "Username", "Role", "Action", "Entity Type", "Entity ID", "Description", "Status", "IP Address"},
		Data:      data,
		ColumnWidths: map[string]float64{"A": 5, "B": 18, "C": 15, "D": 10, "E": 10, "F": 15, "G": 10, "H": 40, "I": 10, "J": 15},
		ConditionalFormats: cf,
	})
	defer f.Close()

	if totalCount > 1000 {
		row := len(data) + 3
		f.SetCellValue("Activity Logs", "A"+strconv.Itoa(row), "PERHATIAN: Data dibatasi 1,000 baris. Total: "+strconv.Itoa(totalCount))
		s, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "#FF0000"}})
		f.SetCellStyle("Activity Logs", "A"+strconv.Itoa(row), "J"+strconv.Itoa(row), s)
		f.MergeCell("Activity Logs", "A"+strconv.Itoa(row), "J"+strconv.Itoa(row))
	}

	fn := svc.GenerateFilename("activity_log_export")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+fn)
	f.Write(c.Writer)
}
