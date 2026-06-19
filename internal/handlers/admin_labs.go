package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/repository"

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

	c.Redirect(http.StatusFound, "/admin/labs")
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
