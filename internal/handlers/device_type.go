package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

func (h *Handler) DeviceTypeList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	search := c.Query("search")
	category := c.Query("category")

	types, err := h.deviceTypeService.List(category, search)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data jenis barang")
		return
	}

	c.HTML(http.StatusOK, "device_type/list.html", gin.H{
		"title": "Jenis Barang", "currentPage": "devices",
		"username": username, "role": role,
		"deviceTypes": types, "filters": gin.H{"search": search, "category": category},
	})
}

func (h *Handler) DeviceTypeDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id, _ := strconv.Atoi(c.Param("id"))
	dt, err := h.deviceTypeService.GetByID(id)
	if err != nil {
		h.errHTML(c, "Jenis barang tidak ditemukan")
		return
	}

	c.HTML(http.StatusOK, "device_type/detail.html", gin.H{
		"title": "Detail Jenis Barang", "currentPage": "devices",
		"username": username, "role": role, "deviceType": dt,
	})
}

func (h *Handler) DeviceTypeCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	c.HTML(http.StatusOK, "device_type/create.html", gin.H{
		"title": "Tambah Jenis Barang", "currentPage": "devices",
		"username": username, "role": role,
	})
}

func (h *Handler) DeviceTypeCreate(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Akses ditolak"); return }

	var req CreateDeviceTypeRequest
	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "device_type/create.html", gin.H{
			"title": "Tambah Jenis Barang", "error": "Nama, kategori, dan tipe item harus diisi",
		})
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	_, err := h.deviceTypeService.Create(services.DeviceTypeCreateInput{
		Name: req.Name, Category: req.Category, Brand: req.Brand, Model: req.Model,
		ItemType: req.ItemType, ItemMode: req.ItemMode, AssetCodePrefix: req.AssetCodePrefix,
		DefaultLocation: req.DefaultLocation, NotesTemplate: req.NotesTemplate,
	}, uid, u, r, ip, ua)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			c.HTML(http.StatusBadRequest, "device_type/create.html", gin.H{
				"title": "Tambah Jenis Barang", "error": "Nama jenis barang sudah ada",
			})
			return
		}
		c.HTML(http.StatusInternalServerError, "device_type/create.html", gin.H{
			"title": "Tambah Jenis Barang", "error": "Gagal menyimpan data",
		})
		return
	}
	c.Redirect(http.StatusFound, "/device-types")
}

func (h *Handler) DeviceTypeEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id, _ := strconv.Atoi(c.Param("id"))
	dt, err := h.deviceTypeService.GetByIDSimple(id)
	if err != nil {
		h.errHTML(c, "Jenis barang tidak ditemukan")
		return
	}

	c.HTML(http.StatusOK, "device_type/edit.html", gin.H{
		"title": "Edit Jenis Barang", "currentPage": "devices",
		"username": username, "role": role, "deviceType": dt,
	})
}

func (h *Handler) DeviceTypeEdit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req EditDeviceTypeRequest
	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "device_type/edit.html", gin.H{
			"title": "Edit Jenis Barang", "error": "Nama, kategori, dan tipe item harus diisi",
		})
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	err := h.deviceTypeService.Update(id, services.DeviceTypeUpdateInput{
		Name: req.Name, Category: req.Category, Brand: req.Brand, Model: req.Model,
		ItemType: req.ItemType, ItemMode: req.ItemMode, AssetCodePrefix: req.AssetCodePrefix,
		DefaultLocation: req.DefaultLocation, NotesTemplate: req.NotesTemplate,
	}, uid, u, r, ip, ua)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			c.HTML(http.StatusBadRequest, "device_type/edit.html", gin.H{
				"title": "Edit Jenis Barang", "error": "Nama sudah digunakan",
			})
			return
		}
		h.errHTML(c, "Gagal mengupdate jenis barang")
		return
	}
	c.Redirect(http.StatusFound, "/device-types")
}

func (h *Handler) DeviceTypeDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	err := h.deviceTypeService.Delete(id, uid, u, r, ip, ua)
	if err != nil {
		if strings.Contains(err.Error(), "foreign key") {
			h.redirectWithError(c, "/device-types", "Jenis barang masih digunakan oleh perangkat")
			return
		}
		h.redirectWithError(c, "/device-types", "Gagal menghapus jenis barang")
		return
	}
	c.Redirect(http.StatusFound, "/device-types")
}
