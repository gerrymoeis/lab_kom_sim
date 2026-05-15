package handlers

import (
	"database/sql"
	"net/http"

	"inventaris-lab-kom/internal/models"

	"github.com/gin-gonic/gin"
)

func (h *Handler) Dashboard(c *gin.Context) {
	userID, username, role, ok := h.user(c)
	if !ok {
		return
	}

	rows, err := h.db.Query(`SELECT id, pc_number, "row", "column", status, processor, ram, storage, operating_system, notes, last_checked FROM pcs ORDER BY "row", "column"`)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data PC")
		return
	}
	defer rows.Close()

	var pcs []models.PC
	for rows.Next() {
		var pc models.PC
		var processor, ram, storage, os, notes sql.NullString
		var lastChecked sql.NullTime
		if rows.Scan(&pc.ID, &pc.PCNumber, &pc.Row, &pc.Column, &pc.Status, &processor, &ram, &storage, &os, &notes, &lastChecked) != nil {
			continue
		}
		if processor.Valid { pc.Processor = processor.String }
		if ram.Valid { pc.RAM = ram.String }
		if storage.Valid { pc.Storage = storage.String }
		if os.Valid { pc.OperatingSystem = os.String }
		if notes.Valid { pc.Notes = notes.String }
		if lastChecked.Valid { pc.LastChecked = &lastChecked.Time }
		pcs = append(pcs, pc)
	}

	statusCounts := make(map[string]int)
	countRows, _ := h.db.Query(`SELECT status, COUNT(*) FROM pcs GROUP BY status`)
	if countRows != nil {
		defer countRows.Close()
		for countRows.Next() {
			var s string; var c int
			if countRows.Scan(&s, &c) == nil { statusCounts[s] = c }
		}
	}

	var totalDevices, totalSoftware int
	h.db.QueryRow("SELECT COUNT(*) FROM devices").Scan(&totalDevices)
	h.db.QueryRow("SELECT COUNT(*) FROM software_catalog").Scan(&totalSoftware)

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
		"title": "Dashboard", "currentPage": "dashboard",
		"user_id": userID, "username": username, "role": role,
		"grid": grid, "pcs": pcs,
		"statusCounts": statusCounts,
		"totalDevices": totalDevices, "totalSoftware": totalSoftware,
	})
}
