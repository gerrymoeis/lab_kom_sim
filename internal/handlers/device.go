package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

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

	tab := c.DefaultQuery("tab", "types")

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

	switch tab {
	case "loans":
		search := c.Query("search")
		status := c.Query("status")
		category := c.Query("category")
		sortBy := c.Query("sort_by")
		sortOrder := c.Query("sort_order")

		loans, total, err := h.deviceLoanService.ListPaginated(repository.DeviceLoanFilters{
			Search:    search,
			Status:    status,
			Category:  category,
			SortBy:    sortBy,
			SortOrder: sortOrder,
		}, page, pageSize)
		if err != nil {
			h.errHTML(c, "Gagal mengambil data peminjaman")
			return
		}

		totalPages := (total + pageSize - 1) / pageSize
		h.renderTemplate(c, http.StatusOK, "device_loan/list.html", gin.H{
			"title": "Peminjaman", "currentPage": "devices",
			"activeTab": "loans",
			"username": username, "role": role,
			"loans": loans,
			"filters":    gin.H{"search": search, "status": status, "category": category, "sort_by": sortBy, "sort_order": sortOrder},
			"categories": h.fetchCategories("loanable"),
			"startRow":   (page-1)*pageSize + 1,
			"page": page, "totalPages": totalPages, "totalItems": total,
			"query": query,
		})

	case "usages":
		search := c.Query("search")
		isAvailable := c.Query("is_available")
		category := c.Query("category")
		sortBy := c.Query("sort_by")
		sortOrder := c.Query("sort_order")

		usages, total, err := h.deviceUsageService.ListPaginated(repository.DeviceUsageFilters{
			Search:      search,
			IsAvailable: isAvailable,
			Category:    category,
			SortBy:      sortBy,
			SortOrder:   sortOrder,
		}, page, pageSize)
		if err != nil {
			h.errHTML(c, "Gagal mengambil data pemakaian")
			return
		}

		totalPages := (total + pageSize - 1) / pageSize
		h.renderTemplate(c, http.StatusOK, "device_usage/list.html", gin.H{
			"title": "Pemakaian Perangkat", "currentPage": "devices",
			"activeTab": "usages",
			"username": username, "role": role,
			"usages": usages,
			"filters":    gin.H{"search": search, "is_available": isAvailable, "category": category, "sort_by": sortBy, "sort_order": sortOrder},
			"categories": h.fetchCategories("consumable"),
			"startRow":   (page-1)*pageSize + 1,
			"page": page, "totalPages": totalPages, "totalItems": total,
			"query": query,
		})

	case "installations":
		search := c.Query("search")
		status := c.Query("status")
		category := c.Query("category")
		sortBy := c.Query("sort_by")
		sortOrder := c.Query("sort_order")

		installations, total, err := h.deviceInstallationService.ListPaginated(repository.InstallationFilters{
			Search:    search,
			Status:    status,
			Category:  category,
			SortBy:    sortBy,
			SortOrder: sortOrder,
		}, page, pageSize)
		if err != nil {
			h.errHTML(c, "Gagal mengambil data instalasi")
			return
		}

		totalPages := (total + pageSize - 1) / pageSize
		h.renderTemplate(c, http.StatusOK, "device_installation/list.html", gin.H{
			"title": "Instalasi Perangkat", "currentPage": "devices",
			"activeTab": "installations",
			"username": username, "role": role,
			"installations": installations,
			"filters":       gin.H{"search": search, "status": status, "category": category, "sort_by": sortBy, "sort_order": sortOrder},
			"categories":    h.fetchCategories("installable"),
			"startRow":   (page-1)*pageSize + 1,
			"page": page, "totalPages": totalPages, "totalItems": total,
			"query": query,
		})

	default: // types tab — grouped by category → device type
		search := c.Query("search")
		condition := c.Query("condition")
		category := c.Query("category")
		sortBy := c.Query("sort_by")
		sortOrder := c.Query("sort_order")

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

		activeLoanIDs, _ := h.deviceService.GetActiveLoanIDs()
		depletedIDs, _ := h.deviceService.GetDepletedIDs()
		if activeLoanIDs == nil {
			activeLoanIDs = make(map[int]bool)
		}
		if depletedIDs == nil {
			depletedIDs = make(map[int]bool)
		}

		installationStatuses, _ := h.deviceService.GetInstallationStatuses()
		if installationStatuses == nil {
			installationStatuses = make(map[int]string)
		}

		groupedData := groupDevices(devices, activeLoanIDs, depletedIDs)

		totalPages := (total + pageSize - 1) / pageSize
		endRow := page * pageSize
		if endRow > total {
			endRow = total
		}
		h.renderTemplate(c, http.StatusOK, "device/list.html", gin.H{
			"title": "Manajemen Perangkat", "currentPage": "devices",
			"activeTab": "types",
			"username": username, "role": role,
			"groupedData":         groupedData,
			"activeLoanIDs":       activeLoanIDs,
			"depletedIDs":         depletedIDs,
			"installationStatuses": installationStatuses,
			"filters":    gin.H{"search": search, "category": category, "condition": condition, "sort_by": sortBy, "sort_order": sortOrder},
			"startRow":   (page-1)*pageSize + 1,
			"endRow":     endRow,
			"page": page, "totalPages": totalPages, "totalItems": total,
			"query":   query,
			"categories": h.fetchCategories(""),
		})
	}
}

