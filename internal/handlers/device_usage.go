package handlers

import (
	"html/template"
	"net/http"
	"net/url"
	"strconv"

	"inventaris-lab-kom/internal/repository"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

func (h *Handler) DeviceUsageList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := h.cfg.DefaultPageSize

	values, _ := url.ParseQuery(c.Request.URL.RawQuery)
	delete(values, "page")
	var query interface{} = ""
	if len(values) > 0 {
		query = template.URL("&" + values.Encode())
	}

	search := c.Query("search")
	sortBy := c.Query("sort_by")

	usages, total, err := h.deviceUsageService.ListPaginated(repository.DeviceUsageFilters{
		Search: search,
		SortBy: sortBy,
	}, page, pageSize)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data pemakaian")
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	c.HTML(http.StatusOK, "device_usage/list.html", gin.H{
		"title": "Pemakaian", "currentPage": "usages",
		"username": username, "role": role,
		"usages": usages,
		"filters": gin.H{"search": search, "sort_by": sortBy},
		"startRow": (page-1)*pageSize + 1,
		"page": page, "totalPages": totalPages, "totalItems": total,
		"query": query,
	})
}

func (h *Handler) DeviceUsageCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}
	devices, err := h.deviceUsageService.GetConsumableDevices()
	if err != nil {
		devices = nil
	}
	deviceID, _ := strconv.Atoi(c.DefaultQuery("device_id", "0"))
	c.HTML(http.StatusOK, "device_usage/create.html", gin.H{
		"title": "Tambah Pemakaian", "currentPage": "usages",
		"username": username, "role": role,
		"devices": devices, "preselectDeviceID": deviceID,
	})
}

func (h *Handler) DeviceUsageCreate(c *gin.Context) {
	var req CreateDeviceUsageRequest
	if err := c.ShouldBind(&req); err != nil {
		_, username, role, _ := h.user(c)
		c.HTML(http.StatusBadRequest, "device_usage/create.html", gin.H{
			"title": "Tambah Pemakaian", "currentPage": "usages",
			"username": username, "role": role, "error": "Data tidak lengkap",
		})
		return
	}

	deviceID, _ := strconv.Atoi(req.DeviceID)
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	_, err := h.deviceUsageService.CreateUsage(services.CreateUsageInput{
		DeviceID:    deviceID,
		UserName:    req.UserName,
		UserType:    req.UserType,
		UsageDate:   req.UsageDate,
		IsAvailable: req.IsAvailable,
		Purpose:     req.Purpose,
	}, uid, u, r, ip, ua)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "device_usage/create.html", gin.H{
			"title": "Tambah Pemakaian", "currentPage": "usages",
			"username": u, "role": r, "error": "Gagal menyimpan pemakaian",
		})
		return
	}
	c.Redirect(http.StatusFound, "/devices?tab=usages")
}

func (h *Handler) DeviceUsageEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	usage, err := h.deviceUsageService.GetByID(id)
	if err != nil {
		h.errHTML(c, "Pemakaian tidak ditemukan")
		return
	}

	c.HTML(http.StatusOK, "device_usage/edit.html", gin.H{
		"title": "Edit Pemakaian", "currentPage": "usages",
		"username": username, "role": role,
		"usage": usage,
		"deviceTypeName": usage.DeviceTypeName,
		"assetCode": usage.DeviceAssetCode,
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
		UserName:    req.UserName,
		UserType:    req.UserType,
		UsageDate:   req.UsageDate,
		IsAvailable: req.IsAvailable,
		Purpose:     req.Purpose,
		Notes:       req.Notes,
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
