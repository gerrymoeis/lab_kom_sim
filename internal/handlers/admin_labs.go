package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/repository"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func joinInts(ints []int, sep string) string {
	parts := make([]string, len(ints))
	for i, v := range ints {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, sep)
}

func (h *GlobalHandler) AdminLabList(c *gin.Context) {
	h.render(c, http.StatusOK, "admin/lab_list.html", gin.H{
		"title":       "Manage Lab",
		"currentPage": "labs",
		"icon":        "bi-gear",
		"labs":        h.cfg.Labs,
	})
}

func (h *GlobalHandler) AdminLabDetail(c *gin.Context) {
	urlPath := c.Param("urlPath")
	lab := h.labFromPath(urlPath)
	if lab == nil {
		h.render(c, http.StatusNotFound, "error.html", gin.H{
			"title":   "Lab Tidak Ditemukan",
			"message": "Lab '" + urlPath + "' tidak ditemukan.",
		})
		c.Abort()
		return
	}

	layoutRepo := repository.NewLayoutRepository(h.globalDB)
	layout, err := layoutRepo.GetByLab(urlPath)
	if err != nil || layout == nil {
		layout = &config.GridLayout{ColsPerRow: []int{8, 8, 8, 8, 8}}
	}
	if layout.RowGaps == nil {
		layout.RowGaps = config.RowGapsFromOld(layout.ColsPerRow, layout.HasGap, layout.GapPos)
	}

	colsStr := joinInts(layout.ColsPerRow, ",")
	totalPCs := 0
	for _, c := range layout.ColsPerRow {
		totalPCs += c
	}

	h.render(c, http.StatusOK, "admin/lab_detail.html", gin.H{
		"title":       "Detail - " + lab.Title,
		"currentPage": "labs",
		"icon":        "bi-eye",
		"lab":         lab,
		"layout":      layout,
		"colsStr":     colsStr,
		"colsPerRow":  layout.ColsPerRow,
		"rowGaps":     layout.RowGaps,
		"totalPCs":    totalPCs,
		"backURL":     "/labs",
	})
}

func (h *GlobalHandler) AdminLabLayout(c *gin.Context) {
	urlPath := c.Param("urlPath")
	lab := h.labFromPath(urlPath)
	if lab == nil {
		h.render(c, http.StatusNotFound, "error.html", gin.H{
			"title":   "Lab Tidak Ditemukan",
			"message": "Lab '" + urlPath + "' tidak ditemukan.",
		})
		c.Abort()
		return
	}

	layoutRepo := repository.NewLayoutRepository(h.globalDB)
	layout, err := layoutRepo.GetByLab(urlPath)
	if err != nil {
		layout = nil
	}

	colsStr := "8,8,8,8,8"
	colsPerRow := []int{8, 8, 8, 8, 8}
	rowGapsJSON := "[]"
	if layout != nil {
		parts := make([]string, len(layout.ColsPerRow))
		for i, v := range layout.ColsPerRow {
			parts[i] = strconv.Itoa(v)
		}
		colsStr = strings.Join(parts, ",")
		colsPerRow = layout.ColsPerRow
		if layout.RowGaps == nil {
			layout.RowGaps = config.RowGapsFromOld(layout.ColsPerRow, layout.HasGap, layout.GapPos)
		}
		rgJSON, _ := json.Marshal(layout.RowGaps)
		rowGapsJSON = string(rgJSON)
	}

	h.render(c, http.StatusOK, "admin/lab_layout.html", gin.H{
		"title":       "Layout - " + lab.Title,
		"currentPage": "labs",
		"icon":        "bi-grid-3x3",
		"lab":         lab,
		"colsStr":     colsStr,
		"colsPerRow":  colsPerRow,
		"rowGapsJSON": rowGapsJSON,
		"backURL":     "/labs/" + urlPath,
	})
}

