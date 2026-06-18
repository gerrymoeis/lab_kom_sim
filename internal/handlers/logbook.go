package handlers

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

func (h *Handler) LogbookList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	pageSize, _ := strconv.Atoi(c.DefaultQuery("size", strconv.Itoa(h.cfg.DefaultPageSize)))
	if pageSize < 1 { pageSize = h.cfg.DefaultPageSize }

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 { page = 1 }

	sortBy := c.DefaultQuery("sort_by", "date")
	sortOrder := c.DefaultQuery("sort_order", "ASC")

	f := repository.LogbookFilters{
		StartDate: c.Query("date_from"),
		EndDate:   c.Query("date_to"),
		Search:    c.Query("search"),
		SortBy:    sortBy,
		SortOrder: sortOrder,
		PageSize:  pageSize,
		Page:      page,
	}

	dupMode := c.Query("dup") == "1"

	allEntries, errAll := h.logbookService.ListAll(f)
	dupFlags := make(map[int]bool)
	if errAll == nil {
		for i := 0; i < len(allEntries); i++ {
			if dupFlags[allEntries[i].ID] {
				continue
			}
			for j := i + 1; j < len(allEntries); j++ {
				if !allEntries[i].Date.Equal(allEntries[j].Date) {
					continue
				}
				if services.IsDuplicateEntry(allEntries[i].Date, allEntries[j].Date,
					allEntries[i].TimeIn, allEntries[j].TimeIn,
					allEntries[i].StudentName, allEntries[j].StudentName,
					allEntries[i].NIM, allEntries[j].NIM, config.DefaultDuplicateConfig) {
					dupFlags[allEntries[i].ID] = true
					dupFlags[allEntries[j].ID] = true
				}
			}
		}
	}

	totalDupCount := 0
	if errAll == nil {
		for _, e := range allEntries {
			if dupFlags[e.ID] {
				totalDupCount++
			}
		}
	}

	var entries []models.LogbookEntry
	total := 0

	if dupMode && errAll == nil {
		var dupEntries []models.LogbookEntry
		for _, e := range allEntries {
			if dupFlags[e.ID] {
				dupEntries = append(dupEntries, e)
			}
		}
		total = len(dupEntries)
		start := (page - 1) * pageSize
		if start < 0 {
			start = 0
		}
		if start > total {
			start = total
		}
		end := start + pageSize
		if end > total {
			end = total
		}
		entries = dupEntries[start:end]
	} else {
		var listErr error
		entries, total, listErr = h.logbookService.List(f)
		if listErr != nil {
			h.errHTML(c, "Gagal mengambil data logbook")
			return
		}
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	startRow := (page-1)*pageSize + 1
	if startRow < 1 {
		startRow = 1
	}

	values, _ := url.ParseQuery(c.Request.URL.RawQuery)
	delete(values, "page")
	values.Del("dup")
	values.Del("success")
	values.Del("error")
	values.Del("toast")
	var queryBack interface{} = ""
	if len(values) > 0 {
		queryBack = template.URL("&" + values.Encode())
	}

	valuesDup, _ := url.ParseQuery(c.Request.URL.RawQuery)
	delete(valuesDup, "page")
	valuesDup.Set("dup", "1")
	valuesDup.Del("success")
	valuesDup.Del("error")
	valuesDup.Del("toast")
	var queryDup interface{} = ""
	if len(valuesDup) > 0 {
		queryDup = template.URL("&" + valuesDup.Encode())
	}

	var query interface{} = queryBack
	if dupMode {
		query = queryDup
	}

	h.renderTemplate(c, http.StatusOK, "logbook/list.html", gin.H{
		"title": "Logbook", "currentPage": "logbook",
		"username": username, "role": role,
		"entries":    entries,
		"dupFlags":   dupFlags,
		"dupCount":   totalDupCount,
		"total":      total,
		"page":       page,
		"totalPages": totalPages,
		"startRow":   startRow,
		"query":      query,
		"queryBack":  queryBack,
		"queryDup":   queryDup,
		"dupMode":    dupMode,
		"filters": gin.H{
			"date_from": f.StartDate, "date_to": f.EndDate,
			"search": f.Search, "sort_by": sortBy, "sort_order": sortOrder,
		},
		"pageSize": pageSize,
	})
}

