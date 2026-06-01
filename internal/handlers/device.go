package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

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
	var query interface{} = ""
	if len(values) > 0 {
		query = template.URL("&" + values.Encode())
	}

	switch tab {
	case "loans":
		search := c.Query("search")
		status := c.Query("status")
		sortBy := c.Query("sort_by")

		loans, total, err := h.deviceLoanService.ListPaginated(repository.DeviceLoanFilters{
			Search: search,
			Status: status,
			SortBy: sortBy,
		}, page, pageSize)
		if err != nil {
			h.errHTML(c, "Gagal mengambil data peminjaman")
			return
		}

		totalPages := (total + pageSize - 1) / pageSize
		c.HTML(http.StatusOK, "device_loan/list.html", gin.H{
			"title": "Peminjaman", "currentPage": "devices",
			"activeTab": "loans",
			"username": username, "role": role,
			"loans": loans,
			"filters":    gin.H{"search": search, "status": status, "sort_by": sortBy},
			"startRow":   (page-1)*pageSize + 1,
			"page": page, "totalPages": totalPages, "totalItems": total,
			"query": query,
		})

	case "usages":
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
			"title": "Pemakaian Perangkat", "currentPage": "devices",
			"activeTab": "usages",
			"username": username, "role": role,
			"usages": usages,
			"filters":    gin.H{"search": search, "sort_by": sortBy},
			"startRow":   (page-1)*pageSize + 1,
			"page": page, "totalPages": totalPages, "totalItems": total,
			"query": query,
		})

	case "installations":
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
			"activeTab": "installations",
			"username": username, "role": role,
			"installations": installations,
			"filters":       gin.H{"search": search, "sort_by": sortBy},
			"startRow":      (page-1)*pageSize + 1,
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

		groupedData := groupDevices(devices, activeLoanIDs, depletedIDs)

		totalPages := (total + pageSize - 1) / pageSize
		c.HTML(http.StatusOK, "device/list.html", gin.H{
			"title": "Manajemen Perangkat", "currentPage": "devices",
			"activeTab": "types",
			"username": username, "role": role,
			"groupedData":  groupedData,
			"activeLoanIDs": activeLoanIDs,
			"depletedIDs":   depletedIDs,
			"filters":    gin.H{"search": search, "category": category, "condition": condition, "sort_by": sortBy, "sort_order": sortOrder},
			"startRow":   (page-1)*pageSize + 1,
			"page": page, "totalPages": totalPages, "totalItems": total,
			"query":   query,
			"categories": h.fetchCategories(),
		})
	}
}

func (h *Handler) fetchCategories() []models.Category {
	cats, err := h.categoryService.List()
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
	c.HTML(http.StatusOK, "device/create.html", gin.H{
		"title": "Tambah Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"deviceTypes": h.fetchDeviceTypes(),
		"categories":  h.fetchCategories(),
	})
}

func (h *Handler) DeviceCreate(c *gin.Context) {
	var req CreateDeviceRequest
	_, username, role, _ := h.user(c)

	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "device/create.html", gin.H{
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
		c.HTML(http.StatusInternalServerError, "device/create.html", gin.H{
			"title": "Tambah Perangkat", "currentPage": "devices",
			"username": username, "role": role, "error": "Gagal menyimpan perangkat",
			"deviceTypes": h.fetchDeviceTypes(),
		})
		return
	}
	c.Redirect(http.StatusFound, "/devices")
}

func (h *Handler) DeviceBatchCreate(c *gin.Context) {
	var req BatchCreateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data batch tidak valid"})
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

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

	// Resolve device type: create inline if needed
	typeID := req.DeviceTypeID
	if typeID == 0 && req.NewTypeName != "" {
		if catID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Pilih atau buat kategori terlebih dahulu"})
			return
		}
		prefix := req.NewTypeAssetCodePrefix
		if prefix == "" {
			prefix = strings.ToUpper(strings.ReplaceAll(req.NewTypeName, " ", "_"))
		}
		id, err := h.deviceTypeService.Create(services.DeviceTypeCreateInput{
			CategoryID:      catID,
			Name:            req.NewTypeName,
			Brand:           req.NewTypeBrand,
			Model:           req.NewTypeModel,
			AssetCodePrefix: prefix,
			UsageType:       req.NewTypeUsageType,
			DefaultLocation: req.NewTypeDefaultLocation,
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
	c.JSON(http.StatusOK, gin.H{"success": true, "codes": codes})
}

func (h *Handler) DeviceDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	slug := c.Param("slug")
	d, err := h.deviceService.GetByAssetCodeSlug(slug)
	if err != nil {
		h.errHTML(c, "Perangkat tidak ditemukan")
		return
	}

	c.HTML(http.StatusOK, "device/detail.html", gin.H{
		"title": "Detail Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"device": d,
	})
}

func (h *Handler) DeviceEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	slug := c.Param("slug")
	d, err := h.deviceService.GetByAssetCodeSlug(slug)
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
	slug := c.Param("slug")
	d, err := h.deviceService.GetByAssetCodeSlug(slug)
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
		AssetCode:    req.AssetCode,
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
	c.Redirect(http.StatusFound, "/devices")
}