func (h *GlobalHandler) AdminLabLayoutSave(c *gin.Context) {
	urlPath := c.Param("urlPath")
	lab := h.labFromPath(urlPath)
	if lab == nil {
		h.render(c, http.StatusNotFound, "error.html", gin.H{
			"title":   "Lab Tidak Ditemukan",
			"message": "Lab '" + urlPath + "' tidak ditemukan.",
		})
		c.Abort()
		return
	}

	colsStr := c.PostForm("cols_per_row")
	rowGapsStr := c.PostForm("row_gaps_json")

	errorData := func(errMsg string) gin.H {
		return gin.H{
			"title":       "Layout - " + lab.Title,
			"currentPage": "labs",
			"icon":        "bi-grid-3x3",
			"lab":         lab,
			"colsStr":     colsStr,
			"colsPerRow":  nil,
			"rowGapsJSON": rowGapsStr,
			"backURL":     "/labs/" + urlPath,
			"error":       errMsg,
		}
	}

	parts := strings.Split(colsStr, ",")
	cols := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			h.render(c, http.StatusBadRequest, "admin/lab_layout.html", errorData(
				fmt.Sprintf("Format cols_per_row tidak valid: %s", p),
			))
			return
		}
		cols = append(cols, n)
	}

	if len(cols) == 0 {
		cols = []int{8, 8, 8, 8, 8}
	}

	var rowGaps [][]int
	if rowGapsStr != "" {
		if err := json.Unmarshal([]byte(rowGapsStr), &rowGaps); err != nil {
			rowGaps = config.RowGapsFromOld(cols, false, 0)
		}
	} else {
		rowGaps = config.RowGapsFromOld(cols, false, 0)
	}

	hasGap := false
	gapPos := 0
	for _, gaps := range rowGaps {
		if len(gaps) > 0 {
			hasGap = true
			gapPos = gaps[0]
			break
		}
	}

	layoutRepo := repository.NewLayoutRepository(h.globalDB)
	if err := layoutRepo.Upsert(urlPath, cols, hasGap, gapPos, rowGaps); err != nil {
		h.render(c, http.StatusInternalServerError, "admin/lab_layout.html", gin.H{
			"title":       "Layout - " + lab.Title,
			"currentPage": "labs",
			"icon":        "bi-grid-3x3",
			"lab":         lab,
			"colsStr":     joinInts(cols, ","),
			"colsPerRow":  cols,
			"rowGapsJSON": rowGapsStr,
			"backURL":     "/labs/" + urlPath,
			"error":       "Gagal menyimpan layout",
		})
		return
	}

	c.Redirect(http.StatusFound, "/labs/"+urlPath)
}

func (h *GlobalHandler) AdminLabSeeds(c *gin.Context) {
	urlPath := c.Param("urlPath")
	lab := h.labFromPath(urlPath)
	if lab == nil {
		h.render(c, http.StatusNotFound, "error.html", gin.H{
			"title":   "Lab Tidak Ditemukan",
			"message": "Lab '" + urlPath + "' tidak ditemukan.",
		})
		c.Abort()
		return
	}

	h.render(c, http.StatusOK, "admin/lab_seeds.html", gin.H{
		"title":       "Seeds - " + lab.Title,
		"currentPage": "labs",
		"icon":        "bi-database",
		"lab":         lab,
		"backURL":     "/labs/" + urlPath,
	})
}

func (h *GlobalHandler) AdminLabReseed(c *gin.Context) {
	urlPath := c.Param("urlPath")
	seedType := c.Param("type")

	lab := h.labFromPath(urlPath)
	if lab == nil {
		h.render(c, http.StatusNotFound, "error.html", gin.H{
			"title":   "Lab Tidak Ditemukan",
			"message": "Lab '" + urlPath + "' tidak ditemukan.",
		})
		c.Abort()
		return
	}

	db, ok := h.labsDB[urlPath]
	if !ok {
		h.render(c, http.StatusNotFound, "error.html", gin.H{
			"title":   "Lab Tidak Ditemukan",
			"message": "Database lab '" + urlPath + "' tidak tersedia.",
		})
		c.Abort()
		return
	}

	if err := database.RunSeedType(db, lab.ID, urlPath, seedType); err != nil {
		h.render(c, http.StatusInternalServerError, "admin/lab_seeds.html", gin.H{
			"title": "Seeds - " + lab.Title,
			"lab":   lab,
			"error": "Seed gagal: " + err.Error(),
		})
		return
	}

	h.render(c, http.StatusOK, "admin/lab_seeds.html", gin.H{
		"title":   "Seeds - " + lab.Title,
		"lab":     lab,
		"success": "Seed berhasil: " + seedType,
	})
}

func (h *GlobalHandler) AdminLabCreatePage(c *gin.Context) {
	h.render(c, http.StatusOK, "admin/lab_create.html", gin.H{
		"title":       "Tambah Lab Baru",
		"currentPage": "labs",
		"icon":        "bi-plus-circle",
		"backURL":     "/labs",
	})
}

