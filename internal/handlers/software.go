package handlers

import (
	"net/http"
	"strings"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"

	"github.com/gin-gonic/gin"
)

// SoftwareList renders the software catalog list
func (h *Handler) SoftwareList(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	search := c.Query("search")
	filterCategory := c.Query("category")

	query := `SELECT id, name, category, description FROM software_catalog WHERE 1=1`
	args := []interface{}{}

	if search != "" {
		query += ` AND (name LIKE ? OR description LIKE ?)`
		s := "%" + search + "%"
		args = append(args, s, s)
	}

	if filterCategory == "required" || filterCategory == "other" {
		query += ` AND category = ?`
		args = append(args, filterCategory)
	}

	query += ` ORDER BY category, name`

	rows, err := h.db.Query(query, args...)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data software",
		})
		return
	}
	defer rows.Close()

	var catalog []models.SoftwareCatalog
	for rows.Next() {
		var s models.SoftwareCatalog
		if err := rows.Scan(&s.ID, &s.Name, &s.Category, &s.Description); err == nil {
			catalog = append(catalog, s)
		}
	}

	// Count per-PC stats
	type SoftwareStat struct {
		models.SoftwareCatalog
		InstalledCount int `json:"installed_count"`
		TotalPCs       int `json:"total_pcs"`
	}

	var totalPCs int
	h.db.QueryRow(`SELECT COUNT(*) FROM pcs`).Scan(&totalPCs)

	var stats []SoftwareStat
	for _, s := range catalog {
		var installed int
		h.db.QueryRow(`SELECT COUNT(*) FROM pc_software WHERE software_id = ? AND installed = TRUE`, s.ID).Scan(&installed)
		stats = append(stats, SoftwareStat{
			SoftwareCatalog: s,
			InstalledCount: installed,
			TotalPCs:       totalPCs,
		})
	}

	c.HTML(http.StatusOK, "software/list.html", gin.H{
		"title":       "Software Catalog - Sistem Inventaris Lab",
		"currentPage": "software",
		"username":    username,
		"role":        role,
		"catalog":     stats,
		"search":      search,
		"filterCat":   filterCategory,
	})
}

// SoftwareCreate handles adding new software to the catalog
func (h *Handler) SoftwareCreate(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	category := c.PostForm("category")
	description := strings.TrimSpace(c.PostForm("description"))

	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nama software harus diisi"})
		return
	}
	if category != "required" && category != "other" {
		category = "other"
	}

	_, err := h.db.Exec(`INSERT INTO software_catalog (name, category, description, created_at, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, name, category, description)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Software dengan nama tersebut sudah ada"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan software"})
		return
	}

	c.Redirect(http.StatusFound, "/software")
}