func (h *Handler) LogbookDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id, _ := strconv.Atoi(c.Param("id"))
	entry, err := h.logbookService.GetByID(id)
	if err != nil {
		h.errHTML(c, "Entry tidak ditemukan")
		return
	}

	h.renderTemplate(c, http.StatusOK, "logbook/detail.html", gin.H{
		"title": "Detail Logbook", "currentPage": "logbook",
		"username": username, "role": role, "entry": entry,
	})
}

func (h *Handler) LogbookUploadPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	h.renderTemplate(c, http.StatusOK, "logbook/upload.html", gin.H{
		"title": "Upload Logbook", "currentPage": "logbook",
		"username": username, "role": role,
		"android": h.cfg.Android,
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
	if err := c.ShouldBind(&uploadReq); err != nil {
		h.errHTML(c, "Data tidak valid")
		return
	}
	fileRef := strings.TrimSpace(uploadReq.FileRef)

	lab := c.GetString("lab")
	if fileRef != "" {
		fn = filepath.Base(fileRef)
		if fn == "" || fn == "." || fn == "/" || fn == "\\" {
			h.errHTML(c, "Nama file tidak valid")
			return
		}
		tempPath := filepath.Join(h.cfg.UploadPath, lab, "temp", fn)
		path = filepath.Join(h.cfg.UploadPath, lab, "logbook", fn)
		os.MkdirAll(filepath.Dir(path), 0755)
		if err := services.CopyFile(tempPath, path); err != nil {
			h.errHTML(c, "Gagal memproses file: file tidak ditemukan")
			return
		}
		os.Remove(tempPath)
	} else {
		file, err := c.FormFile("logbook_image")
		if err != nil {
			h.errHTML(c, "Gagal mengambil file"); return
		}
		if file.Size > 10*1024*1024 {
			h.errHTML(c, "File terlalu besar (max 10MB)"); return
		}
		ext := strings.ToLower(filepath.Ext(file.Filename))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".heic" && ext != ".heif" {
			h.errHTML(c, "Format file tidak didukung"); return
		}
		lf, err := file.Open()
		if err != nil {
			h.errHTML(c, "Gagal membaca file"); return
		}
		buf := make([]byte, 512)
		if _, err := lf.Read(buf); err != nil && err != io.EOF {
			lf.Close()
			h.errHTML(c, "Gagal membaca file"); return
		}
		lf.Close()
		mimeType := http.DetectContentType(buf)
		if !strings.HasPrefix(mimeType, "image/") {
			h.errHTML(c, "File harus berupa gambar"); return
		}
		fn = fmt.Sprintf("logbook_%d%s", time.Now().Unix(), ext)
		tempPath := filepath.Join(h.cfg.UploadPath, lab, "temp", fn)
		os.MkdirAll(filepath.Dir(tempPath), 0755)
		if err := c.SaveUploadedFile(file, tempPath); err != nil {
			h.errHTML(c, "Gagal menyimpan file"); return
		}
		logbookPath := filepath.Join(h.cfg.UploadPath, lab, "logbook", fn)
		os.MkdirAll(filepath.Dir(logbookPath), 0755)
		if err := services.CopyFile(tempPath, logbookPath); err != nil {
			os.Remove(tempPath)
			h.errHTML(c, "Gagal menyimpan file"); return
		}
		path = tempPath
	}

	// Clean up temp file (logbook copy persists for preview)
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

	var modelEntries []models.LogbookEntry
	for _, e := range result.Entries {
		parsed, _ := services.ParseDate(e.Date)
		modelEntries = append(modelEntries, models.LogbookEntry{
			Date: parsed, StudentName: e.StudentName,
			NIM: e.NIM, TimeIn: e.TimeIn, TimeOut: e.TimeOut, Purpose: e.Purpose,
		})
	}
	dupGroups := h.logbookService.CheckDuplicates(modelEntries)

	type dupItem struct {
		GroupID string
		Type    string
		Refs    []services.DuplicateReference
	}
	dupInfo := make([]*dupItem, len(result.Entries))
	for _, g := range dupGroups {
		for _, m := range g.Members {
			dupInfo[m] = &dupItem{GroupID: g.GroupID, Type: g.Type, Refs: g.References}
		}
	}

	h.renderTemplate(c, http.StatusOK, "logbook/preview.html", gin.H{
		"title": "Upload Logbook", "currentPage": "logbook",
		"username": username, "role": role,
		"entries": result.Entries, "total": len(result.Entries),
		"source_file": fn, "success": "Gambar berhasil diproses",
		"dupInfo": dupInfo,
	})
}

func (h *Handler) LogbookSave(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errJSON(c, http.StatusForbidden, "Hanya admin"); return }

	var req LogbookSaveRequest
	if err := c.ShouldBind(&req); err != nil {
		h.errJSON(c, http.StatusBadRequest, "Data tidak valid")
		return
	}

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

	verifiedIdx := make(map[int]bool, len(req.Verified))
	for _, v := range req.Verified {
		if idx, err := strconv.Atoi(v); err == nil {
			verifiedIdx[idx] = true
		}
	}

	saved, dups, err := h.logbookService.BulkSave(bulk, req.SourceFile, verifiedIdx, uid, u, r, ip, ua)
	if err != nil {
		h.errJSON(c, http.StatusInternalServerError, "Gagal menyimpan data")
		return
	}
	if saved == 0 {
		h.errJSON(c, http.StatusConflict, fmt.Sprintf("Semua data adalah duplikat (%d data). Tidak ada yang disimpan. Silakan review data upload, upload file lain, atau kembali ke halaman logbook.", dups))
		return
	}
	message := fmt.Sprintf("Berhasil menyimpan %d data.", saved)
	if dups > 0 {
		message = fmt.Sprintf("Berhasil menyimpan %d data. %d data dilewati karena sudah ada di database.", saved, dups)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true, "saved": saved, "duplicates": dups, "message": message,
	})
}

