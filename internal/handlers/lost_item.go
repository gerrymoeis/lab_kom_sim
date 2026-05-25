package handlers

import (
	"html/template"
	"log"
	"net/http"
	"net/url"
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

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 { page = 1 }
	pageSize := 20

	values, _ := url.ParseQuery(c.Request.URL.RawQuery)
	delete(values, "page")
	var query interface{} = ""
	if len(values) > 0 { query = template.URL("&" + values.Encode()) }

	status := c.Query("status")
	search := c.Query("search")
	sortBy := c.Query("sort_by")
	sortOrder := c.Query("sort_order")

	items, total, err := h.lostItemService.ListPaginated(repository.LostItemFilters{
		Status:    status,
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}, page, pageSize)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data barang hilang")
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	startRow := (page-1)*pageSize + 1

	c.HTML(http.StatusOK, "lost_item/list.html", gin.H{
		"title": "Barang Hilang", "currentPage": "lost_items",
		"username": username, "role": role,
		"lostItems": items, "filters": gin.H{"search": search, "status": status, "sort_by": sortBy, "sort_order": sortOrder},
		"page": page, "startRow": startRow, "totalPages": totalPages, "totalItems": total,
		"query": query,
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
		"android": h.cfg.Android,
	})
}

func (h *Handler) LostItemCreate(c *gin.Context) {
	log.Printf("[DEBUG-LOST] ===== LostItemCreate CALLED =====")
	_, _, role, ok := h.user(c)
	if !ok {
		log.Printf("[DEBUG-LOST] user not authenticated, redirecting")
		return
	}
	if role != "admin" {
		log.Printf("[DEBUG-LOST] role=%s not admin, access denied", role)
		h.errHTML(c, "Akses ditolak")
		return
	}

	var req CreateLostItemRequest
	if err := c.ShouldBind(&req); err != nil {
		log.Printf("[DEBUG-LOST] ShouldBind error: %v", err)
		c.HTML(http.StatusBadRequest, "lost_item/create.html", gin.H{
			"title": "Lapor Barang Hilang", "currentPage": "lost_items",
			"error": "Nama barang dan pelapor harus diisi",
		})
		return
	}

	log.Printf("[DEBUG-LOST] form values: item_name=%q reported_by=%q photo=%q status=%q",
		req.ItemName, req.ReportedBy, req.Photo, req.Status)

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	var deviceID *int
	if req.DeviceID != "" {
		if d, err := strconv.Atoi(req.DeviceID); err == nil && d > 0 {
			deviceID = &d
			log.Printf("[DEBUG-LOST] device_id=%d", *deviceID)
		}
	}

	log.Printf("[DEBUG-LOST] calling processLostItemPhoto with photo=%q", req.Photo)
	photo := processLostItemPhoto(req.Photo)
	log.Printf("[DEBUG-LOST] processLostItemPhoto returned photo=%q", photo)

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
		"android": h.cfg.Android,
	})
}

func processLostItemPhoto(photoRef string) string {
	ref := strings.TrimSpace(photoRef)
	log.Printf("[DEBUG-LOST] processLostItemPhoto: ref=%q", ref)
	if ref == "" {
		log.Printf("[DEBUG-LOST] processLostItemPhoto: empty ref, returning empty")
		return ""
	}
	src := filepath.Join("uploads", "temp", ref)
	dst := filepath.Join("uploads", "lost_items", ref)
	log.Printf("[DEBUG-LOST] processLostItemPhoto: src=%s dst=%s", src, dst)
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		log.Printf("[DEBUG-LOST] processLostItemPhoto: MkdirAll error: %v", err)
		return ""
	}
	log.Printf("[DEBUG-LOST] processLostItemPhoto: CopyFile %s -> %s", src, dst)
	if err := services.CopyFile(src, dst); err != nil {
		log.Printf("[DEBUG-LOST] processLostItemPhoto: CopyFile error: %v", err)
		return ""
	}
	log.Printf("[DEBUG-LOST] processLostItemPhoto: removing original %s", src)
	os.Remove(src)
	log.Printf("[DEBUG-LOST] processLostItemPhoto: SUCCESS, returning ref=%q", ref)
	return ref
}

func (h *Handler) LostItemEdit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	log.Printf("[DEBUG-LOST] ===== LostItemEdit CALLED id=%d =====", id)
	var req EditLostItemRequest
	if err := c.ShouldBind(&req); err != nil {
		log.Printf("[DEBUG-LOST] ShouldBind error: %v", err)
		h.errHTML(c, "Data tidak valid")
		return
	}

	log.Printf("[DEBUG-LOST] edit form values: item_name=%q reported_by=%q photo=%q status=%q",
		req.ItemName, req.ReportedBy, req.Photo, req.Status)

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	var deviceID *int
	if req.DeviceID != "" {
		if d, err := strconv.Atoi(req.DeviceID); err == nil && d > 0 {
			deviceID = &d
		}
	}

	log.Printf("[DEBUG-LOST] req.Photo=%q — will %s", req.Photo, map[bool]string{true: "process new photo", false: "preserve existing photo (COALESCE)"}[req.Photo != ""])
	photo := ""
	if req.Photo != "" {
		photo = processLostItemPhoto(req.Photo)
		log.Printf("[DEBUG-LOST] processLostItemPhoto returned photo=%q", photo)
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
