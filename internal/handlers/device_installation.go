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

func (h *Handler) DeviceInstallationList(c *gin.Context) {
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

	installations, total, err := h.deviceInstallationService.ListPaginated(repository.InstallationFilters{
		Search: search,
		SortBy: sortBy,
	}, page, pageSize)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data instalasi")
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	c.HTML(http.StatusOK, "device_installation/list.html", gin.H{
		"title": "Instalasi Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"installations": installations,
		"filters":       gin.H{"search": search, "sort_by": sortBy},
		"startRow":      (page-1)*pageSize + 1,
		"page":          page, "totalPages": totalPages, "totalItems": total,
		"query": query,
	})
}

func (h *Handler) DeviceInstallationCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}
	devices, err := h.deviceInstallationService.GetInstallableDevices()
	if err != nil {
		devices = nil
	}
	deviceID, _ := strconv.Atoi(c.DefaultQuery("device_id", "0"))
	c.HTML(http.StatusOK, "device_installation/create.html", gin.H{
		"title": "Tambah Instalasi", "currentPage": "devices",
		"username": username, "role": role,
		"devices": devices, "preselectDeviceID": deviceID,
	})
}

func (h *Handler) DeviceInstallationCreate(c *gin.Context) {
	var req CreateInstallationRequest
	if err := c.ShouldBind(&req); err != nil {
		_, username, role, _ := h.user(c)
		c.HTML(http.StatusBadRequest, "device_installation/create.html", gin.H{
			"title": "Tambah Instalasi", "currentPage": "devices",
			"username": username, "role": role, "error": "Lengkapi data yang diperlukan",
		})
		return
	}

	photo := processPhotoRef(req.PhotoFileRef, "device_installations")

	deviceID, _ := strconv.Atoi(req.DeviceID)
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	_, err := h.deviceInstallationService.Create(services.CreateInstallationInput{
		DeviceID:               deviceID,
		LocationInstalled:      req.LocationInstalled,
		InstallationStartDate:  req.InstallationStartDate,
		InstallationFinishDate: req.InstallationFinishDate,
		Photo:                  photo,
		Notes:                  req.Notes,
	}, uid, u, r, ip, ua)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "device_installation/create.html", gin.H{
			"title": "Tambah Instalasi", "currentPage": "devices",
			"username": u, "role": r, "error": "Gagal menyimpan instalasi",
		})
		return
	}
	c.Redirect(http.StatusFound, "/devices?tab=installations")
}

func (h *Handler) DeviceInstallationDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	inst, err := h.deviceInstallationService.GetByID(id)
	if err != nil {
		h.errHTML(c, "Instalasi tidak ditemukan")
		return
	}

	c.HTML(http.StatusOK, "device_installation/detail.html", gin.H{
		"title": "Detail Instalasi", "currentPage": "devices",
		"username": username, "role": role,
		"installation": inst,
		"assetCode":    inst.DeviceAssetCode,
	})
}

func (h *Handler) DeviceInstallationEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	inst, err := h.deviceInstallationService.GetByID(id)
	if err != nil {
		h.errHTML(c, "Instalasi tidak ditemukan")
		return
	}

	c.HTML(http.StatusOK, "device_installation/edit.html", gin.H{
		"title": "Edit Instalasi", "currentPage": "devices",
		"username": username, "role": role,
		"installation": inst,
		"assetCode":    inst.DeviceAssetCode,
	})
}

func (h *Handler) DeviceInstallationEdit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req EditInstallationRequest
	if err := c.ShouldBind(&req); err != nil {
		h.errHTML(c, "Data tidak valid")
		return
	}

	photo := processPhotoRef(req.PhotoFileRef, "device_installations")

	// If no new photo uploaded, keep existing
	if photo == "" {
		inst, err := h.deviceInstallationService.GetByID(id)
		if err == nil {
			photo = inst.Photo
		}
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.deviceInstallationService.Update(id, services.UpdateInstallationInput{
		LocationInstalled:      req.LocationInstalled,
		InstallationStartDate:  req.InstallationStartDate,
		InstallationFinishDate: req.InstallationFinishDate,
		Photo:                  photo,
		Notes:                  req.Notes,
	}, uid, u, r, ip, ua); err != nil {
		h.errHTML(c, "Gagal mengupdate instalasi")
		return
	}
	c.Redirect(http.StatusFound, "/devices?tab=installations")
}

func (h *Handler) DeviceInstallationDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.deviceInstallationService.Delete(id, uid, u, r, ip, ua); err != nil {
		h.redirectWithError(c, "/devices?tab=installations", "Gagal menghapus instalasi")
		return
	}
	c.Redirect(http.StatusFound, "/devices?tab=installations")
}


