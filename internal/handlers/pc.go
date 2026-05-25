package handlers

import (
	"fmt"
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

func (h *Handler) PCList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 { page = 1 }
	pageSize := 20

	values, _ := url.ParseQuery(c.Request.URL.RawQuery)
	delete(values, "page")
	var query interface{} = ""
	if len(values) > 0 { query = template.URL("&" + values.Encode()) }

	filters := repository.PCFilters{
		Search:    c.Query("search"),
		Status:    c.Query("status"),
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
		"query": query, "filters": gin.H{"search": filters.Search, "status": filters.Status, "sort_by": filters.SortBy, "sort_order": filters.SortOrder, "os": filters.OS},
	})
}

func (h *Handler) PCDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	num, _ := strconv.Atoi(c.Param("pc_number"))
	pc, err := h.pcService.GetByPCNumber(num)
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
	log.Printf("[DEBUG-PC] ===== PCCreate CALLED =====")
	log.Printf("[DEBUG-PC] Content-Type: %s", c.Request.Header.Get("Content-Type"))
	var req CreatePCRequest
	if err := c.ShouldBind(&req); err != nil {
		log.Printf("[DEBUG-PC] ShouldBind error: %v", err)
		_, username, role, _ := h.user(c)
		c.HTML(http.StatusBadRequest, "pc/create.html", gin.H{
			"title": "Tambah PC Baru", "error": "Lengkapi data yang diperlukan",
			"currentPage": "pc", "username": username, "role": role,
		})
		return
	}

	log.Printf("[DEBUG-PC] form values: pc_number=%d serial_file_ref=%q front_file_ref=%q serial_number=%q os=%q",
		req.PCNumber, req.SerialFileRef, req.FrontFileRef, req.SerialNumber, req.OperatingSystem)

	log.Printf("[DEBUG-PC] calling processPhotoRefs: serial=%q front=%q", req.SerialFileRef, req.FrontFileRef)
	photoSerial, photoFront := processPhotoRefs(req.SerialFileRef, req.FrontFileRef)
	log.Printf("[DEBUG-PC] processPhotoRefs returned: serial=%q front=%q", photoSerial, photoFront)
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	_, err := h.pcService.CreatePC(services.CreatePCInput{
		PCNumber: req.PCNumber, Row: req.Row, Column: req.Column,
		Status: req.Status, Processor: req.Processor, RAM: req.RAM, Storage: req.Storage,
		SerialNumber: req.SerialNumber, OperatingSystem: req.OperatingSystem,
		DeviceType: req.DeviceType, BrandModel: req.BrandModel, Accessories: req.Accessories,
		PhotoSerial: photoSerial, PhotoFront: photoFront,
		Label: req.Label,
	}, uid, u, r, ip, ua)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "pc/create.html", gin.H{
			"title": "Tambah PC Baru", "error": "Gagal menyimpan. Mungkin nomor PC sudah digunakan.",
			"currentPage": "pc", "username": u, "role": r,
		})
		return
	}
	c.Redirect(http.StatusFound, "/pc")
}

func (h *Handler) PCEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	num, _ := strconv.Atoi(c.Param("pc_number"))
	pc, err := h.pcService.GetByPCNumberEdit(num)
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
	num, _ := strconv.Atoi(c.Param("pc_number"))

	var req EditPCRequest
	if err := c.ShouldBind(&req); err != nil {
		h.errHTML(c, "Data tidak valid")
		return
	}

	photoSerial, photoFront := processPhotoRefs(req.SerialFileRef, req.FrontFileRef)
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.pcService.UpdatePC(num, services.UpdatePCInput{
		Status: req.Status, DeviceType: req.DeviceType,
		SerialNumber: req.SerialNumber, BrandModel: req.BrandModel,
		Accessories: req.Accessories, Processor: req.Processor,
		RAM: req.RAM, Storage: req.Storage, OperatingSystem: req.OperatingSystem,
		Notes: req.Notes, ActionNotes: req.ActionNotes,
		PhotoSerial: photoSerial, PhotoFront: photoFront,
		Label: req.Label,
	}, uid, u, r, ip, ua); err != nil {
		h.errHTML(c, "Gagal mengupdate PC")
		return
	}

	h.pcService.SyncSoftware(num, req.RequiredSw, req.OtherName, req.OtherDesc, uid, u, r, ip, ua)
	c.Redirect(http.StatusFound, fmt.Sprintf("/pc/%d", num))
}

func (h *Handler) PCDelete(c *gin.Context) {
	num, _ := strconv.Atoi(c.Param("pc_number"))
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.pcService.DeletePC(num, uid, u, r, ip, ua); err != nil {
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
		data = append(data, []any{pc.PCNumber, pos, pc.Status, pc.DeviceType, pc.SerialNumber, pc.BrandModel, pc.Processor, pc.RAM, pc.Storage, pc.OperatingSystem, pc.Accessories, pd, ld, pc.Notes, pc.ActionNotes})
	}
	f, _ := svc.GenerateMultiSheetExcel([]services.ExcelExportConfig{
		{
			SheetName: "PC",
			Headers:   []string{"No PC", "Posisi", "Status", "Tipe", "Serial Number", "Brand/Model", "Processor", "RAM", "Storage", "OS", "Accessories", "Tgl Beli", "Tgl Cek", "Catatan", "Tindakan"},
			Data:      data,
			ColumnWidths: map[string]float64{"A": 8, "B": 10, "C": 12, "D": 18, "E": 20, "F": 32, "G": 18, "H": 12, "I": 14, "J": 14, "K": 36, "L": 14, "M": 16, "N": 28, "O": 28},
		},
	})
	defer f.Close()

	fn := svc.GenerateFilename("pc_export")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+fn)
	f.Write(c.Writer)
}

func processPhotoRefs(serialRef, frontRef string) (serial, front string) {
	log.Printf("[DEBUG-PC] processPhotoRefs: serialRef=%q frontRef=%q", serialRef, frontRef)
	for _, p := range []struct{ ref string; result *string }{
		{serialRef, &serial}, {frontRef, &front},
	} {
		ref := strings.TrimSpace(p.ref)
		if ref == "" {
			log.Printf("[DEBUG-PC] processPhotoRefs: ref empty, skipping")
			continue
		}
		src := filepath.Join("uploads", "temp", ref)
		dst := filepath.Join("uploads", "pc", ref)
		log.Printf("[DEBUG-PC] processPhotoRefs: copying %s -> %s", src, dst)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			log.Printf("[DEBUG-PC] processPhotoRefs: MkdirAll error: %v", err)
			continue
		}
		if err := services.CopyFile(src, dst); err != nil {
			log.Printf("[DEBUG-PC] processPhotoRefs: CopyFile error: %v", err)
			continue
		}
		log.Printf("[DEBUG-PC] processPhotoRefs: removing source %s", src)
		os.Remove(src)
		*p.result = ref
		log.Printf("[DEBUG-PC] processPhotoRefs: set result=%q", ref)
	}
	return
}
