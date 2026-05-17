package handlers

import (
	"net/http"
	"strconv"
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

func (h *Handler) ScheduleList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	dayFilter := c.DefaultQuery("day", "")
	search := c.Query("search")

	schedules, err := h.scheduleRepo.List(search)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data jadwal"); return
	}

	if dayFilter != "" {
		var filtered []models.CourseSchedule
		for _, s := range schedules {
			if s.Day == dayFilter {
				filtered = append(filtered, s)
			}
		}
		schedules = filtered
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
	var req CreateScheduleRequest
	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "schedule/create.html", gin.H{
			"title": "Tambah Jadwal", "error": "Semua field wajib diisi",
		})
		return
	}

	_, err := h.scheduleRepo.Create(req.CourseName, req.Lecturer, req.Day, req.Class, req.TimeStart, req.TimeEnd, req.Notes)
	if err != nil {
		h.logCreateError(c, "schedule", map[string]interface{}{"course_name": req.CourseName}, err.Error())
		c.HTML(http.StatusInternalServerError, "schedule/create.html", gin.H{
			"title": "Tambah Jadwal", "error": "Gagal menyimpan data",
		})
		return
	}

	h.logCreate(c, "schedule", 0, map[string]interface{}{
		"course_name": req.CourseName, "lecturer": req.Lecturer, "day": req.Day, "class": req.Class,
		"time": req.TimeStart + "-" + req.TimeEnd,
	})
	c.Redirect(http.StatusFound, "/schedules")
}

func (h *Handler) ScheduleEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	s, err := h.scheduleRepo.GetByID(id)
	if err != nil {
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
	id, _ := strconv.Atoi(c.Param("id"))
	var req EditScheduleRequest
	if err := c.ShouldBind(&req); err != nil {
		h.errHTML(c, "Semua field wajib diisi")
		return
	}

	err := h.scheduleRepo.Update(id, req.CourseName, req.Lecturer, req.Day, req.Class, req.TimeStart, req.TimeEnd, req.Notes)
	if err != nil {
		h.logUpdateError(c, "schedule", 0, map[string]interface{}{"id": id}, err.Error())
		h.errHTML(c, "Gagal mengupdate jadwal")
		return
	}

	h.logUpdate(c, "schedule", 0,
		map[string]interface{}{"id": id, "course_name": req.CourseName},
		map[string]interface{}{"course_name": req.CourseName, "lecturer": req.Lecturer, "day": req.Day, "class": req.Class},
	)
	c.Redirect(http.StatusFound, "/schedules")
}

func (h *Handler) ScheduleDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	courseName, _ := h.scheduleRepo.GetCourseName(id)

	err := h.scheduleRepo.Delete(id)
	if err != nil {
		h.redirectWithError(c, "/schedules", "Gagal menghapus jadwal")
		return
	}

	h.logDelete(c, "schedule", 0, map[string]interface{}{"course_name": courseName})
	c.Redirect(http.StatusFound, "/schedules")
}
