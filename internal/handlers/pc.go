package handlers

import (
	"fmt"
	"html/template"
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

func (h *Handler) PCList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 { page = 1 }
	pageSize := h.cfg.DefaultPageSize

	values, _ := url.ParseQuery(c.Request.URL.RawQuery)
	delete(values, "page")
	var query interface{} = ""
	if len(values) > 0 { query = template.URL("&" + values.Encode()) }

	filters := repository.PCFilters{
		Search:    c.Query("search"),
		Status:    c.Query("status"),
		Placement: c.Query("placement"),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
		OS:        c.Query("os"),
	}
	pcs, total, err := h.pcService.ListPaginated(filters, page, pageSize)
	if err != nil { h.errHTML(c, "Gagal mengambil data PC"); return }

	totalPages := (total + pageSize - 1) / pageSize
	startRow := (page-1)*pageSize + 1

	c.HTML(http.StatusOK, "pc/list.html", gin.H{
		"title": "Manajemen PC", "currentPage": "pc",
		"username": username, "role": role, "pcs": pcs,
		"page": page, "totalPages": totalPages, "totalItems": total,
		"startRow": startRow,
		"query": query, "filters": gin.H{"search": filters.Search, "status": filters.Status, "placement": filters.Placement, "sort_by": filters.SortBy, "sort_order": filters.SortOrder, "os": filters.OS},
	})
}

func (h *Handler) PCDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	label := c.Param("label")
	pc, err := h.pcService.GetByLabel(label)
	if err != nil { h.errHTML(c, "PC tidak ditemukan"); return }

	requiredSW, otherSW, _ := h.pcService.GetSoftware(pc.ID)

	lcFormatted := ""
	if pc.LastChecked != nil { lcFormatted = pc.LastChecked.Format("02/01/2006 15:04") }
	c.HTML(http.StatusOK, "pc/detail.html", gin.H{
		"title": "Detail PC", "currentPage": "pc",
		"username": username, "role": role, "pc": pc,
		"requiredSW": requiredSW, "otherSW": otherSW,
		"lastCheckedFormatted": lcFormatted,
	})
}

func (h *Handler) PCCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	c.HTML(http.StatusOK, "pc/create.html", gin.H{
		"title": "Tambah PC Baru", "currentPage": "pc",
		"username": username, "role": role,
		"android": h.cfg.Android,
	})
}

func (h *Handler) PCCreate(c *gin.Context) {
	var req CreatePCRequest
	if err := c.ShouldBind(&req); err != nil {
		_, username, role, _ := h.user(c)
		c.HTML(http.StatusBadRequest, "pc/create.html", gin.H{
			"title": "Tambah PC Baru", "error": "Lengkapi data yang diperlukan",
			"currentPage": "pc", "username": username, "role": role,
		})
		return
	}

	photoSerial, photoFront := processPhotoRefs(req.SerialFileRef, req.FrontFileRef)
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	_, err := h.pcService.CreatePC(services.CreatePCInput{
		Row: req.Row, Column: req.Column,
		Status: req.Status, Placement: req.Placement,
		Processor: req.Processor, RAM: req.RAM, Storage: req.Storage,
		SerialNumber: req.SerialNumber, OperatingSystem: req.OperatingSystem,
		PCType: req.PCType, BrandModel: req.BrandModel, Accessories: req.Accessories,
		PhotoSerial: photoSerial, PhotoFront: photoFront,
		Label: req.Label,
	}, uid, u, r, ip, ua)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "pc/create.html", gin.H{
			"title": "Tambah PC Baru", "error": "Gagal menyimpan. Mungkin label PC sudah digunakan.",
			"currentPage": "pc", "username": u, "role": r,
		})
		return
	}
	c.Redirect(http.StatusFound, "/pc")
}

func (h *Handler) PCEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	label := c.Param("label")
	pc, err := h.pcService.GetByLabelEdit(label)
	if err != nil { h.errHTML(c, "PC tidak ditemukan"); return }

	requiredSW, otherSW, _ := h.pcService.GetSoftware(pc.ID)

	c.HTML(http.StatusOK, "pc/edit.html", gin.H{
		"title": "Edit PC", "currentPage": "pc",
		"username": username, "role": role, "pc": pc,
		"requiredSW": requiredSW, "otherSW": otherSW,
		"android": h.cfg.Android,
	})
}