func (h *Handler) fetchCategories(usageType string) []models.Category {
	var cats []models.Category
	var err error
	if usageType == "" {
		cats, err = h.categoryService.List()
	} else {
		cats, err = h.categoryService.ListByUsageType(usageType)
	}
	if err != nil {
		return nil
	}
	return cats
}

func (h *Handler) DeviceCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}
	h.renderTemplate(c, http.StatusOK, "device/create.html", gin.H{
		"title": "Tambah Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"deviceTypes": h.fetchDeviceTypes(),
		"categories":  h.fetchCategories(""),
		"android":     h.cfg.Android,
	})
}

func (h *Handler) DeviceCreate(c *gin.Context) {
	var req CreateDeviceRequest
	_, username, role, _ := h.user(c)

	if err := c.ShouldBind(&req); err != nil {
		h.renderTemplate(c, http.StatusBadRequest, "device/create.html", gin.H{
			"title": "Tambah Perangkat", "currentPage": "devices",
			"username": username, "role": role, "error": "Lengkapi data yang diperlukan",
			"deviceTypes": h.fetchDeviceTypes(),
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
		h.renderTemplate(c, http.StatusInternalServerError, "device/create.html", gin.H{
			"title": "Tambah Perangkat", "currentPage": "devices",
			"username": username, "role": role, "error": "Gagal menyimpan perangkat",
			"deviceTypes": h.fetchDeviceTypes(),
		})
		return
	}
	h.redirectWithSuccess(c, "/devices?tab=types", "Perangkat berhasil ditambahkan")
}

func (h *Handler) DeviceBatchCreate(c *gin.Context) {
	var req BatchCreateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data batch tidak valid: " + err.Error()})
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	// Validate new device type fields before creating anything
	if req.NewTypeName != "" && req.NewTypeUsageType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tipe penggunaan harus diisi untuk varian baru"})
		return
	}

	// Resolve category: create inline if needed
	catID := req.CategoryID
	if catID == 0 && req.NewCategoryName != "" {
		prefix := req.NewCategoryPrefix
		if prefix == "" {
			prefix = strings.ToUpper(strings.ReplaceAll(req.NewCategoryName, " ", "_"))
		}
		id, err := h.categoryService.Create(req.NewCategoryName, prefix, uid, u, r, ip, ua)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		catID = id
	}

	// Process photo ref for inline device type creation
	photoFile, err := processDeviceTypePhotoRef(h.cfg.UploadPath, c.GetString("lab"), req.NewTypePhotoFileRef)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Resolve device type: create inline if needed
	typeID := req.DeviceTypeID
	if typeID == 0 && req.NewTypeName != "" {
		if catID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Pilih atau buat kategori terlebih dahulu"})
			return
		}
		prefix := req.NewTypeLabelPrefix
		if prefix == "" {
			prefix = strings.ToUpper(strings.ReplaceAll(req.NewTypeName, " ", "_"))
		}
		id, err := h.deviceTypeService.Create(services.DeviceTypeCreateInput{
			CategoryID:      catID,
			Name:            req.NewTypeName,
			Brand:           req.NewTypeBrand,
			Model:           req.NewTypeModel,
			LabelPrefix: prefix,
			UsageType:       req.NewTypeUsageType,
			DefaultLocation: req.NewTypeDefaultLocation,
			Photo:           photoFile,
		}, uid, u, r, ip, ua)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		typeID = id
	}

	if typeID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tipe perangkat diperlukan"})
		return
	}

	var devices []services.BatchDeviceCreateInput
	for _, d := range req.Devices {
		devices = append(devices, services.BatchDeviceCreateInput{
			SerialNumber: d.SerialNumber,
			Condition:    d.Condition,
			Location:     d.Location,
			PurchaseDate: d.PurchaseDate,
			Notes:        d.Notes,
		})
	}

	codes, err := h.deviceService.BatchCreate(typeID, devices, uid, u, r, ip, ua)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan batch perangkat"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "codes": codes, "message": "Batch perangkat berhasil ditambahkan"})
}

