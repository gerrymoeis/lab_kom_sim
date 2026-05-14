package handlers

import (
	"net/http"
	"time"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"

	"github.com/gin-gonic/gin"
)

// ScheduleList renders course schedule list with today highlight
func (h *Handler) ScheduleList(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	dayFilter := c.DefaultQuery("day", "")
	search := c.Query("search")

	query := `SELECT id, course_name, lecturer, day, class, time_start, time_end, notes FROM course_schedules WHERE 1=1`
	args := []interface{}{}

	if dayFilter != "" {
		query += ` AND day = ?`
		args = append(args, dayFilter)
	}
	if search != "" {
		query += ` AND (course_name LIKE ? OR lecturer LIKE ? OR class LIKE ?)`
		s := "%" + search + "%"
		args = append(args, s, s, s)
	}

	query += ` ORDER BY CASE day
		WHEN 'Senin' THEN 1 WHEN 'Selasa' THEN 2 WHEN 'Rabu' THEN 3
		WHEN 'Kamis' THEN 4 WHEN 'Jumat' THEN 5 WHEN 'Sabtu' THEN 6
		ELSE 7 END, time_start`

	rows, err := h.db.Query(query, args...)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title": "Error", "message": "Gagal mengambil data jadwal",
		})
		return
	}
	defer rows.Close()

	var schedules []models.CourseSchedule
	for rows.Next() {
		var s models.CourseSchedule
		if err := rows.Scan(&s.ID, &s.CourseName, &s.Lecturer, &s.Day, &s.Class, &s.TimeStart, &s.TimeEnd, &s.Notes); err == nil {
			schedules = append(schedules, s)
		}
	}

	todayName := map[time.Weekday]string{
		time.Monday: "Senin", time.Tuesday: "Selasa", time.Wednesday: "Rabu",
		time.Thursday: "Kamis", time.Friday: "Jumat", time.Saturday: "Sabtu",
		time.Sunday: "Minggu",
	}[time.Now().Weekday()]

	c.HTML(http.StatusOK, "schedule/list.html", gin.H{
		"title":       "Jadwal Mata Kuliah - Sistem Inventaris Lab",
		"currentPage": "schedules",
		"username":    username,
		"role":        role,
		"schedules":   schedules,
		"today":       todayName,
		"dayFilter":   dayFilter,
		"search":      search,
		"days":        []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"},
		"error":       c.Query("error"),
	})
}

// ScheduleCreatePage renders create form
func (h *Handler) ScheduleCreatePage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	c.HTML(http.StatusOK, "schedule/create.html", gin.H{
		"title":       "Tambah Jadwal - Sistem Inventaris Lab",
		"currentPage": "schedules",
		"username":    username,
		"role":        role,
		"days":        []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"},
	})
}

// ScheduleCreate handles create submit
func (h *Handler) ScheduleCreate(c *gin.Context) {
	courseName := c.PostForm("course_name")
	lecturer := c.PostForm("lecturer")
	day := c.PostForm("day")
	class := c.PostForm("class")
	timeStart := c.PostForm("time_start")
	timeEnd := c.PostForm("time_end")
	notes := c.PostForm("notes")

	if courseName == "" || lecturer == "" || day == "" || class == "" || timeStart == "" || timeEnd == "" {
		c.HTML(http.StatusBadRequest, "schedule/create.html", gin.H{
			"title": "Tambah Jadwal", "error": "Semua field wajib diisi",
		})
		return
	}

	_, err := h.db.Exec(`INSERT INTO course_schedules (course_name, lecturer, day, class, time_start, time_end, notes) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		courseName, lecturer, day, class, timeStart, timeEnd, notes)
	if err != nil {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogCreate(userID, username, role, "schedule", 0,
				map[string]interface{}{"course_name": courseName, "error": err.Error()},
				ipAddress, userAgent, err.Error())
		}
		c.HTML(http.StatusInternalServerError, "schedule/create.html", gin.H{
			"title": "Tambah Jadwal", "error": "Gagal menyimpan data",
		})
		return
	}

	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		h.activityLogService.LogCreate(userID, username, role, "schedule", 0,
			map[string]interface{}{"course_name": courseName, "lecturer": lecturer, "day": day, "class": class, "time": timeStart + "-" + timeEnd},
			ipAddress, userAgent)
	}

	c.Redirect(http.StatusFound, "/schedules")
}

// ScheduleEditPage renders edit form
func (h *Handler) ScheduleEditPage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	id := c.Param("id")
	var s models.CourseSchedule
	err := h.db.QueryRow(`SELECT id, course_name, lecturer, day, class, time_start, time_end, notes FROM course_schedules WHERE id = ?`, id).
		Scan(&s.ID, &s.CourseName, &s.Lecturer, &s.Day, &s.Class, &s.TimeStart, &s.TimeEnd, &s.Notes)
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"title": "Error", "message": "Jadwal tidak ditemukan"})
		return
	}

	c.HTML(http.StatusOK, "schedule/edit.html", gin.H{
		"title":       "Edit Jadwal - Sistem Inventaris Lab",
		"currentPage": "schedules",
		"username":    username,
		"role":        role,
		"s":           s,
		"days":        []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"},
	})
}

// ScheduleEdit handles edit submit
func (h *Handler) ScheduleEdit(c *gin.Context) {
	id := c.Param("id")
	courseName := c.PostForm("course_name")
	lecturer := c.PostForm("lecturer")
	day := c.PostForm("day")
	class := c.PostForm("class")
	timeStart := c.PostForm("time_start")
	timeEnd := c.PostForm("time_end")
	notes := c.PostForm("notes")

	_, err := h.db.Exec(`UPDATE course_schedules SET course_name=?, lecturer=?, day=?, class=?, time_start=?, time_end=?, notes=? WHERE id=?`,
		courseName, lecturer, day, class, timeStart, timeEnd, notes, id)
	if err != nil {
		userID, username, role, ok := middleware.GetCurrentUser(c)
		if ok {
			ipAddress, userAgent := getRequestContext(c)
			h.activityLogService.LogCreate(userID, username, role, "schedule", 0,
				map[string]interface{}{"action": "update", "id": id, "error": err.Error()},
				ipAddress, userAgent, err.Error())
		}
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"title": "Error", "message": "Gagal mengupdate jadwal"})
		return
	}

	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		h.activityLogService.LogUpdate(userID, username, role, "schedule", 0,
			map[string]interface{}{"id": id, "course_name": courseName},
			map[string]interface{}{"course_name": courseName, "lecturer": lecturer, "day": day, "class": class},
			ipAddress, userAgent)
	}

	c.Redirect(http.StatusFound, "/schedules")
}

// ScheduleDelete handles delete
func (h *Handler) ScheduleDelete(c *gin.Context) {
	id := c.Param("id")

	// Get name for logging
	var courseName string
	h.db.QueryRow(`SELECT course_name FROM course_schedules WHERE id = ?`, id).Scan(&courseName)

	_, err := h.db.Exec(`DELETE FROM course_schedules WHERE id = ?`, id)
	if err != nil {
		c.Redirect(http.StatusFound, "/schedules?error=Gagal menghapus jadwal")
		return
	}

	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		h.activityLogService.LogDelete(userID, username, role, "schedule", 0,
			map[string]interface{}{"course_name": courseName}, ipAddress, userAgent)
	}

	c.Redirect(http.StatusFound, "/schedules")
}
