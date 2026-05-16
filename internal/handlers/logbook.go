package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

// IsDuplicateEntry is in internal/services/duplicate.go

type LogbookFilters struct {
	DateFrom, DateTo *time.Time
	Search, SortBy, SortOrder string
	Limit, Offset int
}

// ─── List ────────────────────────────────────────────────────────

func (h *Handler) LogbookList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 { page = 1 }
	pageSize, _ := strconv.Atoi(c.DefaultQuery("size", "25"))
	if pageSize < 1 || pageSize > 100 { pageSize = 25 }

	f := LogbookFilters{Limit: pageSize, Offset: (page - 1) * pageSize, SortBy: c.DefaultQuery("sort_by", "date"), SortOrder: c.DefaultQuery("sort_order", "DESC"), Search: c.Query("search")}
	if d := c.Query("date_from"); d != "" { if t, err := time.Parse("2006-01-02", d); err == nil { f.DateFrom = &t } }
	if d := c.Query("date_to"); d != "" { if t, err := time.Parse("2006-01-02", d); err == nil { f.DateTo = &t } }

	query := `SELECT id, date, student_name, nim, time_in, time_out, purpose, source_file, created_at FROM logbook_entries WHERE 1=1`
	countQ := `SELECT COUNT(*) FROM logbook_entries WHERE 1=1`
	args := []interface{}{}
	var cond string

	if f.DateFrom != nil { cond += ` AND date >= ?`; args = append(args, *f.DateFrom) }
	if f.DateTo != nil { cond += ` AND date <= ?`; args = append(args, *f.DateTo) }
	if f.Search != "" {
		cond += ` AND (student_name LIKE ? OR nim LIKE ? OR purpose LIKE ?)`
		s := "%" + f.Search + "%"; args = append(args, s, s, s)
	}

	var total int
	h.db.QueryRow(countQ+cond, args...).Scan(&total)

	order := " ORDER BY date"
	switch f.SortBy {
	case "student_name": order = " ORDER BY student_name"
	case "nim": order = " ORDER BY nim"
	case "time_in": order = " ORDER BY time_in"
	case "created_at": order = " ORDER BY created_at"
	}
	if f.SortOrder == "ASC" { order += " ASC" } else { order += " DESC" }

	rows, err := h.db.Query(query+cond+order+" LIMIT ? OFFSET ?", append(args, f.Limit, f.Offset)...)
	if err != nil { h.errHTML(c, "Gagal mengambil data logbook"); return }
	defer rows.Close()

	var entries []models.LogbookEntry
	for rows.Next() {
		var e models.LogbookEntry
		if rows.Scan(&e.ID, &e.Date, &e.StudentName, &e.NIM, &e.TimeIn, &e.TimeOut, &e.Purpose, &e.SourceFile, &e.CreatedAt) == nil {
			entries = append(entries, e)
		}
	}

	tp := (total + pageSize - 1) / pageSize; if tp < 1 { tp = 1 }
	c.HTML(http.StatusOK, "logbook/list.html", gin.H{
		"title": "Logbook Absensi", "currentPage": "logbook",
		"username": username, "role": role,
		"entries": entries, "totalCount": total,
		"page": page, "totalPages": tp, "pageSize": pageSize,
		"filters": gin.H{"date_from": c.Query("date_from"), "date_to": c.Query("date_to"), "search": f.Search, "sort_by": f.SortBy, "sort_order": f.SortOrder},
	})
}

// ─── Upload & OCR ────────────────────────────────────────────────

func (h *Handler) LogbookUploadPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	c.HTML(http.StatusOK, "logbook/upload.html", gin.H{
		"title": "Upload Logbook", "currentPage": "logbook",
		"username": username, "role": role,
	})
}