func (h *Handler) DeviceDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	assetCode := c.Param("assetCode")
	d, err := h.deviceService.GetByLabelSlug(assetCode)
	if err != nil {
		h.errHTML(c, "Perangkat tidak ditemukan")
		return
	}

	if strings.ToLower(d.CategoryPrefix) != c.Param("slug") || strings.ToLower(d.DeviceTypePrefix) != c.Param("typeSlug") {
		h.errHTML(c, "Perangkat tidak ditemukan")
		return
	}

	activeLoanIDs, _ := h.deviceService.GetActiveLoanIDs()
	_, isActiveLoan := activeLoanIDs[d.ID]

	// Fetch usage history based on device type
	var loanHistory []repository.DeviceLoanRow
	var usageHistory []repository.DeviceUsageRow
	var installationHistory *models.DeviceInstallation
	isStockEmpty := false

	switch d.UsageType {
	case "loanable":
		loanHistory, _ = h.deviceLoanService.ListByDeviceID(d.ID)
	case "consumable":
		usageHistory, _ = h.deviceUsageService.ListByDeviceID(d.ID)
		if len(usageHistory) > 0 && usageHistory[0].IsAvailable == "no" {
			isStockEmpty = true
		}
	case "installable":
		installationHistory, _ = h.deviceInstallationService.GetByDeviceID(d.ID)
	}

	h.renderTemplate(c, http.StatusOK, "device/detail.html", gin.H{
		"title": "Detail Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"device":              d,
		"isActiveLoan":        isActiveLoan,
		"isStockEmpty":        isStockEmpty,
		"loanHistory":         loanHistory,
		"usageHistory":        usageHistory,
		"installationHistory": installationHistory,
	})
}

func (h *Handler) DeviceEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	slug := c.Param("slug")
	d, err := h.deviceService.GetByLabelSlug(slug)
	if err != nil {
		h.errHTML(c, "Perangkat tidak ditemukan")
		return
	}

	h.renderTemplate(c, http.StatusOK, "device/edit.html", gin.H{
		"title": "Edit Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"device": d,
		"deviceTypes": h.fetchDeviceTypes(),
	})
}

func (h *Handler) DeviceEdit(c *gin.Context) {
	slug := c.Param("slug")
	d, err := h.deviceService.GetByLabelSlug(slug)
	if err != nil {
		h.errHTML(c, "Perangkat tidak ditemukan")
		return
	}

	var req EditDeviceRequest
	if err := c.ShouldBind(&req); err != nil {
		h.errHTML(c, "Data tidak valid")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.deviceService.UpdateDevice(d.ID, services.UpdateDeviceInput{
		DeviceTypeID: req.DeviceTypeID,
		Label:    req.Label,
		SerialNumber: req.SerialNumber,
		Condition:    req.Condition,
		Location:     req.Location,
		PurchaseDate: req.PurchaseDate,
		Notes:        req.Notes,
		UsageType:    req.UsageType,
	}, uid, u, r, ip, ua); err != nil {
		h.errHTML(c, "Gagal mengupdate perangkat")
		return
	}
	h.redirectWithSuccess(c, "/devices?tab=types", "Perangkat berhasil diperbarui", "update")
}

func (h *Handler) DeviceDelete(c *gin.Context) {
	slug := c.Param("slug")
	d, err := h.deviceService.GetByLabelSlug(slug)
	if err != nil {
		h.redirectWithError(c, "/devices?tab=types", "Perangkat tidak ditemukan")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.deviceService.DeleteDevice(d.ID, uid, u, r, ip, ua); err != nil {
		h.redirectWithError(c, "/devices?tab=types", "Gagal menghapus perangkat")
		return
	}
	h.redirectWithSuccess(c, "/devices?tab=types", "Perangkat berhasil dihapus", "delete")
}

func (h *Handler) NextLabel(c *gin.Context) {
	prefix := c.Query("prefix")
	code := h.deviceService.NextLabel(prefix)
	c.JSON(http.StatusOK, gin.H{"next_label": code})
}

func (h *Handler) NextLabels(c *gin.Context) {
	prefix := c.Query("prefix")
	count, _ := strconv.Atoi(c.DefaultQuery("count", "1"))
	if count < 1 {
		count = 1
	}
	if count > 100 {
		count = 100
	}
	codes := h.deviceService.NextLabels(prefix, count)
	c.JSON(http.StatusOK, gin.H{"labels": codes})
}

func (h *Handler) DeviceTypeEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}
	slug := c.Param("slug")
	dt, err := h.deviceTypeService.GetByLabelSlug(slug)
	if err != nil {
		h.errHTML(c, "Tipe perangkat tidak ditemukan")
		return
	}
	h.renderTemplate(c, http.StatusOK, "device_type/edit.html", gin.H{
		"title": "Edit Tipe Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"deviceType":  dt,
		"categories":  h.fetchCategories(""),
		"deviceTypes": h.fetchDeviceTypes(),
		"android":     h.cfg.Android,
	})
}

