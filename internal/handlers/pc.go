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

	operatingSystems, _ := h.pcService.GetDistinctOS()

	totalPages := (total + pageSize - 1) / pageSize
	startRow := (page-1)*pageSize + 1

	c.HTML(http.StatusOK, "pc/list.html", gin.H{
		"title": "Manajemen PC", "currentPage": "pc",
		"username": username, "role": role, "pcs": pcs,
		"page": page, "totalPages": totalPages, "totalItems": total,
		"startRow": startRow,
		"query": query, "filters": gin.H{"search": filters.Search, "status": filters.Status, "placement": filters.Placement, "sort_by": filters.SortBy, "sort_order": filters.SortOrder, "os": filters.OS},
		"operatingSystems": operatingSystems,
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
	operatingSystems, _ := h.pcService.GetDistinctOS()
	c.HTML(http.StatusOK, "pc/create.html", gin.H{
		"title": "Tambah PC Baru", "currentPage": "pc",
		"username": username, "role": role,
		"android": h.cfg.Android,
		"nextMahasiswaLabel": h.pcService.NextLabel("dipakai", true),
		"nextCadanganLabel":  h.pcService.NextLabel("cadangan", false),
		"operatingSystems":   operatingSystems,
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
		Label: req.Label, IsMahasiswa: req.IsMahasiswa == "true",
		PurchaseDate: req.PurchaseDate,
		LastChecked: req.LastChecked,
		Notes: req.Notes,
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

	pd := ""
	if pc.PurchaseDate != nil { pd = pc.PurchaseDate.Format("2006-01-02") }
	lc := ""
	lcDisplay := ""
	if pc.LastChecked != nil {
		lc = pc.LastChecked.Format("2006-01-02T15:04")
		lcDisplay = pc.LastChecked.Format("02/01/2006 15:04")
	}

	isRegular := len(pc.Label) > 3 && strings.HasPrefix(pc.Label, "pc-")
	if isRegular {
		for _, c := range pc.Label[3:] {
			if c < '0' || c > '9' {
				isRegular = false
				break
			}
		}
	}

	operatingSystems, _ := h.pcService.GetDistinctOS()

	c.HTML(http.StatusOK, "pc/edit.html", gin.H{
		"title": "Edit PC", "currentPage": "pc",
		"username": username, "role": role, "pc": pc,
		"requiredSW": requiredSW, "otherSW": otherSW,
		"android": h.cfg.Android,
		"purchaseDate": pd,
		"lastChecked": lc,
		"lastCheckedDisplay": lcDisplay,
		"isRegularPC": isRegular,
		"operatingSystems": operatingSystems,
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
		Row: req.Row, Column: req.Column,
		Status: req.Status, Placement: req.Placement,
		SerialNumber: req.SerialNumber, BrandModel: req.BrandModel,
		Accessories: req.Accessories, Processor: req.Processor,
		PCType: req.PCType,
		RAM: req.RAM, Storage: req.Storage, OperatingSystem: req.OperatingSystem,
		Notes: req.Notes,
		PhotoSerial: photoSerial, PhotoFront: photoFront,
		Label: newLabel,
		PurchaseDate: req.PurchaseDate,
		LastChecked: req.LastChecked,
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
	label := c.Param("label")
	pc, err := h.pcService.GetByLabel(label)
	if err != nil {
		h.errJSON(c, http.StatusNotFound, "PC tidak ditemukan")
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errJSON(c, http.StatusBadRequest, "Data tidak valid")
		return
	}
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	if err := h.pcService.UpdateStatus(pc.ID, req.Status, uid, u, r, ip, ua); err != nil {
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

type pcLayoutItem struct {
	Label     string `json:"label"`
	Row       int    `json:"row"`
	Column    int    `json:"column"`
	Status    string `json:"status"`
	Placement string `json:"placement"`
}

func isNumericLabel(label string) bool {
	if len(label) < 4 || !strings.HasPrefix(label, "pc-") {
		return false
	}
	for _, c := range label[3:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (h *Handler) PCGetLayout(c *gin.Context) {
	pcs, err := h.pcService.List(repository.PCFilters{})
	if err != nil {
		h.errJSON(c, http.StatusInternalServerError, "Gagal mengambil data layout")
		return
	}

	maxRow := 5
	for _, pc := range pcs {
		if pc.Placement == "dipakai" && isNumericLabel(pc.Label) && pc.Row > maxRow {
			maxRow = pc.Row
		}
	}

	grid := make([][]pcLayoutItem, maxRow)
	for i := range grid {
		grid[i] = make([]pcLayoutItem, 8)
	}
	var cadangan, special []pcLayoutItem

	for _, pc := range pcs {
		item := pcLayoutItem{Label: pc.Label, Row: pc.Row, Column: pc.Column, Status: pc.Status, Placement: pc.Placement}
		if pc.Placement == "cadangan" || (pc.Placement == "dipakai" && isNumericLabel(pc.Label) && (pc.Row < 1 || pc.Column < 1)) {
			cadangan = append(cadangan, item)
		} else if pc.Placement == "dipakai" && isNumericLabel(pc.Label) && pc.Row >= 1 && pc.Row <= maxRow && pc.Column >= 1 && pc.Column <= 8 {
			grid[pc.Row-1][pc.Column-1] = item
		} else {
			special = append(special, item)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"grid":     grid,
		"cadangan": cadangan,
		"special":  special,
		"maxRow":   maxRow,
	})
}

func (h *Handler) PCSwap(c *gin.Context) {
	var req struct {
		A string `json:"a" binding:"required"`
		B string `json:"b" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errJSON(c, http.StatusBadRequest, "Data tidak valid")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.pcService.SwapPCs(req.A, req.B, uid, u, r, ip, ua); err != nil {
		h.errJSON(c, http.StatusInternalServerError, "Gagal menukar PC")
		return
	}

	pcs, _ := h.pcService.List(repository.PCFilters{})
	c.JSON(http.StatusOK, gin.H{"success": true, "pcs": pcs})
}

func (h *Handler) PCReplace(c *gin.Context) {
	var req struct {
		Target string `json:"target" binding:"required"`
		Spare  string `json:"spare" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errJSON(c, http.StatusBadRequest, "Data tidak valid")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.pcService.ReplacePC(req.Target, req.Spare, uid, u, r, ip, ua); err != nil {
		h.errJSON(c, http.StatusInternalServerError, "Gagal mengganti PC")
		return
	}

	pcs, _ := h.pcService.List(repository.PCFilters{})
	c.JSON(http.StatusOK, gin.H{"success": true, "pcs": pcs})
}

func (h *Handler) PCMoveRowToCadangan(c *gin.Context) {
	var req struct {
		Row int `json:"row" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errJSON(c, http.StatusBadRequest, "Data tidak valid")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.pcService.MoveRowToCadangan(req.Row, uid, u, r, ip, ua); err != nil {
		h.errJSON(c, http.StatusInternalServerError, "Gagal memindahkan baris")
		return
	}

	pcs, _ := h.pcService.List(repository.PCFilters{})
	c.JSON(http.StatusOK, gin.H{"success": true, "pcs": pcs})
}

func (h *Handler) PCMove(c *gin.Context) {
	var req struct {
		Label string `json:"label" binding:"required"`
		Row   int    `json:"row" binding:"required"`
		Col   int    `json:"col" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errJSON(c, http.StatusBadRequest, "Data tidak valid")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.pcService.MovePC(req.Label, req.Row, req.Col, uid, u, r, ip, ua); err != nil {
		h.errJSON(c, http.StatusInternalServerError, "Gagal memindahkan PC")
		return
	}

	pcs, _ := h.pcService.List(repository.PCFilters{})
	c.JSON(http.StatusOK, gin.H{"success": true, "pcs": pcs})
}

func (h *Handler) PCPlace(c *gin.Context) {
	var req struct {
		Label string `json:"label" binding:"required"`
		Row   int    `json:"row" binding:"required"`
		Col   int    `json:"col" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errJSON(c, http.StatusBadRequest, "Data tidak valid")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.pcService.PlaceCadangan(req.Label, req.Row, req.Col, uid, u, r, ip, ua); err != nil {
		h.errJSON(c, http.StatusInternalServerError, "Gagal menempatkan PC cadangan")
		return
	}

	pcs, _ := h.pcService.List(repository.PCFilters{})
	c.JSON(http.StatusOK, gin.H{"success": true, "pcs": pcs})
}

func processPhotoRef(photoRef, subDir string) string {
	ref := strings.TrimSpace(photoRef)
	if ref == "" {
		return ""
	}
	src := filepath.Join("uploads", "temp", ref)
	dst := filepath.Join("uploads", subDir, ref)
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return ""
	}
	if err := services.CopyFile(src, dst); err != nil {
		return ""
	}
	os.Remove(src)
	return ref
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