func (h *Handler) LogbookExport(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat export data"); return }

	search := c.Query("search")
	date := c.Query("date")

	entries, _, err := h.logbookService.List(repository.LogbookFilters{
		Search: search, StartDate: date, EndDate: date, PageSize: 10000,
		SortBy: "date", SortOrder: "ASC",
	})
	if err != nil {
		h.errHTML(c, "Gagal mengambil data logbook")
		return
	}

	svc := services.NewExcelService()
	data := make([][]any, 0, len(entries))
	for _, e := range entries {
		data = append(data, []any{e.Date.Format("2006-01-02"), e.StudentName, e.NIM, e.TimeIn, e.TimeOut, e.Purpose})
	}
	f, err := svc.GenerateMultiSheetExcel([]services.ExcelExportConfig{
		{
			SheetName: "Logbook",
			Headers:   []string{"Tanggal", "Nama", "NIM", "Jam Masuk", "Jam Keluar", "Keperluan"},
			Data:      data,
			ColumnWidths: map[string]float64{"A": 14, "B": 28, "C": 18, "D": 12, "E": 12, "F": 36},
		},
	})
	if err != nil {
		h.errHTML(c, "Gagal membuat file excel")
		return
	}
	defer f.Close()

	fn := svc.GenerateFilename("logbook_export")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+fn)
	if err := f.Write(c.Writer); err != nil {
		c.Error(err)
	}
}

func (h *Handler) LogbookExportPreview(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat export data"); return }

	filterDate := c.Query("date")
	search := c.Query("search")

	entries, _, err := h.logbookService.List(repository.LogbookFilters{
		Search: search, StartDate: filterDate, EndDate: filterDate, PageSize: 10000,
		SortBy: "date", SortOrder: "ASC",
	})
	if err != nil {
		h.errHTML(c, "Gagal mengambil data logbook")
		return
	}

	svc := services.NewExcelService()
	data := make([][]any, 0, len(entries))
	for _, e := range entries {
		data = append(data, []any{e.Date.Format("2006-01-02"), e.StudentName, e.NIM, e.TimeIn, e.TimeOut, e.Purpose})
	}

	f, err := svc.GenerateMultiSheetExcel([]services.ExcelExportConfig{
		{
			SheetName: "Logbook",
			Headers:   []string{"Tanggal", "Nama", "NIM", "Jam Masuk", "Jam Keluar", "Keperluan"},
			Data:      data,
			ColumnWidths: map[string]float64{"A": 14, "B": 28, "C": 18, "D": 12, "E": 12, "F": 36},
		},
	})
	if err != nil {
		h.errHTML(c, "Gagal membuat file excel")
		return
	}
	defer f.Close()

	fn := svc.GenerateFilename("logbook_export_preview")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+fn)
	if err := f.Write(c.Writer); err != nil {
		c.Error(err)
	}
}

