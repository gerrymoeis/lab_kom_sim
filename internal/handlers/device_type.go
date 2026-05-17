package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"inventaris-lab-kom/internal/models"

	"github.com/gin-gonic/gin"
)

func (h *Handler) DeviceTypeList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	search := c.Query("search")
	category := c.Query("category")

	types, err := h.deviceTypeRepo.List(category)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data jenis barang")
		return
	}

	if search != "" {
		var filtered []models.DeviceType
		for _, dt := range types {
			if strings.Contains(strings.ToLower(dt.Name), strings.ToLower(search)) ||
				strings.Contains(strings.ToLower(dt.Category), strings.ToLower(search)) {
				filtered = append(filtered, dt)
			}
		}
		types = filtered
	}

	c.HTML(http.StatusOK, "device_type/list.html", gin.H{
		"title": "Jenis Barang", "currentPage": "devices",
		"username": username, "role": role,
		"deviceTypes": types, "search": search, "category": category,
	})
}

func (h *Handler) DeviceTypeDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id, _ := strconv.Atoi(c.Param("id"))
	dt, err := h.deviceTypeRepo.GetByID(id)
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

	result, err := h.deviceTypeRepo.Create(req.Name, req.Category, req.Brand, req.Model, req.ItemType, req.ItemMode, req.AssetCodePrefix, req.DefaultLocation, req.NotesTemplate)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			c.HTML(http.StatusBadRequest, "device_type/create.html", gin.H{
				"title": "Tambah Jenis Barang", "error": "Nama jenis barang sudah ada",
			})
			return
		}
		h.logCreateError(c, "device_type", map[string]interface{}{"name": req.Name}, err.Error())
		c.HTML(http.StatusInternalServerError, "device_type/create.html", gin.H{
			"title": "Tambah Jenis Barang", "error": "Gagal menyimpan data",
		})
		return
	}

	id, _ := result.LastInsertId()
	h.logCreate(c, "device_type", int(id), map[string]interface{}{
		"name": req.Name, "category": req.Category, "item_type": req.ItemType,
	})
	c.Redirect(http.StatusFound, "/device-types")
}

func (h *Handler) DeviceTypeEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id, _ := strconv.Atoi(c.Param("id"))
	dt, err := h.deviceTypeRepo.GetByIDSimple(id)
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

	err := h.deviceTypeRepo.Update(id, req.Name, req.Category, req.Brand, req.Model, req.ItemType, req.ItemMode, req.AssetCodePrefix, req.DefaultLocation, req.NotesTemplate)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			c.HTML(http.StatusBadRequest, "device_type/edit.html", gin.H{
				"title": "Edit Jenis Barang", "error": "Nama sudah digunakan",
			})
			return
		}
		h.logUpdateError(c, "device_type", 0, map[string]interface{}{"id": id}, err.Error())
		h.errHTML(c, "Gagal mengupdate jenis barang")
		return
	}

	h.logUpdate(c, "device_type", 0,
		map[string]interface{}{"id": id},
		map[string]interface{}{"name": req.Name, "category": req.Category},
	)
	c.Redirect(http.StatusFound, "/device-types")
}

func (h *Handler) DeviceTypeDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	err := h.deviceTypeRepo.Delete(id)
	if err != nil {
		if strings.Contains(err.Error(), "foreign key") {
			h.redirectWithError(c, "/device-types", "Jenis barang masih digunakan oleh perangkat")
			return
		}
		h.logDeleteError(c, "device_type", 0, map[string]interface{}{"id": id}, err.Error())
		h.redirectWithError(c, "/device-types", "Gagal menghapus jenis barang")
		return
	}

	h.logDelete(c, "device_type", 0, map[string]interface{}{"id": id})
	c.Redirect(http.StatusFound, "/device-types")
}
