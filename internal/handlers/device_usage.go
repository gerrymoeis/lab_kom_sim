package handlers

import (
	"net/http"
	"strconv"

	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

func (h *Handler) DeviceUsageList(c *gin.Context) {
	c.Redirect(http.StatusFound, "/devices?tab=usages")
}

func (h *Handler) DeviceUsageCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	c.HTML(http.StatusOK, "device_usage/create.html", gin.H{
		"title": "Tambah Pemakaian", "currentPage": "devices",
		"username": username, "role": role,
	})
}

func (h *Handler) DeviceUsageCreate(c *gin.Context) {
	var req CreateDeviceUsageRequest
	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "device_usage/create.html", gin.H{
			"title": "Tambah Pemakaian", "error": "Data tidak lengkap",
		})
		return
	}

	deviceID, _ := strconv.Atoi(req.DeviceID)
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	_, err := h.deviceUsageService.CreateUsage(services.CreateUsageInput{
		DeviceID: deviceID, UserName: req.UserName, UserType: req.UserType,
		UsageDate: req.UsageDate, Quantity: req.Quantity,
		IsAvailable: req.IsAvailable, Purpose: req.Purpose,
	}, uid, u, r, ip, ua)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "device_usage/create.html", gin.H{
			"title": "Tambah Pemakaian", "error": "Gagal menyimpan pemakaian",
		})
		return
	}
	c.Redirect(http.StatusFound, "/devices?tab=usages")
}

func (h *Handler) DeviceUsageEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id, _ := strconv.Atoi(c.Param("id"))
	usage, err := h.deviceUsageService.GetByID(id)
	if err != nil { h.errHTML(c, "Pemakaian tidak ditemukan"); return }

	c.HTML(http.StatusOK, "device_usage/edit.html", gin.H{
		"title": "Edit Pemakaian", "currentPage": "devices",
		"username": username, "role": role, "usage": usage,
	})
}

func (h *Handler) DeviceUsageEdit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req EditDeviceUsageRequest
	if err := c.ShouldBind(&req); err != nil {
		h.errHTML(c, "Data tidak valid")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.deviceUsageService.UpdateUsage(id, services.UpdateUsageInput{
		UserName: req.UserName, UserType: req.UserType,
		UsageDate: req.UsageDate, Quantity: req.Quantity,
		IsAvailable: req.IsAvailable, Purpose: req.Purpose, Notes: req.Notes,
	}, uid, u, r, ip, ua); err != nil {
		h.errHTML(c, "Gagal mengupdate pemakaian")
		return
	}
	c.Redirect(http.StatusFound, "/devices?tab=usages")
}

func (h *Handler) DeviceUsageDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.deviceUsageService.DeleteUsage(id, uid, u, r, ip, ua); err != nil {
		h.redirectWithError(c, "/devices?tab=usages", "Gagal menghapus pemakaian")
		return
	}
	c.Redirect(http.StatusFound, "/devices?tab=usages")
}

func (h *Handler) DeviceUsageUpdateAvailability(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req UpdateAvailabilityRequest
	if err := c.ShouldBind(&req); err != nil || (req.IsAvailable != "yes" && req.IsAvailable != "no") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Status tidak valid"})
		return
	}
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	if err := h.deviceUsageService.UpdateAvailability(id, req.IsAvailable, uid, u, r, ip, ua); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
