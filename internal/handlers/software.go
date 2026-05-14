package handlers

import (
	"net/http"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"

	"github.com/gin-gonic/gin"
)

// SoftwareList renders list of all software
func (h *Handler) SoftwareList(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	rows, err := h.db.Query(`
		SELECT s.id, s.pc_id, s.name, s.version, s.license, s.category, s.install_date, s.notes,
		       p.pc_number
		FROM software s
		JOIN pcs p ON s.pc_id = p.id
		ORDER BY s.category, s.name, p.pc_number
	`)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data software",
		})
		return
	}
	defer rows.Close()

	type SoftwareWithPC struct {
		models.Software
		PCNumber int `json:"pc_number"`
	}

	var software []SoftwareWithPC
	for rows.Next() {
		var sw SoftwareWithPC
		err := rows.Scan(&sw.ID, &sw.PCID, &sw.Name, &sw.Version, &sw.License, &sw.Category,
			&sw.InstallDate, &sw.Notes, &sw.PCNumber)
		if err != nil {
			continue
		}
		software = append(software, sw)
	}

	// Get list of PCs for the form
	pcRows, err := h.db.Query("SELECT id, pc_number FROM pcs ORDER BY pc_number")
	var pcs []models.PC
	if err == nil {
		defer pcRows.Close()
		for pcRows.Next() {
			var pc models.PC
			if err := pcRows.Scan(&pc.ID, &pc.PCNumber); err == nil {
				pcs = append(pcs, pc)
			}
		}
	}

	c.HTML(http.StatusOK, "software/list.html", gin.H{
		"title":       "Tracking Software - Sistem Inventaris Lab",
		"currentPage": "software",
		"username":    username,
		"role":        role,
		"software":    software,
		"pcs":         pcs,
	})
}

// SoftwareCreate handles software creation
func (h *Handler) SoftwareCreate(c *gin.Context) {
	pcID := c.PostForm("pc_id")
	name := c.PostForm("name")
	version := c.PostForm("version")
	license := c.PostForm("license")
	category := c.PostForm("category")
	installDate := c.PostForm("install_date")
	notes := c.PostForm("notes")

	if pcID == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "PC dan nama software harus diisi",
		})
		return
	}

	if category != "required" && category != "other" {
		category = "other"
	}

	var installDatePtr *string
	if installDate != "" {
		installDatePtr = &installDate
	}

	_, err := h.db.Exec(`
		INSERT INTO software (pc_id, name, version, license, category, install_date, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, pcID, name, version, license, category, installDatePtr, notes)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menyimpan data software",
		})
		return
	}

	c.Redirect(http.StatusFound, "/software")
}
