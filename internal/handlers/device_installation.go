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
	values.Del("success")
	values.Del("error")
	values.Del("toast")
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

	h.renderTemplate(c, http.StatusOK, "device_installation/list.html", gin.H{
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
	h.renderTemplate(c, http.StatusOK, "device_installation/create.html", gin.H{
		"title": "Tambah Instalasi", "currentPage": "devices",
		"username": username, "role": role, "android": h.cfg.Android,
		"devices": devices, "preselectDeviceID": deviceID,
	})
}

func (h *Handler) DeviceInstallationCreate(c *gin.Context) {
	var req CreateInstallationRequest
	if err := c.ShouldBind(&req); err != nil {
		_, username, role, _ := h.user(c)
		h.renderTemplate(c, http.StatusBadRequest, "device_installation/create.html", gin.H{
			"title": "Tambah Instalasi", "currentPage": "devices",
			"username": username, "role": role, "android": h.cfg.Android, "error": "Lengkapi data yang diperlukan",
		})
		return
	}

	photo := processPhotoRef(h.cfg.UploadPath, c.GetString("lab"), req.PhotoFileRef, "device_installations")

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
		h.renderTemplate(c, http.StatusInternalServerError, "device_installation/create.html", gin.H{
			"title": "Tambah Instalasi", "currentPage": "devices",
			"username": u, "role": r, "error": "Gagal menyimpan instalasi",
		})
		return
	}
	h.redirectWithSuccess(c, "/devices?tab=installations", "Instalasi berhasil ditambahkan")
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

	h.renderTemplate(c, http.StatusOK, "device_installation/detail.html", gin.H{
		"title": "Detail Instalasi", "currentPage": "devices",
		"username": username, "role": role,
		"installation": inst,
		"deviceLabel":    inst.DeviceLabel,
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

	h.renderTemplate(c, http.StatusOK, "device_installation/edit.html", gin.H{
		"title": "Edit Instalasi", "currentPage": "devices",
		"username": username, "role": role, "android": h.cfg.Android,
		"installation": inst,
		"deviceLabel":    inst.DeviceLabel,
	})
}

func (h *Handler) DeviceInstallationEdit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req EditInstallationRequest
	if err := c.ShouldBind(&req); err != nil {
		h.errHTML(c, "Data tidak valid")
		return
	}

	photo := processPhotoRef(h.cfg.UploadPath, c.GetString("lab"), req.PhotoFileRef, "device_installations")

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
	h.redirectWithSuccess(c, "/devices?tab=installations", "Instalasi berhasil diperbarui", "update")
}

func (h *Handler) DeviceInstallationDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.deviceInstallationService.Delete(id, uid, u, r, ip, ua); err != nil {
		h.redirectWithError(c, "/devices?tab=installations", "Gagal menghapus instalasi")
		return
	}
	h.redirectWithSuccess(c, "/devices?tab=installations", "Instalasi berhasil dihapus", "delete")
}

func (h *Handler) DeviceInstallationBatchDelete(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		h.errJSON(c, http.StatusBadRequest, "Tidak ada item yang dipilih")
		return
	}
	intIDs, err := parseInt64IDs(req.IDs)
	if err != nil {
		h.errJSON(c, http.StatusBadRequest, err.Error())
		return
	}
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	if err := h.deviceInstallationService.BatchDelete(intIDs, uid, u, r, ip, ua); err != nil {
		h.errJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Instalasi berhasil dihapus"})
}

