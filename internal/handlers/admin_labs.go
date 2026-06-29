package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/repository"

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
			log.Printf("Warning: gagal comment out .env: %v", err)
		}
	}

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
