package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
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

	query := `SELECT sc.id, sc.name, sc.category, sc.description, COUNT(ps.software_id), pc.cnt FROM software_catalog sc LEFT JOIN pc_software ps ON sc.id = ps.software_id AND ps.installed = TRUE CROSS JOIN (SELECT COUNT(*) AS cnt FROM pcs) pc WHERE 1=1`
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

	query += ` GROUP BY sc.id, sc.name, sc.category, sc.description, pc.cnt ORDER BY CASE WHEN sc.category = 'required' THEN 0 ELSE 1 END, sc.name`

	rows, err := h.db.Query(query, args...)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data software")
		return
	}
	defer rows.Close()

	type Stat struct{ models.SoftwareCatalog; InstalledCount, TotalPCs int }
	var stats []Stat
	for rows.Next() {
		var st Stat
		if rows.Scan(&st.ID, &st.Name, &st.Category, &st.Description, &st.InstalledCount, &st.TotalPCs) == nil {
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
	var items []struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Category    string `json:"category"`
		Description string `json:"description"`
	}
	if err := h.db.X.Select(&items, `SELECT id, name, category, COALESCE(description, '') AS description FROM software_catalog WHERE category = 'other' ORDER BY name`); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data"})
		return
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

	var pcList []struct {
		PCID      int  `json:"id"`
		PCNumber  int  `json:"pc_number"`
		Installed bool `json:"installed"`
	}
	if err := h.db.X.Select(&pcList, `SELECT p.id, p.pc_number, COALESCE(ps.installed, FALSE) AS installed FROM pcs p LEFT JOIN pc_software ps ON p.id = ps.pc_id AND ps.software_id = ? ORDER BY p.pc_number`, id); err != nil {
		h.errHTML(c, "Gagal mengambil data PC")
		return
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

	tx, err := h.db.Begin()
	if err != nil { h.redirectWithError(c, "/software", "Gagal memulai transaksi"); return }
	defer tx.Rollback()

	tx.Exec(`DELETE FROM pc_software WHERE software_id = ?`, id)
	for _, pidStr := range pcIDs {
		pid, _ := strconv.Atoi(pidStr)
		if pid > 0 {
			tx.Exec(`INSERT INTO pc_software (pc_id, software_id, installed) VALUES (?, ?, TRUE)`, pid, id)
		}
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	h.activityLogService.LogUpdate(uid, u, r, "software", 0,
		map[string]interface{}{"software_id": id},
		map[string]interface{}{"pc_ids": pcIDs}, ip, ua)
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

	rows, err := h.db.Query(`SELECT sc.id, sc.name, sc.category, sc.description, COUNT(ps.software_id), pc.cnt FROM software_catalog sc LEFT JOIN pc_software ps ON sc.id = ps.software_id AND ps.installed = TRUE CROSS JOIN (SELECT COUNT(*) AS cnt FROM pcs) pc GROUP BY sc.id, sc.name, sc.category, sc.description, pc.cnt ORDER BY sc.category, sc.name`)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data software")
		return
	}
	defer rows.Close()

	type SW struct{ ID int; Name, Category string; Description sql.NullString; Count, TotalPCs int }
	var software []SW
	for rows.Next() {
		var s SW
		if rows.Scan(&s.ID, &s.Name, &s.Category, &s.Description, &s.Count, &s.TotalPCs) == nil {
			software = append(software, s)
		}
	}

	data := [][]interface{}{}
	for i, s := range software {
		desc := "-"
		if s.Description.Valid { desc = s.Description.String }
		cat := "Other"
		if s.Category == "required" { cat = "Required" }
		data = append(data, []interface{}{i + 1, s.Name, cat, desc, fmt.Sprintf("%d / %d PC", s.Count, s.TotalPCs)})
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
