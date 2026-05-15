package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

// SoftwareList — already refactored with JOIN query in phase 1

func (h *Handler) SoftwareList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	search := c.Query("search")
	filterCategory := c.Query("category")

	query := `SELECT sc.id, sc.name, sc.category, sc.description, COUNT(ps.software_id) FROM software_catalog sc LEFT JOIN pc_software ps ON sc.id = ps.software_id AND ps.installed = TRUE WHERE 1=1`
	var args []interface{}

	if search != "" {
		query += ` AND (sc.name LIKE ? OR sc.description LIKE ?)`
		s := "%" + search + "%"
		args = append(args, s, s)
	}
	if filterCategory == "required" || filterCategory == "other" {
		query += ` AND sc.category = ?`
		args = append(args, filterCategory)
	}

	query += ` GROUP BY sc.id, sc.name, sc.category, sc.description ORDER BY CASE WHEN sc.category = 'required' THEN 0 ELSE 1 END, sc.name`

	rows, err := h.db.Query(query, args...)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data software")
		return
	}
	defer rows.Close()

	var totalPCs int
	h.db.QueryRow(`SELECT COUNT(*) FROM pcs`).Scan(&totalPCs)

	type Stat struct{ models.SoftwareCatalog; InstalledCount, TotalPCs int }
	var stats []Stat
	for rows.Next() {
		var st Stat
		if rows.Scan(&st.ID, &st.Name, &st.Category, &st.Description, &st.InstalledCount) == nil {
			st.TotalPCs = totalPCs
			stats = append(stats, st)
		}
	}

	c.HTML(http.StatusOK, "software/list.html", gin.H{
		"title": "Software Catalog", "currentPage": "software",
		"username": username, "role": role,
		"catalog": stats, "search": search, "filterCat": filterCategory,
		"error": c.Query("error"),
	})
}

func (h *Handler) GetSoftwareCatalogJSON(c *gin.Context) {
	rows, err := h.db.Query(`SELECT id, name, category, description FROM software_catalog WHERE category = 'other' ORDER BY name`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data"})
		return
	}
	defer rows.Close()

	type Item struct {
		ID int `json:"id"`; Name string `json:"name"`; Category string `json:"category"`; Description string `json:"description"`
	}
	var items []Item
	for rows.Next() {
		var it Item
		if rows.Scan(&it.ID, &it.Name, &it.Category, &it.Description) == nil {
			items = append(items, it)
		}
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) SoftwareEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat mengedit software"); return }

	id := c.Param("id")
	var sw models.SoftwareCatalog
	if err := h.db.QueryRow(`SELECT id, name, category, description FROM software_catalog WHERE id = ?`, id).Scan(&sw.ID, &sw.Name, &sw.Category, &sw.Description); err != nil {
		h.errHTML(c, "Software tidak ditemukan")
		return
	}

	rows, err := h.db.Query(`SELECT p.id, p.pc_number, COALESCE(ps.installed, FALSE) FROM pcs p LEFT JOIN pc_software ps ON p.id = ps.pc_id AND ps.software_id = ? ORDER BY p.pc_number`, id)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data PC")
		return
	}
	defer rows.Close()

	type PCWithSoftware struct{ PCID, PCNumber int; Installed bool }
	var pcList []PCWithSoftware
	for rows.Next() {
		var p PCWithSoftware
		if rows.Scan(&p.PCID, &p.PCNumber, &p.Installed) == nil {
			pcList = append(pcList, p)
		}
	}

	c.HTML(http.StatusOK, "software/edit.html", gin.H{
		"title": "Edit Software - " + sw.Name, "currentPage": "software",
		"username": username, "role": role,
		"software": sw, "pcList": pcList,
	})
}