func (h *Handler) PCEdit(c *gin.Context) {
	label := c.Param("label")

	var req EditPCRequest
	if err := c.ShouldBind(&req); err != nil {
		h.errHTML(c, "Data tidak valid")
		return
	}

	photoSerial, photoFront := processPhotoRefs(req.SerialFileRef, req.FrontFileRef)
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	newLabel := req.Label
	if newLabel == "" {
		newLabel = label
	}

	if err := h.pcService.UpdatePC(label, services.UpdatePCInput{
		Status: req.Status, Placement: req.Placement,
		SerialNumber: req.SerialNumber, BrandModel: req.BrandModel,
		Accessories: req.Accessories, Processor: req.Processor,
		PCType: req.PCType,
		RAM: req.RAM, Storage: req.Storage, OperatingSystem: req.OperatingSystem,
		Notes: req.Notes,
		PhotoSerial: photoSerial, PhotoFront: photoFront,
		Label: newLabel,
	}, uid, u, r, ip, ua); err != nil {
		h.errHTML(c, "Gagal mengupdate PC")
		return
	}

	h.pcService.SyncSoftware(newLabel, req.RequiredSw, req.OtherName, req.OtherDesc, uid, u, r, ip, ua)
	c.Redirect(http.StatusFound, fmt.Sprintf("/pc/%s", newLabel))
}

func (h *Handler) PCDelete(c *gin.Context) {
	label := c.Param("label")
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.pcService.DeletePC(label, uid, u, r, ip, ua); err != nil {
		h.redirectWithError(c, "/pc", "Gagal menghapus PC. Mungkin PC sedang dipinjam.")
		return
	}
	c.Redirect(http.StatusFound, "/pc")
}

func (h *Handler) PCStatusAPI(c *gin.Context) {
	pcs, err := h.pcService.List(repository.PCFilters{})
	if err != nil {
		h.errJSON(c, http.StatusInternalServerError, "Gagal mengambil data")
		return
	}
	statusCounts := make(map[string]int)
	for _, pc := range pcs { statusCounts[pc.Status]++ }
	c.JSON(http.StatusOK, gin.H{"counts": statusCounts, "pcs": pcs})
}

func (h *Handler) UpdatePCStatusAPI(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct { Status string `json:"status" form:"status"` }
	if err := c.ShouldBind(&req); err != nil {
		h.errJSON(c, http.StatusBadRequest, "Status diperlukan")
		return
	}
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	if err := h.pcService.UpdateStatus(id, req.Status, uid, u, r, ip, ua); err != nil {
		h.errJSON(c, http.StatusInternalServerError, "Gagal mengupdate status")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) PCExport(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat export data"); return }

	pcs, _ := h.pcService.ExportAll()
	svc := services.NewExcelService()
	data := make([][]any, 0, len(pcs))
	for _, pc := range pcs {
		pos := fmt.Sprintf("(%d,%d)", pc.Row, pc.Column)
		pd := "-"; if pc.PurchaseDate != nil { pd = pc.PurchaseDate.Format("2006-01-02") }
		ld := "-"; if pc.LastChecked != nil { ld = pc.LastChecked.Format("2006-01-02") }
		data = append(data, []any{pc.Label, pos, pc.Status, pc.Placement, pc.PCType, pc.SerialNumber, pc.BrandModel, pc.Processor, pc.RAM, pc.Storage, pc.OperatingSystem, pc.Accessories, pd, ld, pc.Notes})
	}
	f, _ := svc.GenerateMultiSheetExcel([]services.ExcelExportConfig{
		{
			SheetName: "PC",
			Headers:   []string{"No PC", "Posisi", "Status", "Penempatan", "Jenis PC", "Serial Number", "Brand/Model", "Processor", "RAM", "Storage", "OS", "Accessories", "Tgl Beli", "Tgl Cek", "Catatan"},
			Data:      data,
			ColumnWidths: map[string]float64{"A": 8, "B": 10, "C": 12, "D": 14, "E": 18, "F": 20, "G": 32, "H": 18, "I": 12, "J": 14, "K": 14, "L": 36, "M": 14, "N": 16, "O": 28},
		},
	})
	defer f.Close()

	fn := svc.GenerateFilename("pc_export")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+fn)
	f.Write(c.Writer)
}

func processPhotoRefs(serialRef, frontRef string) (serial, front string) {
	for _, p := range []struct{ ref string; result *string }{
		{serialRef, &serial}, {frontRef, &front},
	} {
		ref := strings.TrimSpace(p.ref)
		if ref == "" {
			continue
		}
		src := filepath.Join("uploads", "temp", ref)
		dst := filepath.Join("uploads", "pc", ref)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			continue
		}
		if err := services.CopyFile(src, dst); err != nil {
			continue
		}
		os.Remove(src)
		*p.result = ref
	}
	return
}
