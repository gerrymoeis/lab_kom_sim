package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"inventaris-lab-kom/internal/repository"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

func (h *Handler) LogbookList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 50 }

	entries, total, err := h.logbookService.List(repository.LogbookFilters{
		Search: c.Query("search"), Page: page, PageSize: pageSize,
	})
	if err != nil { h.errHTML(c, "Gagal mengambil data logbook"); return }

	totalPages := (total + pageSize - 1) / pageSize
	c.HTML(http.StatusOK, "logbook/list.html", gin.H{
		"title": "Logbook", "currentPage": "logbook",
		"username": username, "role": role,
		"entries": entries, "page": page,
		"totalPages": totalPages, "search": c.Query("search"),
		"date": c.Query("date"), "success": c.Query("success"),
	})
}

func (h *Handler) LogbookUploadPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	c.HTML(http.StatusOK, "logbook/upload.html", gin.H{
		"title": "Upload Logbook", "currentPage": "logbook",
		"username": username, "role": role,
	})
}

func (h *Handler) LogbookUpload(c *gin.Context) {
	userID, username, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat mengupload"); return }
	ip, ua := getRequestContext(c)

	var path, fn string

	var uploadReq struct {
		FileRef string `form:"file_ref"`
	}
	c.ShouldBind(&uploadReq)
	fileRef := strings.TrimSpace(uploadReq.FileRef)
	if fileRef != "" {
		fn = fileRef
		tempPath := filepath.Join("uploads", "temp", fn)
		path = filepath.Join("uploads", "logbook", fn)
		os.MkdirAll(filepath.Dir(path), 0755)
		if err := services.CopyFile(tempPath, path); err != nil {
			h.errHTML(c, "Gagal memproses file: file tidak ditemukan")
			return
		}
		os.Remove(tempPath)
	} else {
		file, err := c.FormFile("logbook_image")
		if err != nil { h.errHTML(c, "Gagal mengambil file"); return }
		ext := strings.ToLower(filepath.Ext(file.Filename))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".heic" && ext != ".heif" {
			h.errHTML(c, "Format file tidak didukung"); return
		}
		fn = fmt.Sprintf("logbook_%d%s", time.Now().Unix(), ext)
		tempPath := filepath.Join("uploads", "temp", fn)
		os.MkdirAll(filepath.Dir(tempPath), 0755)
		if err := c.SaveUploadedFile(file, tempPath); err != nil { h.errHTML(c, "Gagal menyimpan file"); return }
		path = tempPath
	}

	// Ensure temp file cleanup after OCR processing
	if fileRef == "" {
		defer os.Remove(path)
	}

	ocr := services.NewOCRService(h.cfg.GeminiAPIKey, h.cfg.OpenRouterAPIKey)
	result, err := ocr.ExtractLogbookFromImage(path)
	if err != nil {
		h.errHTML(c, "Gagal memproses gambar: "+err.Error())
		return
	}

	h.activityLogService.LogUpload(userID, username, role, "logbook", 0, fn, "image", ip, ua)

	c.HTML(http.StatusOK, "logbook/preview.html", gin.H{
		"title": "Upload Logbook", "currentPage": "logbook",
		"username": username, "role": role,
		"entries": result.Entries, "total": len(result.Entries),
		"source_file": fn, "success": "Gambar berhasil diproses",
	})
}

func (h *Handler) LogbookSave(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errJSON(c, http.StatusForbidden, "Hanya admin"); return }

	var req LogbookSaveRequest
	c.ShouldBind(&req)

	bulk := make([]repository.BulkEntry, 0, len(req.Date))
	for i := 0; i < len(req.Date) && i < len(req.StudentName); i++ {
		dv, err1 := services.ParseDate(req.Date[i])
		tiv, err2 := time.Parse("15:04", req.TimeIn[i])
		tov, err3 := time.Parse("15:04", req.TimeOut[i])
		if err1 != nil || err2 != nil || err3 != nil { continue }

		p := ""
		if i < len(req.Purpose) { p = req.Purpose[i] }
		if p != "" { p = services.ToTitleCaseWithAbbr(p) }
		bulk = append(bulk, repository.BulkEntry{
			Date: dv, StudentName: req.StudentName[i], NIM: req.NIM[i],
			TimeIn: tiv.Format("15:04"), TimeOut: tov.Format("15:04"),
			Purpose: p,
		})
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	saved, dups, err := h.logbookService.BulkSave(bulk, req.SourceFile, uid, u, r, ip, ua)
	if err != nil {
		h.errJSON(c, http.StatusInternalServerError, "Gagal menyimpan data")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true, "saved": saved, "duplicates": dups,
		"message": fmt.Sprintf("Berhasil menyimpan %d data.", saved),
	})
}

