package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"inventaris-lab-kom/internal/repository"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

func (h *Handler) LostItemList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	status := c.Query("status")
	search := c.Query("search")

	items, err := h.lostItemService.List(repository.LostItemFilters{
		Status: status,
		Search: search,
	})
	if err != nil {
		h.errHTML(c, "Gagal mengambil data barang hilang")
		return
	}

	c.HTML(http.StatusOK, "lost_item/list.html", gin.H{
		"title": "Barang Hilang", "currentPage": "lost_items",
		"username": username, "role": role,
		"lostItems": items, "status": status, "search": search,
	})
}

func (h *Handler) LostItemCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	c.HTML(http.StatusOK, "lost_item/create.html", gin.H{
		"title": "Lapor Barang Hilang", "currentPage": "lost_items",
		"username": username, "role": role,
	})
}

func (h *Handler) LostItemCreate(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok {
		return
	}
	if role != "admin" {
		h.errHTML(c, "Akses ditolak")
		return
	}

	var req CreateLostItemRequest
	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "lost_item/create.html", gin.H{
			"title": "Lapor Barang Hilang", "currentPage": "lost_items",
			"error": "Nama barang dan pelapor harus diisi",
		})
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	var deviceID *int
	if req.DeviceID != "" {
		if d, err := strconv.Atoi(req.DeviceID); err == nil && d > 0 {
			deviceID = &d
		}
	}

	photo := processLostItemPhoto(req.Photo)

	_, err := h.lostItemService.Create(services.CreateLostItemInput{
		DeviceID:        deviceID,
		ItemName:        req.ItemName,
		ItemDescription: req.ItemDescription,
		ReportedBy:      req.ReportedBy,
		ReportedDate:    req.ReportedDate,
		LastSeenAt:      req.LastSeenAt,
		LocationLastSeen: req.LocationLastSeen,
		Status:          req.Status,
		Photo:           photo,
	}, uid, u, r, ip, ua)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "lost_item/create.html", gin.H{
			"title": "Lapor Barang Hilang", "currentPage": "lost_items",
			"error": "Gagal menyimpan data",
		})
		return
	}
	c.Redirect(http.StatusFound, "/lost-items")
}

func (h *Handler) LostItemDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	item, err := h.lostItemService.GetByID(id)
	if err != nil {
		h.errHTML(c, "Barang hilang tidak ditemukan")
		return
	}

	c.HTML(http.StatusOK, "lost_item/detail.html", gin.H{
		"title": "Detail Barang Hilang", "currentPage": "lost_items",
		"username": username, "role": role, "lostItem": item,
	})
}

func (h *Handler) LostItemEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	item, err := h.lostItemService.GetByID(id)
	if err != nil {
		h.errHTML(c, "Barang hilang tidak ditemukan")
		return
	}

	c.HTML(http.StatusOK, "lost_item/edit.html", gin.H{
		"title": "Edit Barang Hilang", "currentPage": "lost_items",
		"username": username, "role": role, "lostItem": item,
	})
}

func processLostItemPhoto(photoRef string) string {
	ref := strings.TrimSpace(photoRef)
	if ref == "" {
		return ""
	}
	src := filepath.Join("uploads", "temp", ref)
	dst := filepath.Join("uploads", "lost_items", ref)
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return ""
	}
	if err := services.CopyFile(src, dst); err != nil {
		return ""
	}
	os.Remove(src)
	return ref
}

func (h *Handler) LostItemEdit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req EditLostItemRequest
	if err := c.ShouldBind(&req); err != nil {
		h.errHTML(c, "Data tidak valid")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	var deviceID *int
	if req.DeviceID != "" {
		if d, err := strconv.Atoi(req.DeviceID); err == nil && d > 0 {
			deviceID = &d
		}
	}

	photo := ""
	if req.Photo != "" {
		photo = processLostItemPhoto(req.Photo)
	}

	err := h.lostItemService.Update(id, services.UpdateLostItemInput{
		DeviceID:         deviceID,
		ItemName:         req.ItemName,
		ItemDescription:  req.ItemDescription,
		ReportedBy:       req.ReportedBy,
		ReportedDate:     req.ReportedDate,
		LastSeenAt:       req.LastSeenAt,
		LocationLastSeen: req.LocationLastSeen,
		Status:           req.Status,
		OwnerName:        req.OwnerName,
		OwnerClass:       req.OwnerClass,
		OwnerNim:         req.OwnerNim,
		ReturnedDate:     req.ReturnedDate,
		Photo:            photo,
	}, uid, u, r, ip, ua)
	if err != nil {
		h.errHTML(c, "Gagal mengupdate data")
		return
	}
	c.Redirect(http.StatusFound, "/lost-items")
}

func (h *Handler) LostItemDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	oldPhoto, _ := h.lostItemService.GetByID(id)
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.lostItemService.Delete(id, uid, u, r, ip, ua); err != nil {
		h.redirectWithError(c, "/lost-items", "Gagal menghapus data")
		return
	}

	if oldPhoto != nil && oldPhoto.Photo != "" {
		os.Remove(filepath.Join("uploads", "lost_items", oldPhoto.Photo))
	}

	c.Redirect(http.StatusFound, "/lost-items")
}