func (h *Handler) DeviceDelete(c *gin.Context) {
	slug := c.Param("slug")
	d, err := h.deviceService.GetByAssetCodeSlug(slug)
	if err != nil {
		h.redirectWithError(c, "/devices", "Perangkat tidak ditemukan")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.deviceService.DeleteDevice(d.ID, uid, u, r, ip, ua); err != nil {
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

func (h *Handler) DeviceTypeEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}
	slug := c.Param("slug")
	dt, err := h.deviceTypeService.GetByPrefixSlug(slug)
	if err != nil {
		h.errHTML(c, "Tipe perangkat tidak ditemukan")
		return
	}
	c.HTML(http.StatusOK, "device_type/edit.html", gin.H{
		"title": "Edit Tipe Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"deviceType":  dt,
		"categories":  h.fetchCategories(),
		"deviceTypes": h.fetchDeviceTypes(),
	})
}

func (h *Handler) DeviceTypeEdit(c *gin.Context) {
	slug := c.Param("slug")
	dt, err := h.deviceTypeService.GetByPrefixSlug(slug)
	if err != nil {
		h.errHTML(c, "Tipe perangkat tidak ditemukan")
		return
	}
	var req EditDeviceTypeRequest
	if err := c.ShouldBind(&req); err != nil {
		h.errHTML(c, "Data tidak valid")
		return
	}
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.deviceTypeService.Update(dt.ID, services.DeviceTypeUpdateInput{
		CategoryID:      req.CategoryID,
		Name:            req.Name,
		Brand:           req.Brand,
		Model:           req.Model,
		AssetCodePrefix: req.AssetCodePrefix,
		UsageType:       req.UsageType,
		DefaultLocation: req.DefaultLocation,
	}, uid, u, r, ip, ua); err != nil {
		h.errHTML(c, err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/devices")
}

func (h *Handler) DeviceTypeDelete(c *gin.Context) {
	slug := c.Param("slug")
	dt, err := h.deviceTypeService.GetByPrefixSlug(slug)
	if err != nil {
		h.redirectWithError(c, "/devices", "Tipe perangkat tidak ditemukan")
		return
	}
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.deviceTypeService.Delete(dt.ID, uid, u, r, ip, ua); err != nil {
		h.redirectWithError(c, "/devices", err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/devices")
}

func (h *Handler) DeviceTypeDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	slug := c.Param("slug")
	dt, err := h.deviceTypeService.GetByPrefixSlug(slug)
	if err != nil {
		h.errHTML(c, "Tipe perangkat tidak ditemukan")
		return
	}
	c.HTML(http.StatusOK, "device_type/detail.html", gin.H{
		"title": "Detail Tipe Perangkat", "currentPage": "devices",
		"username": username, "role": role,
		"deviceType": dt,
		"categories": h.fetchCategories(),
	})
}

// ============== Category Handlers ==============

func (h *Handler) CategoryDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	cat, err := h.categoryService.GetByID(id)
	if err != nil {
		h.errHTML(c, "Kategori tidak ditemukan")
		return
	}
	types, _ := h.deviceTypeService.GetByCategoryID(id)
	deviceCount, _ := h.deviceService.CountByCategoryID(id)

	c.HTML(http.StatusOK, "category/detail.html", gin.H{
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
	id, _ := strconv.Atoi(c.Param("id"))
	cat, err := h.categoryService.GetByID(id)
	if err != nil {
		h.errHTML(c, "Kategori tidak ditemukan")
		return
	}
	c.HTML(http.StatusOK, "category/edit.html", gin.H{
		"title": "Edit Kategori", "currentPage": "devices",
		"username": username, "role": role,
		"category": cat,
	})
}

func (h *Handler) CategoryEdit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req EditCategoryRequest
	if err := c.ShouldBind(&req); err != nil {
		h.errHTML(c, "Data tidak valid")
		return
	}
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.categoryService.Update(id, req.Name, req.DefaultPrefix, uid, u, r, ip, ua); err != nil {
		h.errHTML(c, err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/devices")
}

func (h *Handler) CategoryDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	// Check cascade
	typeCount, _ := h.deviceTypeService.CountByCategoryID(id)
	deviceCount, _ := h.deviceService.CountByCategoryID(id)

	if typeCount > 0 || deviceCount > 0 {
		h.redirectWithError(c, "/devices",
			fmt.Sprintf("Tidak dapat menghapus: masih ada %d tipe dan %d perangkat dalam kategori ini", typeCount, deviceCount))
		return
	}

	if err := h.categoryService.Delete(id, uid, u, r, ip, ua); err != nil {
		h.redirectWithError(c, "/devices", err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/devices")
}

func groupDevices(devices []models.Device, activeLoanIDs, depletedIDs map[int]bool) models.DeviceGroupedData {
	grouped := models.DeviceGroupedData{
		ActiveLoanIDs: activeLoanIDs,
		DepletedIDs:   depletedIDs,
	}
	var curCat *models.CategoryGroup
	var curType *models.DeviceTypeGroup
	for _, d := range devices {
		if curCat == nil || curCat.CategoryName != d.CategoryName {
			grouped.Categories = append(grouped.Categories, models.CategoryGroup{
				CategoryName:   d.CategoryName,
				CategoryPrefix: d.CategoryPrefix,
			})
			curCat = &grouped.Categories[len(grouped.Categories)-1]
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
