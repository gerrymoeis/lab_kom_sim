package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) Dashboard(c *gin.Context) {
	userID, username, role, ok := h.user(c)
	if !ok { return }

	data, err := h.dashboardService.GetDashboardData()
	if err != nil { h.errHTML(c, "Gagal mengambil data dashboard"); return }

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title": "Dashboard", "currentPage": "dashboard",
		"user_id": userID, "username": username, "role": role,
		"grid": data.Grid, "pcs": data.PCs,
		"extraPCs": data.ExtraPCs,
		"statusCounts": data.StatusCounts,
		"spareCount": data.SpareCount,
		"totalDevices": data.DeviceCount, "totalSoftware": data.SoftwareCount,
		"pcLecturer": data.PCLecturer, "pcLaboran": data.PCLaboran,
		"pcCCTV": data.PCCCTV,
	})
}