func (h *Handler) DeviceTypeEdit(c *gin.Context) {
	slug := c.Param("slug")
	dt, err := h.deviceTypeService.GetByLabelSlug(slug)
	if err != nil {
		h.errHTML(c, "Tipe perangkat tidak ditemukan")
		return
	}
	var req EditDeviceTypeRequest
	if err := c.ShouldBind(&req); err != nil {
		h.renderEditPageWithError(c, dt, "Data tidak valid")
		return
	}
	uid, u, r, ok := h.user(c)
	if !ok {
		return
	}
	ip, ua := getRequestContext(c)

	// Process photo ref — fallback ke existing photo jika tidak upload baru
	photoFile := dt.Photo
	if req.PhotoFileRef != "" {
		photoFile, err = processDeviceTypePhotoRef(h.cfg.UploadPath, c.GetString("lab"), req.PhotoFileRef)
		if err != nil {
			h.renderEditPageWithError(c, dt, err.Error())
			return
		}
	}

	if err := h.deviceTypeService.Update(dt.ID, services.DeviceTypeUpdateInput{
		CategoryID:      req.CategoryID,
		Name:            req.Name,
		Brand:           req.Brand,
		Model:           req.Model,
		LabelPrefix: req.LabelPrefix,
		UsageType:       req.UsageType,
		DefaultLocation: req.DefaultLocation,
		Photo:           photoFile,
	}, uid, u, r, ip, ua); err != nil {
		h.renderEditPageWithError(c, dt, err.Error())
		return
	}
	h.redirectWithSuccess(c, "/devices?tab=types", "Tipe perangkat berhasil diperbarui", "update")
}

func (h *Handler) renderEditPageWithError(c *gin.Context, dt *models.DeviceType, errMsg string) {
	_, username, role, _ := h.user(c)
	h.renderTemplate(c, http.StatusBadRequest, "device_type/edit.html", gin.H{
		"title": "Edit Tipe Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"error":       errMsg,
		"deviceType":  dt,
		"categories":  h.fetchCategories(""),
		"deviceTypes": h.fetchDeviceTypes(),
		"android":     h.cfg.Android,
	})
}

func (h *Handler) DeviceTypeDelete(c *gin.Context) {
	slug := c.Param("slug")
	dt, err := h.deviceTypeService.GetByLabelSlug(slug)
	if err != nil {
		h.redirectWithError(c, "/devices?tab=types", "Tipe perangkat tidak ditemukan")
		return
	}
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.deviceTypeService.Delete(dt.ID, uid, u, r, ip, ua); err != nil {
		h.redirectWithError(c, "/devices?tab=types", err.Error())
		return
	}
	h.redirectWithSuccess(c, "/devices?tab=types", "Tipe perangkat berhasil dihapus", "delete")
}

