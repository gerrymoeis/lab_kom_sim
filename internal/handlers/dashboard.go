package handlers

import (
	"database/sql"
	"net/http"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"

	"github.com/gin-gonic/gin"
)

// Dashboard renders main dashboard with PC grid
func (h *Handler) Dashboard(c *gin.Context) {
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Get all PCs ordered by row and column
	rows, err := h.db.Query(`
		SELECT id, pc_number, "row", "column", status, processor, ram, storage, operating_system, notes, last_checked
		FROM pcs
		ORDER BY "row", "column"
	`)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data PC",
		})
		return
	}
	defer rows.Close()

	var pcs []models.PC
	for rows.Next() {
		var pc models.PC
		var processor, ram, storage, operatingSystem, notes sql.NullString
		var lastChecked sql.NullTime
		
		err := rows.Scan(&pc.ID, &pc.PCNumber, &pc.Row, &pc.Column, &pc.Status, 
			&processor, &ram, &storage, &operatingSystem, &notes, &lastChecked)
		if err != nil {
			continue
		}
		
		// Convert NullString to string
		if processor.Valid {
			pc.Processor = processor.String
		}
		if ram.Valid {
			pc.RAM = ram.String
		}
		if storage.Valid {
			pc.Storage = storage.String
		}
		if operatingSystem.Valid {
			pc.OperatingSystem = operatingSystem.String
		}
		if notes.Valid {
			pc.Notes = notes.String
		}
		if lastChecked.Valid {
			pc.LastChecked = &lastChecked.Time
		}
		
		pcs = append(pcs, pc)
	}

	// Get status counts
	statusCounts := make(map[string]int)
	countRows, err := h.db.Query(`
		SELECT status, COUNT(*) as count
		FROM pcs
		GROUP BY status
	`)
	if err == nil {
		defer countRows.Close()
		for countRows.Next() {
			var status string
			var count int
			if err := countRows.Scan(&status, &count); err == nil {
				statusCounts[status] = count
			}
		}
	}

	// Get total devices count
	var totalDevices int
	h.db.QueryRow("SELECT COUNT(*) FROM devices").Scan(&totalDevices)

	// Get total software count
	var totalSoftware int
	h.db.QueryRow("SELECT COUNT(*) FROM software").Scan(&totalSoftware)

	// Organize PCs into grid (5 rows x 8 columns)
	grid := make([][]models.PC, 5)
	for i := range grid {
		grid[i] = make([]models.PC, 8)
	}

	for _, pc := range pcs {
		if pc.Row >= 1 && pc.Row <= 5 && pc.Column >= 1 && pc.Column <= 8 {
			grid[pc.Row-1][pc.Column-1] = pc
		}
	}

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title":         "Dashboard - Sistem Inventaris Lab",
		"user_id":       userID,
		"username":      username,
		"role":          role,
		"currentPage":   "dashboard",
		"grid":          grid,
		"pcs":           pcs,
		"statusCounts":  statusCounts,
		"totalDevices":  totalDevices,
		"totalSoftware": totalSoftware,
	})
}