func (h *GlobalHandler) AdminLabCreate(c *gin.Context) {
	id := strings.TrimSpace(c.PostForm("id"))
	title := strings.TrimSpace(c.PostForm("title"))
	urlPath := strings.TrimSpace(c.PostForm("url"))
	rowsStr := strings.TrimSpace(c.PostForm("rows"))
	colsStr := strings.TrimSpace(c.PostForm("cols"))

	errorData := func(errMsg string) gin.H {
		return gin.H{
			"title":       "Tambah Lab Baru",
			"currentPage": "labs",
			"icon":        "bi-plus-circle",
			"backURL":     "/labs",
			"error":       errMsg,
			"id":          id,
			"labTitle":    title,
			"url":         urlPath,
			"rows":        rowsStr,
			"cols":        colsStr,
		}
	}

	if id == "" || title == "" || urlPath == "" || rowsStr == "" || colsStr == "" {
		h.render(c, http.StatusBadRequest, "admin/lab_create.html", errorData("Semua field harus diisi"))
		return
	}

	for _, l := range h.cfg.Labs {
		if l.URLPath == urlPath {
			h.render(c, http.StatusBadRequest, "admin/lab_create.html", errorData("URL path '"+urlPath+"' sudah digunakan oleh lab lain"))
			return
		}
		if l.ID == id {
			h.render(c, http.StatusBadRequest, "admin/lab_create.html", errorData("ID '"+id+"' sudah digunakan oleh lab lain"))
			return
		}
	}

	rows, err := strconv.Atoi(rowsStr)
	if err != nil || rows < 1 {
		h.render(c, http.StatusBadRequest, "admin/lab_create.html", errorData("Jumlah baris harus angka positif"))
		return
	}

	colParts := strings.Split(colsStr, ",")
	if len(colParts) != rows {
		h.render(c, http.StatusBadRequest, "admin/lab_create.html", errorData("Jumlah kolom/baris tidak sesuai dengan jumlah baris"))
		return
	}
	cols := make([]int, rows)
	for i, p := range colParts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 1 {
			h.render(c, http.StatusBadRequest, "admin/lab_create.html", errorData("Kolom/baris ke-"+strconv.Itoa(i+1)+" tidak valid"))
			return
		}
		cols[i] = n
	}

	// Determine DB path from existing lab's directory
	dirExisting := filepath.Dir(h.cfg.Labs[0].DBPath)
	dbPath := filepath.Join(dirExisting, "lab_"+urlPath+".db")
	uploadDir := filepath.Join(h.cfg.UploadPath, urlPath)

	// --- Init DB + Migrations (safe: zero side effects if fails) ---
	db, err := database.InitDB(dbPath, "")
	if err != nil {
		h.render(c, http.StatusInternalServerError, "admin/lab_create.html", errorData("Gagal membuat database: "+err.Error()))
		return
	}

	if err := database.RunMigrations(db, false, id, urlPath, h.cfg.UploadPath, false); err != nil {
		db.Close()
		os.Remove(dbPath)
		h.render(c, http.StatusInternalServerError, "admin/lab_create.html", errorData("Gagal migrasi database: "+err.Error()))
		return
	}

	if err := database.SeedDefaultUser(db); err != nil {
		log.Printf("Warning: gagal seed default user untuk lab %s: %v", urlPath, err)
	}

	// --- Seed global DB ---
	colsJSON, _ := json.Marshal(cols)
	rowGaps := config.RowGapsFromOld(cols, false, 0)
	rowGapsJSON, _ := json.Marshal(rowGaps)

	if _, err := h.globalDB.Exec(`INSERT INTO grid_layouts (lab_url_path, cols_per_row, has_gap, gap_pos, row_gaps) VALUES (?, ?, 0, 0, ?)`,
		urlPath, string(colsJSON), string(rowGapsJSON)); err != nil {
		db.Close()
		os.Remove(dbPath)
		h.render(c, http.StatusInternalServerError, "admin/lab_create.html", errorData("Gagal menyimpan layout: "+err.Error()))
		return
	}

	var adminID int
	h.globalDB.QueryRow("SELECT id FROM global_users WHERE is_super_admin = 1 LIMIT 1").Scan(&adminID)
	if adminID > 0 {
		h.globalDB.Exec("INSERT OR IGNORE INTO lab_permissions (user_id, lab_url_path, role) VALUES (?, ?, 'admin')", adminID, urlPath)
	}

	// Create upload subdirs
	for _, sub := range []string{"pc", "device_types", "temp", "logbook", "device_installations"} {
		if err := os.MkdirAll(filepath.Join(uploadDir, sub), 0755); err != nil {
			log.Printf("Warning: gagal buat upload subdir %s/%s: %v", urlPath, sub, err)
		}
	}

	// Build LabConfig (EnvIndex will be set after AppendLabEnv)
	newLab := config.LabConfig{
		ID:        id,
		Title:     title,
		DBPath:    dbPath,
		URLPath:   urlPath,
		UploadDir: uploadDir,
		Layout:    config.GetGridLayout(urlPath),
	}

	// --- Append .env ---
	envIndex, err := config.AppendLabEnv(h.cfg.EnvPath, newLab)
	if err != nil {
		db.Close()
		os.Remove(dbPath)
		h.render(c, http.StatusInternalServerError, "admin/lab_create.html", errorData("Gagal menulis .env: "+err.Error()))
		return
	}
	newLab.EnvIndex = envIndex

	// --- Append in-memory config ---
	h.cfg.Labs = append(h.cfg.Labs, newLab)

	// --- Register handler + DB ---
	newHandler := NewHandler(db, h.cfg, h.notifier, h.globalAuthService, h.globalDB, urlPath)
	h.registrar.Register(urlPath, newHandler)
	h.labsDB[urlPath] = db

	// --- Start backup + public build services ---
	labBackupCfg := h.cfg.Backup
	for i, dir := range labBackupCfg.Dir {
		labBackupCfg.Dir[i] = filepath.Join(dir, urlPath)
	}
	backupSvc := services.NewBackupService(db, labBackupCfg)
	backupSvc.Start()
	h.notifier.Add(backupSvc)

	pubSvc := services.NewPublicBuildService(db, h.cfg.PublicBuild, urlPath, title, h.cfg.UploadPath)
	pubSvc.Start()
	h.notifier.Add(pubSvc)

	// --- Redirect ---
	session := sessions.Default(c)
	session.AddFlash("Lab '"+title+"' berhasil dibuat", "success")
	session.Save()
	c.Redirect(http.StatusFound, "/labs/"+urlPath)
}