func (h *Handler) SoftwareEdit(c *gin.Context) {
	id := c.Param("id")
	pcIDs := c.PostFormArray("pc_ids[]")
	checked := make(map[string]bool)
	for _, pid := range pcIDs { checked[pid] = true }

	rows, _ := h.db.Query(`SELECT id FROM pcs ORDER BY id`)
	defer rows.Close()

	var allIDs []int
	for rows.Next() { var pid int; rows.Scan(&pid); allIDs = append(allIDs, pid) }
	rows.Close()

	tx, _ := h.db.Begin()
	defer tx.Rollback()

	for _, pid := range allIDs {
		pidStr := fmt.Sprintf("%d", pid)
		if checked[pidStr] {
			var exists bool
			tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM pc_software WHERE pc_id = ? AND software_id = ?)`, pid, id).Scan(&exists)
			if exists {
				tx.Exec(`UPDATE pc_software SET installed = TRUE WHERE pc_id = ? AND software_id = ?`, pid, id)
			} else {
				tx.Exec(`INSERT INTO pc_software (pc_id, software_id, installed) VALUES (?, ?, TRUE)`, pid, id)
			}
		} else {
			tx.Exec(`DELETE FROM pc_software WHERE pc_id = ? AND software_id = ?`, pid, id)
		}
	}
	tx.Commit()

	c.Redirect(http.StatusFound, "/software")
}

func (h *Handler) SoftwareDelete(c *gin.Context) {
	id := c.Param("id")

	var name string
	if err := h.db.QueryRow(`SELECT name FROM software_catalog WHERE id = ?`, id).Scan(&name); err != nil {
		h.redirectWithError(c, "/software", "Software tidak ditemukan")
		return
	}

	if _, err := h.db.Exec(`DELETE FROM software_catalog WHERE id = ?`, id); err != nil {
		h.redirectWithError(c, "/software", "Gagal menghapus software")
		return
	}

	h.logDelete(c, "software", 0, map[string]interface{}{"name": name})
	c.Redirect(http.StatusFound, "/software")
}

func (h *Handler) SoftwareCreate(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	category := c.PostForm("category")
	description := strings.TrimSpace(c.PostForm("description"))

	if name == "" {
		h.redirectWithError(c, "/software", "Nama software harus diisi")
		return
	}
	if category != "required" && category != "other" { category = "other" }

	if _, err := h.db.Exec(`INSERT INTO software_catalog (name, category, description, created_at, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, name, category, description); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			h.logCreateError(c, "software", map[string]interface{}{"name": name, "category": category}, "Duplicate: "+name)
			h.redirectWithError(c, "/software", "Software dengan nama tersebut sudah ada")
			return
		}
		h.logCreateError(c, "software", map[string]interface{}{"name": name, "category": category}, err.Error())
		h.redirectWithError(c, "/software", "Gagal menyimpan software")
		return
	}

	h.logCreate(c, "software", 0, map[string]interface{}{"name": name, "category": category, "description": description})
	c.Redirect(http.StatusFound, "/software")
}

func (h *Handler) SoftwareExport(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat export data software"); return }

	rows, err := h.db.Query(`SELECT sc.id, sc.name, sc.category, sc.description, COUNT(ps.software_id) FROM software_catalog sc LEFT JOIN pc_software ps ON sc.id = ps.software_id AND ps.installed = TRUE GROUP BY sc.id, sc.name, sc.category, sc.description ORDER BY sc.category, sc.name`)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data software")
		return
	}
	defer rows.Close()

	type SW struct{ ID int; Name, Category string; Description sql.NullString; Count int }
	var software []SW
	for rows.Next() {
		var s SW
		if rows.Scan(&s.ID, &s.Name, &s.Category, &s.Description, &s.Count) == nil {
			software = append(software, s)
		}
	}

	var totalPCs int
	h.db.QueryRow(`SELECT COUNT(*) FROM pcs`).Scan(&totalPCs)

	data := [][]interface{}{}
	for i, s := range software {
		desc := "-"
		if s.Description.Valid { desc = s.Description.String }
		cat := "Other"
		if s.Category == "required" { cat = "Required" }
		data = append(data, []interface{}{i + 1, s.Name, cat, desc, fmt.Sprintf("%d / %d PC", s.Count, totalPCs)})
	}

	svc := services.NewExcelService()
	f, _ := svc.GenerateExcel(services.ExcelExportConfig{
		SheetName: "Software Catalog",
		Headers:   []string{"No", "Nama Software", "Kategori", "Deskripsi", "PC Terinstall"},
		Data:      data,
		ColumnWidths: map[string]float64{"A": 5, "B": 30, "C": 12, "D": 40, "E": 15},
	})
	defer f.Close()

	filename := svc.GenerateFilename("software_catalog_export")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	f.Write(c.Writer)
}