func (h *Handler) DeviceTypeDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	slug := c.Param("slug")
	dt, err := h.deviceTypeService.GetByLabelSlug(slug)
	if err != nil {
		h.errHTML(c, "Tipe perangkat tidak ditemukan")
		return
	}
	deviceCount, _ := h.deviceService.CountByDeviceTypeID(dt.ID)
	devices, _ := h.deviceService.List(repository.DeviceFilters{DeviceTypeID: strconv.Itoa(dt.ID)})
	activeLoanIDs, _ := h.deviceService.GetActiveLoanIDs()
	depletedIDs, _ := h.deviceService.GetDepletedIDs()
	installationStatuses, _ := h.deviceService.GetInstallationStatuses()
	if activeLoanIDs == nil {
		activeLoanIDs = make(map[int]bool)
	}
	if depletedIDs == nil {
		depletedIDs = make(map[int]bool)
	}
	if installationStatuses == nil {
		installationStatuses = make(map[int]string)
	}
	h.renderTemplate(c, http.StatusOK, "device_type/detail.html", gin.H{
		"title": "Detail Tipe Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"deviceType":          dt,
		"deviceCount":         deviceCount,
		"devices":             devices,
		"activeLoanIDs":       activeLoanIDs,
		"depletedIDs":         depletedIDs,
		"installationStatuses": installationStatuses,
	})
}

// ============== Category Handlers ==============

func (h *Handler) CategoryDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}
	slug := c.Param("slug")
	cat, err := h.categoryService.GetByLabelSlug(slug)
	if err != nil {
		h.errHTML(c, "Kategori tidak ditemukan")
		return
	}
	types, _ := h.deviceTypeService.GetByCategoryID(cat.ID)
	deviceCount, _ := h.deviceService.CountByCategoryID(cat.ID)

	h.renderTemplate(c, http.StatusOK, "category/detail.html", gin.H{
		"title": cat.Name, "currentPage": "devices",
		"username": username, "role": role,
		"category":     cat,
		"deviceTypes":  types,
		"deviceCount":  deviceCount,
		"typeCount":    len(types),
	})
}

func (h *Handler) CategoryEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}
	slug := c.Param("slug")
	cat, err := h.categoryService.GetByLabelSlug(slug)
	if err != nil {
		h.errHTML(c, "Kategori tidak ditemukan")
		return
	}
	h.renderTemplate(c, http.StatusOK, "category/edit.html", gin.H{
		"title": "Edit Kategori", "currentPage": "devices",
		"username": username, "role": role,
		"category": cat,
	})
}

func (h *Handler) CategoryEdit(c *gin.Context) {
	slug := c.Param("slug")
	cat, err := h.categoryService.GetByLabelSlug(slug)
	if err != nil {
		h.errHTML(c, "Kategori tidak ditemukan")
		return
	}
	var req EditCategoryRequest
	if err := c.ShouldBind(&req); err != nil {
		h.errHTML(c, "Data tidak valid")
		return
	}
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.categoryService.Update(cat.ID, req.Name, req.LabelPrefix, uid, u, r, ip, ua); err != nil {
		h.errHTML(c, err.Error())
		return
	}
	h.redirectWithSuccess(c, "/devices?tab=types", "Kategori berhasil diperbarui", "update")
}

func (h *Handler) CategoryDelete(c *gin.Context) {
	slug := c.Param("slug")
	cat, err := h.categoryService.GetByLabelSlug(slug)
	if err != nil {
		h.redirectWithError(c, "/devices?tab=types", "Kategori tidak ditemukan")
		return
	}
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.categoryService.Delete(cat.ID, uid, u, r, ip, ua); err != nil {
		h.redirectWithError(c, "/devices?tab=types", err.Error())
		return
	}
	h.redirectWithSuccess(c, "/devices?tab=types", "Kategori berhasil dihapus", "delete")
}

func usageTypePriority(ut string) int {
	switch ut {
	case "loanable":
		return 0
	case "consumable":
		return 1
	case "installable":
		return 2
	default:
		return 3
	}
}

