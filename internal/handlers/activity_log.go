package handlers

import (
	"net/http"
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

	cursor := c.Query("cursor")
	direction := c.DefaultQuery("dir", "next")
	limit := 50

	cursorID, cursorCreatedAt := parseActivityCursor(cursor)
	filters := services.ActivityLogFilters{
		Limit: limit, CursorID: cursorID, CursorCreatedAt: cursorCreatedAt, Direction: direction,
	}

	if d := c.Query("date_from"); d != "" { if t, err := services.ParseDate(d); err == nil { filters.DateFrom = &t } }
	if d := c.Query("date_to"); d != "" { if t, err := services.ParseDate(d); err == nil { eod := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second); filters.DateTo = &eod } }
	if a := c.Query("action"); a != "" { filters.Action = a }
	if e := c.Query("entity_type"); e != "" { filters.EntityType = e }
	if u := c.Query("username"); u != "" { filters.Username = u }
	if s := c.Query("status"); s != "" { filters.Status = s }

	keyword := c.Query("search")
	searchOffset, _ := strconv.Atoi(c.Query("search_offset"))

	var alogs []models.ActivityLog
	var hasMore bool
	var prevCursor, nextCursor string

	if keyword != "" {
		r, _, err := h.activityLogService.SearchLogs(keyword, limit+1, searchOffset)
		if err != nil { h.errHTML(c, "Gagal mencari activity logs"); return }
		if len(r) > limit {
			hasMore = true
			r = r[:limit]
		}
		alogs = r
		if searchOffset > 0 {
			prevOffset := searchOffset - limit
			if prevOffset < 0 { prevOffset = 0 }
			prevCursor = strconv.Itoa(prevOffset)
		}
		if hasMore {
			nextCursor = strconv.Itoa(searchOffset + limit)
		}
	} else {
		r, hMore, err := h.activityLogService.GetLogsCursor(filters)
		if err != nil { h.errHTML(c, "Gagal mengambil activity logs"); return }
		alogs = r
		hasMore = hMore
		if len(alogs) > 0 {
			if cursor != "" {
				prevCursor = buildActivityCursor(alogs[0])
			}
			if hasMore {
				nextCursor = buildActivityCursor(alogs[len(alogs)-1])
			}
		}
	}

	usernames, _ := h.activityLogService.GetUsernames()

	searchMode := keyword != ""
	filterMap := gin.H{
		"date_from": c.Query("date_from"), "date_to": c.Query("date_to"),
		"action": c.Query("action"), "entity_type": c.Query("entity_type"),
		"username": c.Query("username"), "status": c.Query("status"),
		"search": keyword,
	}
	if searchMode {
		filterMap["search_offset"] = searchOffset
	}

	c.HTML(http.StatusOK, "activity_log/list.html", gin.H{
		"title": "Activity Logs", "currentPage": "activity_logs",
		"username": username, "role": role,
		"logs":       alogs,
		"hasMore":    hasMore,
		"prevCursor": prevCursor,
		"nextCursor": nextCursor,
		"searchMode": searchMode,
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