func (h *GlobalHandler) AdminLabDelete(c *gin.Context) {
	urlPath := c.Param("urlPath")
	lab := h.labFromPath(urlPath)
	if lab == nil {
		h.render(c, http.StatusNotFound, "error.html", gin.H{
			"title":   "Lab Tidak Ditemukan",
			"message": "Lab '" + urlPath + "' tidak ditemukan.",
		})
		c.Abort()
		return
	}

	// Jangan izinkan delete lab terakhir
	if len(h.cfg.Labs) <= 1 {
		session := sessions.Default(c)
		session.AddFlash("Tidak dapat menghapus lab terakhir. Minimal harus ada 1 lab.", "error")
		session.Save()
		c.Redirect(http.StatusFound, "/labs")
		return
	}

	// Hapus semua lab_permissions untuk lab ini
	h.globalDB.Exec("DELETE FROM lab_permissions WHERE lab_url_path = ?", urlPath)

	// Tutup koneksi DB lab
	if db, ok := h.labsDB[urlPath]; ok {
		db.Close()
		delete(h.labsDB, urlPath)
	}

	// Rename file DB + WAL/SHM ke .deleted (soft-delete, reversible)
	renameLabDB(lab.DBPath)

	// Hapus dari config slice
	newLabs := make([]config.LabConfig, 0, len(h.cfg.Labs)-1)
	for _, l := range h.cfg.Labs {
		if l.URLPath != urlPath {
			newLabs = append(newLabs, l)
		}
	}
	h.cfg.Labs = newLabs

	// Comment out di .env jika menggunakan format baru (EnvIndex > 0)
	if lab.EnvIndex > 0 {
		if err := config.CommentOutLabEnv(h.cfg.EnvPath, lab.EnvIndex); err != nil {
			// Rollback: restore lab ke config
			h.cfg.Labs = append(h.cfg.Labs, *lab)
			h.labsDB[urlPath] = nil
			session := sessions.Default(c)
			session.AddFlash("Gagal menghapus lab: tidak dapat menulis .env ("+err.Error()+")", "error")
			session.Save()
			c.Redirect(http.StatusFound, "/labs")
			return
		}
	}

	// Hapus grid_layouts
	h.globalDB.Exec("DELETE FROM grid_layouts WHERE lab_url_path = ?", urlPath)

	// Set flash success via session
	session := sessions.Default(c)
	session.AddFlash("Lab berhasil dihapus", "success")
	session.Save()

	c.Redirect(http.StatusFound, "/labs")
}

// renameLabDB merename file SQLite .db + WAL/SHM dengan suffix .deleted
// untuk soft-delete yang reversible tanpa kehilangan data di disk.
func renameLabDB(dbPath string) {
	if dbPath == "" {
		return
	}
	if err := os.Rename(dbPath, dbPath+".deleted"); err != nil {
		log.Printf("Warning: gagal rename DB %s: %v", dbPath, err)
	}
	for _, ext := range []string{"-wal", "-shm"} {
		p := dbPath + ext
		if _, err := os.Stat(p); err == nil {
			if err := os.Rename(p, p+".deleted"); err != nil {
				log.Printf("Warning: gagal rename %s: %v", p, err)
			}
		}
	}
}