func (h *Handler) DeviceExport(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok {
		return
	}
	if role != "admin" {
		h.errHTML(c, "Hanya admin yang dapat export data")
		return
	}

	devices, err := h.deviceService.List(repository.DeviceFilters{})
	if err != nil {
		h.errHTML(c, "Gagal mengambil data perangkat")
		return
	}

	loans, err := h.deviceLoanService.ExportAll()
	if err != nil {
		h.errHTML(c, "Gagal mengambil data peminjaman")
		return
	}

	usages, err := h.deviceUsageService.ExportAll()
	if err != nil {
		h.errHTML(c, "Gagal mengambil data pemakaian")
		return
	}

	installations, err := h.deviceInstallationService.ExportAll()
	if err != nil {
		h.errHTML(c, "Gagal mengambil data instalasi")
		return
	}

	formatDate := func(t *time.Time) string {
		if t == nil {
			return "-"
		}
		return t.Format("02/01/2006")
	}
	formatTime := func(t time.Time) string {
		return t.Format("02/01/2006")
	}

	sort.SliceStable(devices, func(i, j int) bool {
		pi := usageTypePriority(devices[i].UsageType)
		pj := usageTypePriority(devices[j].UsageType)
		if pi != pj { return pi < pj }
		if devices[i].CategoryName != devices[j].CategoryName { return devices[i].CategoryName < devices[j].CategoryName }
		if devices[i].DeviceTypeName != devices[j].DeviceTypeName { return devices[i].DeviceTypeName < devices[j].DeviceTypeName }
		return devices[i].Label < devices[j].Label
	})
	deviceData := make([][]any, 0, len(devices))
	for i, d := range devices {
		deviceData = append(deviceData, []any{
			i + 1, d.Label, d.SerialNumber, d.DeviceTypeName, d.CategoryName,
			d.Condition, d.Location, d.UsageType, formatDate(d.PurchaseDate), d.Notes,
		})
	}

	loanData := make([][]any, 0, len(loans))
	for i, l := range loans {
		status := l.ComputedStatus
		switch status {
		case "active":
			status = "Masih Dipinjam"
		case "returned":
			status = "Dikembalikan"
		case "overdue":
			status = "Terlambat"
		}
		loanData = append(loanData, []any{
			i + 1, l.DeviceLabel, l.DeviceTypeName, l.CategoryName,
			l.BorrowerName, l.BorrowerType,
			formatTime(l.LoanDate), formatTime(l.ReturnDate), formatDate(l.ActualReturnDate),
			status, l.ExtensionCount, l.Purpose, l.Notes,
		})
	}

	usageData := make([][]any, 0, len(usages))
	for i, u := range usages {
		available := "Habis"
		if u.IsAvailable == "yes" {
			available = "Masih Ada"
		}
		usageData = append(usageData, []any{
			i + 1, u.DeviceLabel, u.DeviceTypeName, u.CategoryName,
			u.UserName, u.UserType, formatTime(u.UsageDate), available, u.Purpose, u.Notes,
		})
	}

	installData := make([][]any, 0, len(installations))
	for i, inst := range installations {
		var status string
		if inst.InstallationFinishDate != nil {
			status = "Selesai"
		} else if inst.InstallationStartDate != nil {
			status = "Berlangsung"
		} else {
			status = "Belum Mulai"
		}
		installData = append(installData, []any{
			i + 1, inst.DeviceLabel, inst.DeviceTypeName, inst.CategoryName,
			inst.LocationInstalled, status,
			formatDate(inst.InstallationStartDate), formatDate(inst.InstallationFinishDate),
		})
	}

	svc := services.NewExcelService()
	f, err := svc.GenerateMultiSheetExcel([]services.ExcelExportConfig{
		{
			SheetName: "Perangkat",
			Headers:   []string{"No", "Asset Code", "Serial Number", "Tipe Device", "Kategori", "Kondisi", "Lokasi", "Usage Type", "Tgl Beli", "Catatan"},
			Data:      deviceData,
			ColumnWidths: map[string]float64{"A": 5, "B": 14, "C": 18, "D": 20, "E": 12, "F": 12, "G": 20, "H": 14, "I": 14, "J": 28},
		},
		{
			SheetName: "Peminjaman",
			Headers:   []string{"No", "Asset Code", "Tipe Device", "Kategori", "Peminjam", "Tipe Peminjam", "Tgl Pinjam", "Deadline", "Tgl Kembali", "Status", "Perpanjangan(x)", "Keperluan", "Catatan"},
			Data:      loanData,
			ColumnWidths: map[string]float64{"A": 5, "B": 14, "C": 18, "D": 12, "E": 24, "F": 14, "G": 14, "H": 14, "I": 14, "J": 16, "K": 16, "L": 20, "M": 28},
		},
		{
			SheetName: "Pemakaian",
			Headers:   []string{"No", "Asset Code", "Tipe Device", "Kategori", "Pengguna", "Tipe Pengguna", "Tgl Pemakaian", "Status Stok", "Keperluan", "Catatan"},
			Data:      usageData,
			ColumnWidths: map[string]float64{"A": 5, "B": 14, "C": 18, "D": 12, "E": 24, "F": 14, "G": 14, "H": 14, "I": 20, "J": 28},
		},
		{
			SheetName: "Instalasi",
			Headers:   []string{"No", "Asset Code", "Tipe Device", "Kategori", "Lokasi Instalasi", "Status", "Tgl Mulai", "Tgl Selesai"},
			Data:      installData,
			ColumnWidths: map[string]float64{"A": 5, "B": 14, "C": 18, "D": 12, "E": 24, "F": 16, "G": 14, "H": 14},
		},
	})
	if err != nil {
		h.errHTML(c, "Gagal membuat file excel")
		return
	}
	defer f.Close()

	fn := svc.GenerateFilename("devices_export")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+fn)
	f.Write(c.Writer)
}

