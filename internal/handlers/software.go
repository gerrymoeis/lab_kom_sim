package handlers

import (
	"net/http"
	"strings"

	"inventaris-lab-kom/internal/database"
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
	countQuery := `SELECT COUNT(*) FROM software_catalog WHERE 1=1`
	args := []interface{}{}

	if search != "" {
		query += ` AND (name LIKE ? OR description LIKE ?)`
		countQuery += ` AND (name LIKE ? OR description LIKE ?)`
		s := "%" + search + "%"
		args = append(args, s, s)
	}

	if filterCategory == "required" || filterCategory == "other" {
		query += ` AND category = ?`
		countQuery += ` AND category = ?`
		args = append(args, filterCategory)
	}

	query += ` ORDER BY category, name`

	var totalCount int
	err := h.db.QueryRow(countQuery, args...).Scan(&totalCount)
	if err != nil {
		totalCount = 0
	}

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

	// Count per-PC stats for each software
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

// SoftwareCreate handles adding new software to catalog
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

// SoftwareToggle toggles installed status for a software on a PC
func (h *Handler) SoftwareToggle(c *gin.Context) {
	pcID := c.PostForm("pc_id")
	softwareID := c.PostForm("software_id")
	installed := c.PostForm("installed") == "true"

	if pcID == "" || softwareID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "PC dan software harus diisi"})
		return
	}

	var exists bool
	h.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pc_software WHERE pc_id = ? AND software_id = ?)`, pcID, softwareID).Scan(&exists)

	if exists {
		_, err := h.db.Exec(`UPDATE pc_software SET installed = ? WHERE pc_id = ? AND software_id = ?`, installed, pcID, softwareID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate status software"})
			return
		}
	} else {
		_, err := h.db.Exec(`INSERT INTO pc_software (pc_id, software_id, installed, created_at, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, pcID, softwareID, installed)
		_ = err
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "installed": installed})
}

// SoftwareAddToPC adds an existing or new software to a PC
func (h *Handler) SoftwareAddToPC(c *gin.Context) {
	pcID := c.PostForm("pc_id")
	name := strings.TrimSpace(c.PostForm("name"))
	version := strings.TrimSpace(c.PostForm("version"))

	if pcID == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "PC dan nama software harus diisi"})
		return
	}

	// Find or create in catalog
	var softwareID int
	err := h.db.QueryRow(`SELECT id FROM software_catalog WHERE name = ?`, name).Scan(&softwareID)
	if err != nil {
		// Create new entry as "other"
		var newID int
		err = h.db.QueryRow(`INSERT INTO software_catalog (name, category, created_at, updated_at) VALUES (?, 'other', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) RETURNING id`, name).Scan(&newID)
		if err != nil {
			// SQLite fallback
			res, execErr := h.db.Exec(`INSERT INTO software_catalog (name, category, created_at, updated_at) VALUES (?, 'other', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, name)
			if execErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan software"})
				return
			}
			lastID, _ := res.LastInsertId()
			softwareID = int(lastID)
		} else {
			softwareID = newID
		}
	}

	// Add to PC
	var exists bool
	h.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pc_software WHERE pc_id = ? AND software_id = ?)`, pcID, softwareID).Scan(&exists)
	if !exists {
		_, err = h.db.Exec(`INSERT INTO pc_software (pc_id, software_id, installed, version, created_at, updated_at) VALUES (?, ?, TRUE, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, pcID, softwareID, version)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menambahkan software ke PC"})
			return
		}
	}

	// Get the software name for response
	var swName string
	h.db.QueryRow(`SELECT name FROM software_catalog WHERE id = ?`, softwareID).Scan(&swName)

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"software_id":  softwareID,
		"software_name": swName,
	})
}

// SoftwareRemoveFromPC removes a software from a PC
func (h *Handler) SoftwareRemoveFromPC(c *gin.Context) {
	pcID := c.PostForm("pc_id")
	softwareID := c.PostForm("software_id")

	if pcID == "" || softwareID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data tidak lengkap"})
		return
	}

	_, err := h.db.Exec(`DELETE FROM pc_software WHERE pc_id = ? AND software_id = ?`, pcID, softwareID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus software"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetSoftwareCatalogJSON returns all software catalog entries as JSON (for AJAX dropdown)
func (h *Handler) GetSoftwareCatalogJSON(c *gin.Context) {
	rows, err := h.db.Query(`SELECT id, name, category FROM software_catalog ORDER BY name`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data"})
		return
	}
	defer rows.Close()

	type Item struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Category string `json:"category"`
	}

	var items []Item
	for rows.Next() {
		var it Item
		if rows.Scan(&it.ID, &it.Name, &it.Category) == nil {
			items = append(items, it)
		}
	}

	c.JSON(http.StatusOK, items)
}

// ensure pc_software record exists (used by dbwrap for SELECT)
var _ *database.DB