func (h *Handler) LogbookCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	h.renderTemplate(c, http.StatusOK, "logbook/create.html", gin.H{
		"title": "Tambah Logbook", "currentPage": "logbook",
		"username": username, "role": role,
	})
}

func (h *Handler) LogbookCreate(c *gin.Context) {
	_, username, role, _ := h.user(c)

	var req CreateLogbookRequest
	if err := c.ShouldBind(&req); err != nil {
		errMsg := "Lengkapi data yang diperlukan"
		if strings.TrimSpace(req.NIM) != "" && len(strings.ReplaceAll(req.NIM, " ", "")) != 11 {
			errMsg = "NIM harus tepat 11 digit angka"
		}
		h.renderTemplate(c, http.StatusBadRequest, "logbook/create.html", gin.H{
			"title": "Tambah Logbook", "currentPage": "logbook",
			"username": username, "role": role, "error": errMsg,
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
		h.renderTemplate(c, http.StatusInternalServerError, "logbook/create.html", gin.H{
			"title": "Tambah Logbook", "currentPage": "logbook",
			"username": u, "role": r, "error": "Gagal menyimpan data",
		})
		return
	}
	if id == 0 {
		h.renderTemplate(c, http.StatusBadRequest, "logbook/create.html", gin.H{
			"title": "Tambah Logbook", "currentPage": "logbook",
			"username": u, "role": r, "error": "Data sudah ada (duplikat)",
		})
		return
	}
	h.redirectWithSuccess(c, "/logbook", "Data logbook berhasil ditambahkan")
}

func (h *Handler) LogbookEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id, _ := strconv.Atoi(c.Param("id"))
	entry, err := h.logbookService.GetByID(id)
	if err != nil { h.errHTML(c, "Data tidak ditemukan"); return }

	h.renderTemplate(c, http.StatusOK, "logbook/edit.html", gin.H{
		"title": "Edit Logbook", "currentPage": "logbook",
		"username": username, "role": role, "entry": entry,
	})
}

func (h *Handler) LogbookEdit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_, username, role, _ := h.user(c)

	renderEditWithError := func(errMsg string) {
		entry, err := h.logbookService.GetByID(id)
		if err != nil {
			h.errHTML(c, "Data tidak ditemukan")
			return
		}
		h.renderTemplate(c, http.StatusBadRequest, "logbook/edit.html", gin.H{
			"title": "Edit Logbook", "currentPage": "logbook",
			"username": username, "role": role,
			"entry": entry, "error": errMsg,
		})
	}

	var req EditLogbookRequest
	if err := c.ShouldBind(&req); err != nil {
		errMsg := "Lengkapi data yang diperlukan"
		if strings.TrimSpace(req.NIM) != "" && len(strings.ReplaceAll(req.NIM, " ", "")) != 11 {
			errMsg = "NIM harus tepat 11 digit angka"
		}
		renderEditWithError(errMsg)
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.logbookService.UpdateEntry(id, services.UpdateLogbookInput{
		Date: req.Date, StudentName: req.StudentName, NIM: req.NIM,
		TimeIn: req.TimeIn, TimeOut: req.TimeOut, Purpose: req.Purpose,
	}, uid, u, r, ip, ua); err != nil {
		renderEditWithError("Gagal mengupdate data")
		return
	}
	h.redirectWithSuccess(c, "/logbook", "Data logbook berhasil diperbarui", "update")
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
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Data logbook berhasil dihapus"})
	} else {
		h.redirectWithSuccess(c, "/logbook", "Data logbook berhasil dihapus", "delete")
	}
}

func (h *Handler) LogbookBatchDelete(c *gin.Context) {
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
	if err := h.logbookService.BatchDelete(intIDs, uid, u, r, ip, ua); err != nil {
		h.errJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Data logbook berhasil dihapus"})
}