func groupDevices(devices []models.Device, activeLoanIDs, depletedIDs map[int]bool) models.DeviceGroupedData {
	sorted := make([]models.Device, len(devices))
	copy(sorted, devices)
	sort.SliceStable(sorted, func(i, j int) bool {
		pi := usageTypePriority(sorted[i].UsageType)
		pj := usageTypePriority(sorted[j].UsageType)
		if pi != pj {
			return pi < pj
		}
		if sorted[i].CategoryName != sorted[j].CategoryName {
			return sorted[i].CategoryName < sorted[j].CategoryName
		}
		if sorted[i].DeviceTypeName != sorted[j].DeviceTypeName {
			return sorted[i].DeviceTypeName < sorted[j].DeviceTypeName
		}
		return sorted[i].Label < sorted[j].Label
	})

	grouped := models.DeviceGroupedData{
		ActiveLoanIDs: activeLoanIDs,
		DepletedIDs:   depletedIDs,
	}
	var curUsage *models.UsageTypeGroup
	var curCat *models.CategoryGroup
	var curType *models.DeviceTypeGroup
	for _, d := range sorted {
		if curUsage == nil || curUsage.UsageType != d.UsageType {
			grouped.UsageGroups = append(grouped.UsageGroups, models.UsageTypeGroup{
				UsageType: d.UsageType,
			})
			curUsage = &grouped.UsageGroups[len(grouped.UsageGroups)-1]
			curCat = nil
			curType = nil
		}
		if curCat == nil || curCat.CategoryName != d.CategoryName {
			curUsage.Categories = append(curUsage.Categories, models.CategoryGroup{
				CategoryName:   d.CategoryName,
				CategoryPrefix: d.CategoryPrefix,
			})
			curCat = &curUsage.Categories[len(curUsage.Categories)-1]
			curType = nil
		}
		if curType == nil || curType.TypeName != d.DeviceTypeName {
			curCat.Types = append(curCat.Types, models.DeviceTypeGroup{
				TypeName:   d.DeviceTypeName,
				TypePrefix: d.DeviceTypePrefix,
				UsageType:  d.UsageType,
				TypePhoto:  d.DeviceTypePhoto,
			})
			curType = &curCat.Types[len(curCat.Types)-1]
		}
		curType.Devices = append(curType.Devices, d)
	}
	return grouped
}

func processDeviceTypePhotoRef(uploadPath, lab, fileRef string) (string, error) {
	ref := filepath.Base(strings.TrimSpace(fileRef))
	if ref == "" || ref == "." || ref == "/" || ref == "\\" {
		return "", nil
	}
	src := filepath.Join(uploadPath, lab, "temp", ref)
	dst := filepath.Join(uploadPath, lab, "device_types", ref)
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return "", fmt.Errorf("gagal membuat direktori foto: %w", err)
	}
	if err := services.CopyFile(src, dst); err != nil {
		return "", fmt.Errorf("gagal menyalin foto: %w", err)
	}
	os.Remove(src)
	return ref, nil
}

func (h *Handler) DeviceBatchDelete(c *gin.Context) {
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
	if err := h.deviceService.BatchDelete(intIDs, uid, u, r, ip, ua); err != nil {
		h.errJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Perangkat berhasil dihapus"})
}

func (h *Handler) DeviceTypeBatchDelete(c *gin.Context) {
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
	if err := h.deviceTypeService.BatchDelete(intIDs, uid, u, r, ip, ua); err != nil {
		h.errJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Tipe perangkat berhasil dihapus"})
}

func (h *Handler) CategoryBatchDelete(c *gin.Context) {
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
	if err := h.categoryService.BatchDelete(intIDs, uid, u, r, ip, ua); err != nil {
		h.errJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Kategori berhasil dihapus"})
}
