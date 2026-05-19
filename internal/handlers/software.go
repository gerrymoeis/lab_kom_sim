package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

func (h *Handler) SoftwareList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	search := c.Query("search")
	filterCategory := c.Query("category")

	stats, err := h.softwareService.List(search, filterCategory)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data software")
		return
	}

	c.HTML(http.StatusOK, "software/list.html", gin.H{
		"title": "Software Catalog", "currentPage": "software",
		"username": username, "role": role,
		"catalog": stats, "search": search, "filterCat": filterCategory,
		"error": c.Query("error"),
	})
}

func (h *Handler) GetSoftwareCatalogJSON(c *gin.Context) {
	items, err := h.softwareService.GetOtherCatalog()
	if err != nil { h.errJSON(c, http.StatusInternalServerError, "Gagal mengambil data"); return }
	c.JSON(http.StatusOK, items)
}

func (h *Handler) SoftwareDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id, _ := strconv.Atoi(c.Param("id"))
	sw, err := h.softwareService.GetByID(id)
	if err != nil {
		h.errHTML(c, "Software tidak ditemukan")
		return
	}

	pcList, err := h.softwareService.GetPCInstallStatus(id)
	if err != nil { h.errHTML(c, "Gagal mengambil data PC"); return }

	installedCount := 0
	for _, p := range pcList {
		if p.Installed { installedCount++ }
	}

	c.HTML(http.StatusOK, "software/detail.html", gin.H{
		"title": "Detail Software - " + sw.Name, "currentPage": "software",
		"username": username, "role": role,
		"software": sw, "pcList": pcList,
		"installedCount": installedCount,
		"totalPCs": len(pcList),
	})
}

func (h *Handler) SoftwareEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat mengedit software"); return }

	id, _ := strconv.Atoi(c.Param("id"))
	sw, err := h.softwareService.GetByID(id)
	if err != nil {
		h.errHTML(c, "Software tidak ditemukan")
		return
	}

	pcList, err := h.softwareService.GetPCInstallStatus(id)
	if err != nil { h.errHTML(c, "Gagal mengambil data PC"); return }

	installedCount := 0
	for _, p := range pcList {
		if p.Installed { installedCount++ }
	}

	c.HTML(http.StatusOK, "software/edit.html", gin.H{
		"title": "Edit Software - " + sw.Name, "currentPage": "software",
		"username": username, "role": role,
		"software": sw, "pcList": pcList,
		"installedCount": installedCount,
		"totalPCs": len(pcList),
	})
}

func (h *Handler) SoftwareEdit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	sw, err := h.softwareService.GetByID(id)
	if err != nil {
		h.errHTML(c, "Software tidak ditemukan")
		return
	}

	var req struct {
		Name        string   `form:"name"`
		Category    string   `form:"category"`
		Description string   `form:"description"`
		PCIDs       []string `form:"pc_ids[]"`
	}
	c.ShouldBind(&req)

	if req.Name == "" {
		req.Name = sw.Name
	}
	if req.Category == "" {
		req.Category = sw.Category
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	if err := h.softwareService.Update(id, req.Name, req.Category, req.Description, req.PCIDs, uid, u, r, ip, ua); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			h.redirectWithError(c, "/software/"+c.Param("id")+"/edit", "Nama software sudah ada")
			return
		}
		h.redirectWithError(c, "/software/"+c.Param("id")+"/edit", "Gagal mengupdate software")
		return
	}

	c.Redirect(http.StatusFound, "/software")
}

func (h *Handler) SoftwareDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	if err := h.softwareService.Delete(id, uid, u, r, ip, ua); err != nil {
		h.redirectWithError(c, "/software", "Gagal menghapus software")
		return
	}
	c.Redirect(http.StatusFound, "/software")
}

func (h *Handler) SoftwareCreate(c *gin.Context) {
	var req CreateSoftwareRequest
	if err := c.ShouldBind(&req); err != nil {
		h.redirectWithError(c, "/software", "Nama software harus diisi")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	err := h.softwareService.Create(services.SoftwareCreateInput{
		Name: req.Name, Category: req.Category, Description: req.Description,
	}, uid, u, r, ip, ua)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			h.redirectWithError(c, "/software", "Software dengan nama tersebut sudah ada")
			return
		}
		h.redirectWithError(c, "/software", "Gagal menyimpan software")
		return
	}
	c.Redirect(http.StatusFound, "/software")
}

func (h *Handler) SoftwareExport(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat export data software"); return }

	stats, err := h.softwareService.Export()
	if err != nil {
		h.errHTML(c, "Gagal mengambil data software")
		return
	}

	data := [][]any{}
	for i, s := range stats {
		desc := s.Description
		if desc == "" { desc = "-" }
		cat := "Other"
		if s.Category == "required" { cat = "Required" }
		data = append(data, []any{i + 1, s.Name, cat, desc, fmt.Sprintf("%d / %d PC", s.InstalledCount, s.TotalPCs)})
	}

	svc := services.NewExcelService()
	f, _ := svc.GenerateExcel(services.ExcelExportConfig{
		SheetName: "Software Catalog",
		Headers:   []string{"No", "Nama Software", "Kategori", "Deskripsi", "PC Terinstall"},
		Data:      data,
		ColumnWidths: map[string]float64{"A": 5, "B": 30, "C": 12, "D": 40, "E": 15},
	})
	defer f.Close()

	filename := svc.GenerateFilename("software_catalog_export")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	f.Write(c.Writer)
}