func (h *Handler) LogbookUpload(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat mengupload"); return }

	var path, fn string

	fileRef := strings.TrimSpace(c.PostForm("file_ref"))
	if fileRef != "" {
		// File was pre-uploaded via /api/upload-image, move from temp
		fn = fileRef
		tempPath := filepath.Join("uploads", "temp", fn)
		path = filepath.Join("uploads", "logbook", fn)
		os.MkdirAll(filepath.Dir(path), 0755)
		if err := copyFile(tempPath, path); err != nil {
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
		fn = strings.TrimSuffix(fn, ext) + ".jpeg"
		path = filepath.Join("uploads", "logbook", fn)
		if err := h.imageService.CompressAndSave(tempPath, path, 1280); err != nil {
			os.Remove(tempPath); h.errHTML(c, "Gagal memproses gambar"); return
		}
		os.Remove(tempPath)
	}

	apiKey := h.cfg.GeminiAPIKey
	if apiKey == "" { h.errHTML(c, "GEMINI_API_KEY tidak dikonfigurasi"); return }

	result, err := services.NewOCRService(apiKey, h.cfg.OpenRouterAPIKey).ExtractLogbookFromImage(path)

	success, errorMsg, rawText := true, "", ""
	var entries []services.LogbookEntry

	if err != nil {
		success = false; errorMsg = fmt.Sprintf("Gagal OCR: %v. File tetap tersimpan.", err)
	} else {
		success = result.Success; errorMsg = result.Error
		entries = result.Entries; rawText = result.RawText
	}

	c.HTML(http.StatusOK, "logbook/preview.html", gin.H{
		"title": "Preview OCR", "currentPage": "logbook",
		"username": username, "role": role,
		"entries": entries, "raw_text": rawText, "success": success, "error": errorMsg,
		"source_file": fn,
	})
}

// ─── Save ─────────────────────────────────────────────────────────

func (h *Handler) LogbookSave(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"}); return }
	if role != "admin" { c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"}); return }

	if err := c.Request.ParseForm(); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal parsing form"}); return }

	sf := c.PostForm("source_file")
	dates, names, nims := c.PostFormArray("date[]"), c.PostFormArray("student_name[]"), c.PostFormArray("nim[]")
	timeIns, timeOuts, purposes := c.PostFormArray("time_in[]"), c.PostFormArray("time_out[]"), c.PostFormArray("purpose[]")

	if len(dates) == 0 { c.JSON(http.StatusBadRequest, gin.H{"error": "Tidak ada data"}); return }

	cfg := config.DefaultDuplicateConfig

	now := time.Now()

	dateMap := make(map[string]time.Time)
	for i := 0; i < len(dates); i++ {
		if dates[i] == "" && names[i] == "" && nims[i] == "" { continue }
		dv := now
		if dates[i] != "" {
			if d, e := time.Parse("2006-01-02", dates[i]); e == nil { dv = d } else if d, e := time.Parse("02/01/2006", dates[i]); e == nil { dv = d }
		}
		dateMap[dv.Format("2006-01-02")] = dv
	}

	type dupEntry struct{ date time.Time; name, nim, timeIn string }
	existing := make(map[string][]dupEntry) // key: "date|time_in"
	for _, dv := range dateMap {
		if r, _ := h.db.Query(`SELECT date, student_name, nim, time_in FROM logbook_entries WHERE date = ?`, dv); r != nil {
			for r.Next() {
				var ed time.Time; var en, eni, et string
				if r.Scan(&ed, &en, &eni, &et) == nil {
					k := dv.Format("2006-01-02") + "|" + et
					existing[k] = append(existing[k], dupEntry{ed, en, eni, et})
				}
			}
			r.Close()
		}
	}

	tx, _ := h.db.Begin()
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO logbook_entries (date, student_name, nim, time_in, time_out, purpose, source_file, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyiapkan statement"}); return }
	defer stmt.Close()

	saved, dups := 0, 0
	for i := 0; i < len(dates); i++ {
		if dates[i] == "" && names[i] == "" && nims[i] == "" { continue }

		dv := now
		if dates[i] != "" {
			if d, e := time.Parse("2006-01-02", dates[i]); e == nil { dv = d } else if d, e := time.Parse("02/01/2006", dates[i]); e == nil { dv = d }
		}
		ti := timeIns[i]
		to := ""; if i < len(timeOuts) { to = timeOuts[i] }
		pu := ""; if i < len(purposes) { pu = purposes[i] }

		dup := false
		k := dv.Format("2006-01-02") + "|" + ti
		for _, e := range existing[k] {
			if services.IsDuplicateEntry(dv, e.date, ti, e.timeIn, names[i], e.name, nims[i], e.nim, cfg) { dup = true; break }
		}
		if dup { dups++; continue }

		if _, e := stmt.Exec(dv, names[i], nims[i], ti, to, pu, sf, now, now); e != nil {
			if strings.Contains(e.Error(), "UNIQUE") { dups++; continue }
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Gagal menyimpan: %v", e)})
			return
		}
		saved++
	}

	if err := tx.Commit(); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit"}); return }

	msg := fmt.Sprintf("Berhasil menyimpan %d entry", saved)
	if dups > 0 { msg += fmt.Sprintf(" (%d duplikat dilewati)", dups) }
	c.JSON(http.StatusOK, gin.H{"success": true, "message": msg, "saved": saved, "duplicates": dups, "total_processed": len(dates)})
}

// ─── Export ───────────────────────────────────────────────────────

func (h *Handler) LogbookExport(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat export"); return }

	q := `SELECT id, date, student_name, nim, time_in, time_out, purpose, source_file, created_at FROM logbook_entries WHERE 1=1`
	var args []interface{}
	if s := c.Query("start_date"); s != "" { q += ` AND date >= ?`; args = append(args, s) }
	if e := c.Query("end_date"); e != "" { q += ` AND date <= ?`; args = append(args, e) }
	q += ` ORDER BY date DESC, time_in DESC`

	rows, _ := h.db.Query(q, args...)
	var entries []models.LogbookEntry
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var e models.LogbookEntry
			if rows.Scan(&e.ID, &e.Date, &e.StudentName, &e.NIM, &e.TimeIn, &e.TimeOut, &e.Purpose, &e.SourceFile, &e.CreatedAt) == nil {
				entries = append(entries, e)
			}
		}
	}

	data := [][]interface{}{}
	for i, e := range entries {
		data = append(data, []interface{}{i + 1, e.Date.Format("2006-01-02"), e.StudentName, e.NIM, e.Purpose, e.TimeIn, e.TimeOut})
	}

	svc := services.NewExcelService()
	f, _ := svc.GenerateExcel(services.ExcelExportConfig{
		SheetName: "Logbook",
		Headers:   []string{"No", "Tanggal", "Nama Mahasiswa", "NIM", "Keperluan", "Jam Masuk", "Jam Keluar"},
		Data:      data,
		ColumnWidths: map[string]float64{"A": 5, "B": 12, "C": 25, "D": 13, "E": 30, "F": 11, "G": 11},
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
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat export"); return }

	dates, names, nims := c.QueryArray("date[]"), c.QueryArray("student_name[]"), c.QueryArray("nim[]")
	timeIns, timeOuts, purposes := c.QueryArray("time_in[]"), c.QueryArray("time_out[]"), c.QueryArray("purpose[]")

	data := [][]interface{}{}
	for i := 0; i < len(dates); i++ {
		pu := ""; if i < len(purposes) { pu = purposes[i] }
		to := ""; if i < len(timeOuts) { to = timeOuts[i] }
		data = append(data, []interface{}{i + 1, dates[i], names[i], nims[i], pu, timeIns[i], to})
	}

	svc := services.NewExcelService()
	f, _ := svc.GenerateExcel(services.ExcelExportConfig{
		SheetName: "Logbook", Headers: []string{"No", "Tanggal", "Nama Mahasiswa", "NIM", "Keperluan", "Jam Masuk", "Jam Keluar"},
		Data: data, ColumnWidths: map[string]float64{"A": 5, "B": 12, "C": 25, "D": 13, "E": 30, "F": 11, "G": 11},
	})
	defer f.Close()

	fn := svc.GenerateFilename("logbook_preview")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+fn)
	f.Write(c.Writer)
}

// ─── Manual CRUD ─────────────────────────────────────────────────

func (h *Handler) LogbookCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	c.HTML(http.StatusOK, "logbook/create.html", gin.H{
		"title": "Tambah Entry Logbook", "currentPage": "logbook",
		"username": username, "role": role,
	})
}

func (h *Handler) LogbookCreate(c *gin.Context) {
	ds := c.PostForm("date"); sn := c.PostForm("student_name"); nim := c.PostForm("nim")
	ti := c.PostForm("time_in"); to := c.PostForm("time_out"); pu := c.PostForm("purpose")

	if ds == "" || sn == "" || nim == "" || ti == "" {
		c.HTML(http.StatusBadRequest, "logbook/create.html", gin.H{"title": "Tambah Entry Logbook", "error": "Semua field harus diisi"}); return
	}

	sn = toTitleCaseWithAbbr(sn); nim = strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(nim, " ", ""))); pu = toTitleCaseWithAbbr(pu)

	date, _ := time.Parse("2006-01-02", ds)

	// Check duplicate
	if r, _ := h.db.Query(`SELECT date, student_name, nim, time_in FROM logbook_entries WHERE date = ? AND time_in = ?`, date, ti); r != nil {
		for r.Next() {
			var ed time.Time; var en, eni, et string
			if r.Scan(&ed, &en, &eni, &et) == nil && services.IsDuplicateEntry(date, ed, ti, et, sn, en, nim, eni, config.DefaultDuplicateConfig) {
				c.HTML(http.StatusBadRequest, "logbook/create.html", gin.H{"title": "Tambah Entry Logbook", "error": "Entry duplikat ditemukan"}); return
			}
		}
		r.Close()
	}

	_, err := h.db.Exec(`INSERT INTO logbook_entries (date, student_name, nim, time_in, time_out, purpose, source_file, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 'manual_entry', ?, ?)`, date, sn, nim, ti, to, pu, time.Now(), time.Now())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") { c.HTML(http.StatusBadRequest, "logbook/create.html", gin.H{"title": "Tambah Entry Logbook", "error": "Entry duplikat"}); return }
		h.logCreateError(c, "logbook", map[string]interface{}{"student_name": sn}, err.Error())
		h.errHTML(c, "Gagal menyimpan"); return
	}

	var lid int
	h.db.QueryRow(`SELECT MAX(id) FROM logbook_entries`).Scan(&lid)

	h.logCreate(c, "logbook", lid, map[string]interface{}{"date": ds, "student_name": sn, "nim": nim})
	c.Redirect(http.StatusFound, "/logbook")
}

