package handlers

import (
	"html/template"
	"net/http"
	"net/url"
	"strconv"

	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

func (h *Handler) fetchDeviceTypes() []models.DeviceType {
	dts, err := h.deviceTypeService.GetAllSimple()
	if err != nil {
		return nil
	}
	return dts
}

func (h *Handler) DeviceList(c *gin.Context) {
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
	category := c.Query("category")
	condition := c.Query("condition")
	sortBy := c.Query("sort_by")
	sortOrder := c.DefaultQuery("sort_order", "ASC")

	devices, total, err := h.deviceService.ListPaginated(repository.DeviceFilters{
		Search:    search,
		Category:  category,
		Condition: condition,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}, page, pageSize)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data perangkat")
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	c.HTML(http.StatusOK, "device/list.html", gin.H{
		"title": "Manajemen Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"devices": devices,
		"deviceTypes": h.fetchDeviceTypes(),
		"filters": gin.H{"search": search, "category": category, "condition": condition, "sort_by": sortBy, "sort_order": sortOrder},
		"startRow": (page-1)*pageSize + 1,
		"page": page, "totalPages": totalPages, "totalItems": total,
		"query": query,
	})
}

func (h *Handler) DeviceCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}
	c.HTML(http.StatusOK, "device/create.html", gin.H{
		"title": "Tambah Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"deviceTypes": h.fetchDeviceTypes(),
	})
}

func (h *Handler) DeviceCreate(c *gin.Context) {
	var req CreateDeviceRequest
	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "device/create.html", gin.H{
			"title": "Tambah Perangkat", "error": "Lengkapi data yang diperlukan",
		})
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	_, _, err := h.deviceService.CreateDevice(services.CreateDeviceInput{
		DeviceTypeID: req.DeviceTypeID,
		SerialNumber: req.SerialNumber,
		Condition:    req.Condition,
		Location:     req.Location,
		PurchaseDate: req.PurchaseDate,
		Notes:        req.Notes,
	}, uid, u, r, ip, ua)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "device/create.html", gin.H{
			"title": "Tambah Perangkat", "error": "Gagal menyimpan perangkat",
		})
		return
	}
	c.Redirect(http.StatusFound, "/devices")
}

func (h *Handler) DeviceDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	d, err := h.deviceService.GetByID(id)
	if err != nil {
		h.errHTML(c, "Perangkat tidak ditemukan")
		return
	}

	dt, _ := h.deviceTypeService.GetByID(d.DeviceTypeID)
	dtName := ""
	if dt != nil {
		dtName = dt.Name
	}

	c.HTML(http.StatusOK, "device/detail.html", gin.H{
		"title": "Detail Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"device": d, "deviceTypeName": dtName,
	})
}

func (h *Handler) DeviceEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	d, err := h.deviceService.GetByID(id)
	if err != nil {
		h.errHTML(c, "Perangkat tidak ditemukan")
		return
	}

	c.HTML(http.StatusOK, "device/edit.html", gin.H{
		"title": "Edit Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"device": d,
		"deviceTypes": h.fetchDeviceTypes(),
	})
}

func (h *Handler) DeviceEdit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req EditDeviceRequest
	if err := c.ShouldBind(&req); err != nil {
		h.errHTML(c, "Data tidak valid")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.deviceService.UpdateDevice(id, services.UpdateDeviceInput{
		DeviceTypeID: req.DeviceTypeID,
		AssetCode:    req.AssetCode,
		SerialNumber: req.SerialNumber,
		Condition:    req.Condition,
		Location:     req.Location,
		PurchaseDate: req.PurchaseDate,
		Notes:        req.Notes,
	}, uid, u, r, ip, ua); err != nil {
		h.errHTML(c, "Gagal mengupdate perangkat")
		return
	}
	c.Redirect(http.StatusFound, "/devices")
}

func (h *Handler) DeviceDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.deviceService.DeleteDevice(id, uid, u, r, ip, ua); err != nil {
		h.redirectWithError(c, "/devices", "Gagal menghapus perangkat")
		return
	}
	c.Redirect(http.StatusFound, "/devices")
}

func (h *Handler) GetNextAssetCode(c *gin.Context) {
	prefix := c.Query("prefix")
	code := h.deviceService.GetNextAssetCode(prefix)
	c.JSON(http.StatusOK, gin.H{"next_code": code})
}
