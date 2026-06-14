package handlers

import (
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"inventaris-lab-kom/internal/services"
	"inventaris-lab-kom/internal/timeutil"

	"github.com/gin-gonic/gin"
)

var dayNames = map[time.Weekday]string{
	time.Monday: "Senin", time.Tuesday: "Selasa", time.Wednesday: "Rabu",
	time.Thursday: "Kamis", time.Friday: "Jumat", time.Saturday: "Sabtu",
	time.Sunday: "Minggu",
}

func (h *Handler) ScheduleList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 { page = 1 }
	pageSize := h.cfg.DefaultPageSize

	values, _ := url.ParseQuery(c.Request.URL.RawQuery)
	delete(values, "page")
	var query interface{} = ""
	if len(values) > 0 { query = template.URL("&" + values.Encode()) }

	dayFilter := c.DefaultQuery("day", "")
	search := c.Query("search")
	sortBy := c.Query("sort_by")

	schedules, total, err := h.scheduleService.ListPaginated(search, dayFilter, sortBy, page, pageSize)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data jadwal"); return
	}

	totalPages := (total + pageSize - 1) / pageSize
	startRow := (page-1)*pageSize + 1

	h.renderTemplate(c, http.StatusOK, "schedule/list.html", gin.H{
		"title": "Jadwal Mata Kuliah", "currentPage": "schedules",
		"username": username, "role": role,
		"schedules": schedules, "today": dayNames[time.Now().In(timeutil.Location()).Weekday()],
		"filters": gin.H{"search": search, "day": dayFilter, "sort_by": sortBy},
		"days": []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"},
		"page": page, "startRow": startRow, "totalPages": totalPages, "totalItems": total,
		"query": query,
	})
}

func (h *Handler) ScheduleCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	h.renderTemplate(c, http.StatusOK, "schedule/create.html", gin.H{
		"title": "Tambah Jadwal", "currentPage": "schedules",
		"username": username, "role": role,
		"days": []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"},
	})
}

func (h *Handler) ScheduleCreate(c *gin.Context) {
	var req CreateScheduleRequest
	if err := c.ShouldBind(&req); err != nil {
		h.renderTemplate(c, http.StatusBadRequest, "schedule/create.html", gin.H{
			"title": "Tambah Jadwal", "error": "Semua field wajib diisi",
		})
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	err := h.scheduleService.Create(services.ScheduleCreateInput{
		CourseName: req.CourseName, Lecturer: req.Lecturer, Day: req.Day,
		Class: req.Class, TimeStart: req.TimeStart, TimeEnd: req.TimeEnd, Notes: req.Notes,
	}, uid, u, r, ip, ua)
	if err != nil {
		h.renderTemplate(c, http.StatusInternalServerError, "schedule/create.html", gin.H{
			"title": "Tambah Jadwal", "error": "Gagal menyimpan data",
		})
		return
	}
	h.redirectWithSuccess(c, "/schedules", "Jadwal berhasil ditambahkan")
}

func (h *Handler) ScheduleEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id, _ := strconv.Atoi(c.Param("id"))
	s, err := h.scheduleService.GetByID(id)
	if err != nil {
		h.errHTML(c, "Jadwal tidak ditemukan")
		return
	}

	h.renderTemplate(c, http.StatusOK, "schedule/edit.html", gin.H{
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

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	err := h.scheduleService.Update(id, services.ScheduleUpdateInput{
		CourseName: req.CourseName, Lecturer: req.Lecturer, Day: req.Day,
		Class: req.Class, TimeStart: req.TimeStart, TimeEnd: req.TimeEnd, Notes: req.Notes,
	}, uid, u, r, ip, ua)
	if err != nil {
		h.errHTML(c, "Gagal mengupdate jadwal")
		return
	}
	h.redirectWithSuccess(c, "/schedules", "Jadwal berhasil diperbarui", "update")
}

func (h *Handler) ScheduleDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	if err := h.scheduleService.Delete(id, uid, u, r, ip, ua); err != nil {
		h.redirectWithError(c, "/schedules", "Gagal menghapus jadwal")
		return
	}
	h.redirectWithSuccess(c, "/schedules", "Jadwal berhasil dihapus", "delete")
}

func (h *Handler) ScheduleBatchDelete(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		h.errJSON(c, http.StatusBadRequest, "Tidak ada item yang dipilih")
		return
	}
	intIDs, err := parseInt64IDs(req.IDs)
	if err != nil {
		h.errJSON(c, http.StatusBadRequest, err.Error())
		return
	}
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	if err := h.scheduleService.BatchDelete(intIDs, uid, u, r, ip, ua); err != nil {
		h.errJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Jadwal berhasil dihapus"})
}