func (h *Handler) LogbookEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	var e models.LogbookEntry
	if err := h.db.QueryRow(`SELECT id, date, student_name, nim, time_in, time_out, purpose, source_file FROM logbook_entries WHERE id = ?`, c.Param("id")).Scan(&e.ID, &e.Date, &e.StudentName, &e.NIM, &e.TimeIn, &e.TimeOut, &e.Purpose, &e.SourceFile); err != nil {
		h.errHTML(c, "Entry tidak ditemukan"); return
	}

	c.HTML(http.StatusOK, "logbook/edit.html", gin.H{
		"title": "Edit Entry Logbook", "currentPage": "logbook",
		"username": username, "role": role, "entry": e,
		"dateStr": e.Date.Format("2006-01-02"),
	})
}

func (h *Handler) LogbookEdit(c *gin.Context) {
	id := c.Param("id"); ds := c.PostForm("date"); sn := c.PostForm("student_name"); nim := c.PostForm("nim")
	ti := c.PostForm("time_in"); to := c.PostForm("time_out"); pu := c.PostForm("purpose")

	if ds == "" || sn == "" || nim == "" || ti == "" { h.errHTML(c, "Semua field harus diisi"); return }
	date, _ := time.Parse("2006-01-02", ds)
	sn = toTitleCaseWithAbbr(sn); nim = strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(nim, " ", ""))); pu = toTitleCaseWithAbbr(pu)

	var oldDate time.Time; var oldName, oldNIM string
	h.db.QueryRow(`SELECT date, student_name, nim FROM logbook_entries WHERE id = ?`, id).Scan(&oldDate, &oldName, &oldNIM)

	if _, err := h.db.Exec(`UPDATE logbook_entries SET date=?, student_name=?, nim=?, time_in=?, time_out=?, purpose=?, updated_at=? WHERE id=?`, date, sn, nim, ti, to, pu, time.Now(), id); err != nil {
		h.logUpdateError(c, "logbook", 0, map[string]interface{}{"id": id}, err.Error())
		h.errHTML(c, "Gagal mengupdate"); return
	}

	h.logUpdate(c, "logbook", 0, map[string]interface{}{"date": oldDate, "student_name": oldName, "nim": oldNIM}, map[string]interface{}{"date": ds, "student_name": sn, "nim": nim})
	c.Redirect(http.StatusFound, "/logbook")
}

