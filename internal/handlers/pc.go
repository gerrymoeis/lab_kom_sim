package handlers

import (
	"fmt"
	"net/http"
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

	pcs, err := h.pcRepo.List(repository.PCFilters{})
	if err != nil { h.errHTML(c, "Gagal mengambil data PC"); return }

	c.HTML(http.StatusOK, "pc/list.html", gin.H{
		"title": "Manajemen PC", "currentPage": "pc",
		"username": username, "role": role, "pcs": pcs,
	})
}

func (h *Handler) PCDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	num, _ := strconv.Atoi(c.Param("pc_number"))
	pc, err := h.pcRepo.GetByPCNumber(num)
	if err != nil { h.errHTML(c, "PC tidak ditemukan"); return }

	requiredSW, otherSW, _ := h.pcRepo.GetSoftware(pc.ID)

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
	})
}

func (h *Handler) PCCreate(c *gin.Context) {
	var req CreatePCRequest
	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "pc/create.html", gin.H{
			"title": "Tambah PC Baru", "error": "Lengkapi data yang diperlukan",
		})
		return
	}

	photoSerial, photoFront := processPhotoRefs(c)
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	_, err := h.pcService.CreatePC(services.CreatePCInput{
		PCNumber: req.PCNumber, Row: req.Row, Column: req.Column,
		Status: req.Status, Processor: req.Processor, RAM: req.RAM, Storage: req.Storage,
		SerialNumber: req.SerialNumber, OperatingSystem: req.OperatingSystem,
		DeviceType: req.DeviceType, BrandModel: req.BrandModel, Accessories: req.Accessories,
		PhotoSerial: photoSerial, PhotoFront: photoFront,
	}, uid, u, r, ip, ua)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "pc/create.html", gin.H{
			"title": "Tambah PC Baru", "error": "Gagal menyimpan. Mungkin nomor PC sudah digunakan.",
		})
		return
	}
	c.Redirect(http.StatusFound, "/pc")
}

func (h *Handler) PCEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	num, _ := strconv.Atoi(c.Param("pc_number"))
	pc, err := h.pcRepo.GetByPCNumberEdit(num)
	if err != nil { h.errHTML(c, "PC tidak ditemukan"); return }

	requiredSW, otherSW, _ := h.pcRepo.GetSoftware(pc.ID)

	c.HTML(http.StatusOK, "pc/edit.html", gin.H{
		"title": "Edit PC", "currentPage": "pc",
		"username": username, "role": role, "pc": pc,
		"requiredSW": requiredSW, "otherSW": otherSW,
	})
}

func (h *Handler) PCEdit(c *gin.Context) {
	num, _ := strconv.Atoi(c.Param("pc_number"))

	var req EditPCRequest
	if err := c.ShouldBind(&req); err != nil {
		h.errHTML(c, "Data tidak valid")
		return
	}

	photoSerial, photoFront := processPhotoRefs(c)
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.pcService.UpdatePC(num, services.UpdatePCInput{
		Status: req.Status, DeviceType: req.DeviceType,
		SerialNumber: req.SerialNumber, BrandModel: req.BrandModel,
		Accessories: req.Accessories, Processor: req.Processor,
		RAM: req.RAM, Storage: req.Storage, OperatingSystem: req.OperatingSystem,
		Notes: req.Notes, ActionNotes: req.ActionNotes,
		PhotoSerial: photoSerial, PhotoFront: photoFront,
	}, uid, u, r, ip, ua); err != nil {
		h.errHTML(c, "Gagal mengupdate PC")
		return
	}

	h.pcRepo.SyncSoftware(num, c.PostFormArray("required_sw"), c.PostFormArray("other_name"), c.PostFormArray("other_desc"))
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
	pcs, err := h.pcRepo.List(repository.PCFilters{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Status diperlukan"})
		return
	}
	if err := h.pcService.UpdateStatus(id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) PCExport(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat export data"); return }

	pcs, _ := h.pcRepo.ExportAll()
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

func processPhotoRefs(c *gin.Context) (serial, front string) {
	for _, p := range []struct{ field string; result *string }{
		{"serial_file_ref", &serial}, {"front_file_ref", &front},
	} {
		ref := strings.TrimSpace(c.PostForm(p.field))
		if ref == "" { continue }
		src := filepath.Join("uploads", "temp", ref)
		dst := filepath.Join("uploads", "pc", ref)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil { continue }
		if err := services.CopyFile(src, dst); err != nil { continue }
		os.Remove(src)
		*p.result = ref
	}
	return
}
