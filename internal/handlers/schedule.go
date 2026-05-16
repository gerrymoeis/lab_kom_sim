package handlers

import (
	"net/http"
	"time"

	"inventaris-lab-kom/internal/models"

	"github.com/gin-gonic/gin"
)

// dayNames returns the Indonesian day name for a time.Weekday
var dayNames = map[time.Weekday]string{
	time.Monday: "Senin", time.Tuesday: "Selasa", time.Wednesday: "Rabu",
	time.Thursday: "Kamis", time.Friday: "Jumat", time.Saturday: "Sabtu",
	time.Sunday: "Minggu",
}

var dayOrder = "CASE day WHEN 'Senin' THEN 1 WHEN 'Selasa' THEN 2 WHEN 'Rabu' THEN 3 WHEN 'Kamis' THEN 4 WHEN 'Jumat' THEN 5 WHEN 'Sabtu' THEN 6 ELSE 7 END"

func (h *Handler) ScheduleList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	dayFilter := c.DefaultQuery("day", "")
	search := c.Query("search")

	query := `SELECT id, course_name, lecturer, day, class, time_start, time_end, notes FROM course_schedules WHERE 1=1`
	var args []interface{}

	if dayFilter != "" {
		query += ` AND day = ?`
		args = append(args, dayFilter)
	}
	if search != "" {
		query += ` AND (course_name LIKE ? OR lecturer LIKE ? OR class LIKE ?)`
		s := "%" + search + "%"
		args = append(args, s, s, s)
	}

	query += ` ORDER BY ` + dayOrder + `, time_start`

	var schedules []models.CourseSchedule
	if r, e := h.db.Query(query, args...); e != nil {
		h.errHTML(c, "Gagal mengambil data jadwal"); return
	} else {
		defer r.Close()
		for r.Next() {
			var s models.CourseSchedule
			if r.Scan(&s.ID, &s.CourseName, &s.Lecturer, &s.Day, &s.Class, &s.TimeStart, &s.TimeEnd, &s.Notes) == nil {
				schedules = append(schedules, s)
			}
		}
	}

	c.HTML(http.StatusOK, "schedule/list.html", gin.H{
		"title": "Jadwal Mata Kuliah", "currentPage": "schedules",
		"username": username, "role": role,
		"schedules": schedules, "today": dayNames[time.Now().Weekday()],
		"dayFilter": dayFilter, "search": search,
		"days": []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"},
		"error": c.Query("error"),
	})
}

func (h *Handler) ScheduleCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}
	c.HTML(http.StatusOK, "schedule/create.html", gin.H{
		"title": "Tambah Jadwal", "currentPage": "schedules",
		"username": username, "role": role,
		"days": []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"},
	})
}

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
		h.logCreateError(c, "schedule", map[string]interface{}{"course_name": courseName}, err.Error())
		c.HTML(http.StatusInternalServerError, "schedule/create.html", gin.H{
			"title": "Tambah Jadwal", "error": "Gagal menyimpan data",
		})
		return
	}

	h.logCreate(c, "schedule", 0, map[string]interface{}{
		"course_name": courseName, "lecturer": lecturer, "day": day, "class": class,
		"time": timeStart + "-" + timeEnd,
	})
	c.Redirect(http.StatusFound, "/schedules")
}

func (h *Handler) ScheduleEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	id := c.Param("id")
	var s models.CourseSchedule
	if err := h.db.QueryRow(`SELECT id, course_name, lecturer, day, class, time_start, time_end, notes FROM course_schedules WHERE id = ?`, id).Scan(&s.ID, &s.CourseName, &s.Lecturer, &s.Day, &s.Class, &s.TimeStart, &s.TimeEnd, &s.Notes); err != nil {
		h.errHTML(c, "Jadwal tidak ditemukan")
		return
	}

	c.HTML(http.StatusOK, "schedule/edit.html", gin.H{
		"title": "Edit Jadwal", "currentPage": "schedules",
		"username": username, "role": role, "s": s,
		"days": []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"},
	})
}

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
		h.logUpdateError(c, "schedule", 0, map[string]interface{}{"id": id}, err.Error())
		h.errHTML(c, "Gagal mengupdate jadwal")
		return
	}

	h.logUpdate(c, "schedule", 0,
		map[string]interface{}{"id": id, "course_name": courseName},
		map[string]interface{}{"course_name": courseName, "lecturer": lecturer, "day": day, "class": class},
	)
	c.Redirect(http.StatusFound, "/schedules")
}

func (h *Handler) ScheduleDelete(c *gin.Context) {
	id := c.Param("id")

	var courseName string
	h.db.QueryRow(`SELECT course_name FROM course_schedules WHERE id = ?`, id).Scan(&courseName)

	_, err := h.db.Exec(`DELETE FROM course_schedules WHERE id = ?`, id)
	if err != nil {
		h.redirectWithError(c, "/schedules", "Gagal menghapus jadwal")
		return
	}

	h.logDelete(c, "schedule", 0, map[string]interface{}{"course_name": courseName})
	c.Redirect(http.StatusFound, "/schedules")
}
