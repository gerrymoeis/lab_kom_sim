package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/services"

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
// GetSoftwareCatalogJSON returns all software catalog entries as JSON (for AJAX autocomplete)
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

// SoftwareExport exports software catalog to Excel
func (h *Handler) SoftwareExport(c *gin.Context) {
	_, _, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	if role != "admin" {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"title":   "Error",
			"message": "Hanya admin yang dapat export data software",
		})
		return
	}

	rows, err := h.db.Query(`
		SELECT sc.id, sc.name, sc.category, sc.description,
		       COUNT(ps.pc_id) FILTER (WHERE ps.installed = TRUE) as installed_count
		FROM software_catalog sc
		LEFT JOIN pc_software ps ON sc.id = ps.software_id
		GROUP BY sc.id, sc.name, sc.category, sc.description
		ORDER BY sc.category, sc.name
	`)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data software",
		})
		return
	}
	defer rows.Close()

	type SoftwareExportRow struct {
		ID             int
		Name           string
		Category       string
		Description    sql.NullString
		InstalledCount int
	}

	var software []SoftwareExportRow
	for rows.Next() {
		var s SoftwareExportRow
		if err := rows.Scan(&s.ID, &s.Name, &s.Category, &s.Description, &s.InstalledCount); err == nil {
			software = append(software, s)
		}
	}

	var totalPCs int
	h.db.QueryRow(`SELECT COUNT(*) FROM pcs`).Scan(&totalPCs)

	data := [][]interface{}{}
	for i, s := range software {
		desc := "-"
		if s.Description.Valid {
			desc = s.Description.String
		}
		category := "Other"
		if s.Category == "required" {
			category = "Required"
		}
		row := []interface{}{
			i + 1,
			s.Name,
			category,
			desc,
			fmt.Sprintf("%d / %d PC", s.InstalledCount, totalPCs),
		}
		data = append(data, row)
	}

	excelService := services.NewExcelService()
	config := services.ExcelExportConfig{
		SheetName: "Software Catalog",
		Headers:   []string{"No", "Nama Software", "Kategori", "Deskripsi", "PC Terinstall"},
		Data:      data,
		ColumnWidths: map[string]float64{
			"A": 5,
			"B": 30,
			"C": 12,
			"D": 40,
			"E": 15,
		},
	}

	f, err := excelService.GenerateExcel(config)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal generate file Excel",
		})
		return
	}
	defer f.Close()

	filename := excelService.GenerateFilename("software_catalog_export")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Transfer-Encoding", "binary")

	if err := f.Write(c.Writer); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal generate file Excel",
		})
		return
	}
}