func (h *Handler) LogbookExport(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat export data"); return }

	search := c.Query("search")
	date := c.Query("date")

	entries, _, _ := h.logbookService.List(repository.LogbookFilters{
		Search: search, StartDate: date, EndDate: date, PageSize: 10000,
	})

	svc := services.NewExcelService()
	data := make([][]any, 0, len(entries))
	for _, e := range entries {
		data = append(data, []any{e.Date.Format("2006-01-02"), e.StudentName, e.NIM, e.TimeIn, e.TimeOut, e.Purpose})
	}
	f, _ := svc.GenerateMultiSheetExcel([]services.ExcelExportConfig{
		{
			SheetName: "Logbook",
			Headers:   []string{"Tanggal", "Nama", "NIM", "Jam Masuk", "Jam Keluar", "Keperluan"},
			Data:      data,
			ColumnWidths: map[string]float64{"A": 14, "B": 28, "C": 18, "D": 12, "E": 12, "F": 36},
		},
	})
	defer f.Close()

	fn := svc.GenerateFilename("logbook_export")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+fn)
	f.Write(c.Writer)
}

func (h *Handler) LogbookExportPreview(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat export data"); return }

	filterDate := c.Query("date")
	search := c.Query("search")

	entries, total, _ := h.logbookService.List(repository.LogbookFilters{
		Search: search, StartDate: filterDate, EndDate: filterDate, PageSize: 10000,
	})

	svc := services.NewExcelService()
	data := make([][]any, 0, len(entries))
	for _, e := range entries {
		data = append(data, []any{e.Date.Format("2006-01-02"), e.StudentName, e.NIM, e.TimeIn, e.TimeOut, e.Purpose})
	}

	f, _ := svc.GenerateMultiSheetExcel([]services.ExcelExportConfig{
		{
			SheetName: "Logbook",
			Headers:   []string{"Tanggal", "Nama", "NIM", "Jam Masuk", "Jam Keluar", "Keperluan"},
			Data:      data,
			ColumnWidths: map[string]float64{"A": 14, "B": 28, "C": 18, "D": 12, "E": 12, "F": 36},
		},
	})
	defer f.Close()

	fn := svc.GenerateFilename("logbook_export_preview")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+fn)
	f.Write(c.Writer)
	_ = total
}

func (h *Handler) LogbookCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	c.HTML(http.StatusOK, "logbook/create.html", gin.H{
		"title": "Tambah Logbook", "currentPage": "logbook",
		"username": username, "role": role,
	})
}

func (h *Handler) LogbookCreate(c *gin.Context) {
	var req CreateLogbookRequest
	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "logbook/create.html", gin.H{
			"title": "Tambah Logbook", "error": "Lengkapi data yang diperlukan",
		})
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	id, err := h.logbookService.CreateEntry(services.CreateLogbookInput{
		Date: req.Date, StudentName: req.StudentName, NIM: req.NIM,
		TimeIn: req.TimeIn, TimeOut: req.TimeOut, Purpose: req.Purpose,
	}, uid, u, r, ip, ua)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "logbook/create.html", gin.H{
			"title": "Tambah Logbook", "error": "Gagal menyimpan data",
		})
		return
	}
	if id == 0 {
		c.HTML(http.StatusBadRequest, "logbook/create.html", gin.H{
			"title": "Tambah Logbook", "error": "Data sudah ada (duplikat)",
		})
		return
	}
	c.Redirect(http.StatusFound, "/logbook")
}

func (h *Handler) LogbookEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id, _ := strconv.Atoi(c.Param("id"))
	entry, err := h.logbookService.GetByID(id)
	if err != nil { h.errHTML(c, "Data tidak ditemukan"); return }

	c.HTML(http.StatusOK, "logbook/edit.html", gin.H{
		"title": "Edit Logbook", "currentPage": "logbook",
		"username": username, "role": role, "entry": entry,
	})
}

func (h *Handler) LogbookEdit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req EditLogbookRequest
	if err := c.ShouldBind(&req); err != nil {
		h.errHTML(c, "Data tidak valid")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.logbookService.UpdateEntry(id, services.UpdateLogbookInput{
		Date: req.Date, StudentName: req.StudentName, NIM: req.NIM,
		TimeIn: req.TimeIn, TimeOut: req.TimeOut, Purpose: req.Purpose,
	}, uid, u, r, ip, ua); err != nil {
		h.errHTML(c, "Gagal mengupdate data")
		return
	}
	c.Redirect(http.StatusFound, "/logbook")
}

func (h *Handler) LogbookDelete(c *gin.Context) {
	// Called via form POST (redirect) or AJAX
	id, _ := strconv.Atoi(c.Param("id"))
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.logbookService.DeleteEntry(id, uid, u, r, ip, ua); err != nil {
		if c.GetHeader("Accept") == "application/json" {
			h.errJSON(c, http.StatusInternalServerError, "Gagal menghapus data")
		} else {
			h.redirectWithError(c, "/logbook", "Gagal menghapus data")
		}
		return
	}

	if c.GetHeader("Accept") == "application/json" {
		c.JSON(http.StatusOK, gin.H{"success": true})
	} else {
		c.Redirect(http.StatusFound, "/logbook")
	}
}
