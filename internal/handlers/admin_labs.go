package handlers

import (
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

func (h *GlobalHandler) AdminLabList(c *gin.Context) {
	h.render(c, http.StatusOK, "admin_labs.html", gin.H{
		"title": "Manage Lab",
		"labs":  h.cfg.Labs,
	})
}

func (h *GlobalHandler) AdminLabLayout(c *gin.Context) {
	urlPath := c.Param("urlPath")
	lab := h.labFromPath(urlPath)
	if lab == nil {
		c.AbortWithStatus(404)
		return
	}

	layoutRepo := repository.NewLayoutRepository(h.globalDB)
	layout, err := layoutRepo.GetByLab(urlPath)
	if err != nil {
		layout = nil
	}

	colsStr := "8,8,8,8,8"
	if layout != nil {
		parts := make([]string, len(layout.ColsPerRow))
		for i, v := range layout.ColsPerRow {
			parts[i] = strconv.Itoa(v)
		}
		colsStr = strings.Join(parts, ",")
	}

	h.render(c, http.StatusOK, "admin_lab_layout.html", gin.H{
		"title":    "Layout - " + lab.Title,
		"lab":      lab,
		"cols_str": colsStr,
		"layout":   layout,
	})
}

func (h *GlobalHandler) AdminLabLayoutSave(c *gin.Context) {
	urlPath := c.Param("urlPath")
	lab := h.labFromPath(urlPath)
	if lab == nil {
		c.AbortWithStatus(404)
		return
	}

	colsStr := c.PostForm("cols_per_row")
	hasGap := c.PostForm("has_gap") == "1"
	gapPos, _ := strconv.Atoi(c.PostForm("gap_pos"))

	parts := strings.Split(colsStr, ",")
	cols := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			h.render(c, http.StatusBadRequest, "admin_lab_layout.html", gin.H{
				"title":    "Layout - " + lab.Title,
				"lab":      lab,
				"cols_str": colsStr,
				"error":    fmt.Sprintf("Format cols_per_row tidak valid: %s", p),
			})
			return
		}
		cols = append(cols, n)
	}

	if len(cols) == 0 {
		cols = []int{8, 8, 8, 8, 8}
	}

	layoutRepo := repository.NewLayoutRepository(h.globalDB)
	if err := layoutRepo.Upsert(urlPath, cols, hasGap, gapPos); err != nil {
		h.render(c, http.StatusInternalServerError, "admin_lab_layout.html", gin.H{
			"title": "Layout - " + lab.Title,
			"lab":   lab,
			"error": "Gagal menyimpan layout",
		})
		return
	}

	c.Redirect(http.StatusFound, "/labs")
}

func (h *GlobalHandler) AdminLabSeeds(c *gin.Context) {
	urlPath := c.Param("urlPath")
	lab := h.labFromPath(urlPath)
	if lab == nil {
		c.AbortWithStatus(404)
		return
	}

	h.render(c, http.StatusOK, "admin_lab_seeds.html", gin.H{
		"title": "Seeds - " + lab.Title,
		"lab":   lab,
	})
}

func (h *GlobalHandler) AdminLabReseed(c *gin.Context) {
	urlPath := c.Param("urlPath")
	seedType := c.Param("type")

	lab := h.labFromPath(urlPath)
	if lab == nil {
		c.AbortWithStatus(404)
		return
	}

	db, ok := h.labsDB[urlPath]
	if !ok {
		c.AbortWithStatus(404)
		return
	}

	if err := database.RunSeedType(db, lab.ID, urlPath, seedType); err != nil {
		h.render(c, http.StatusInternalServerError, "admin_lab_seeds.html", gin.H{
			"title": "Seeds - " + lab.Title,
			"lab":   lab,
			"error": "Seed gagal: " + err.Error(),
		})
		return
	}

	h.render(c, http.StatusOK, "admin_lab_seeds.html", gin.H{
		"title": "Seeds - " + lab.Title,
		"lab":   lab,
		"success": "Seed berhasil: " + seedType,
	})
}

func (h *GlobalHandler) AdminLabDelete(c *gin.Context) {
	urlPath := c.Param("urlPath")
	lab := h.labFromPath(urlPath)
	if lab == nil {
		c.AbortWithStatus(404)
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