func (h *Handler) LogbookDelete(c *gin.Context) {
	id := c.Param("id")
	var eid int; var d time.Time; var sn, nim string
	if err := h.db.QueryRow(`SELECT id, date, student_name, nim FROM logbook_entries WHERE id = ?`, id).Scan(&eid, &d, &sn, &nim); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data"}); return
	}

	if _, err := h.db.Exec("DELETE FROM logbook_entries WHERE id = ?", id); err != nil {
		h.logDeleteError(c, "logbook", eid, map[string]interface{}{"student_name": sn}, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus"}); return
	}

	h.logDelete(c, "logbook", eid, map[string]interface{}{"student_name": sn, "nim": nim, "date": d.Format("2006-01-02")})
	c.Redirect(http.StatusFound, "/logbook")
}

func toTitleCaseWithAbbr(text string) string {
	text = strings.TrimSpace(text)
	if text == "" { return "" }
	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")
	words := strings.Fields(text)
	for i, w := range words {
		if len(w) > 0 { words[i] = strings.ToUpper(string(w[0])) + strings.ToLower(w[1:]) }
	}
	r := strings.Join(words, " ")
	r = regexp.MustCompile(`\b([A-Z])([A-Z])\b`).ReplaceAllString(r, "$1.$2")
	return strings.TrimSuffix(r, ".")
}

func normalizeStudentName(s string) string { return toTitleCaseWithAbbr(s) }
func normalizeNIM(n string) string { return strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(n, " ", ""))) }
func normalizePurpose(s string) string { return toTitleCaseWithAbbr(s) }
