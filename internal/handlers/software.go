package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"inventaris-lab-kom/internal/repository"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

func (h *Handler) SoftwareList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 { page = 1 }
	pageSize := h.cfg.DefaultPageSize

	values, _ := url.ParseQuery(c.Request.URL.RawQuery)
	delete(values, "page")
	values.Del("success")
	values.Del("error")
	values.Del("toast")
	var query interface{} = ""
	if len(values) > 0 { query = template.URL("&" + values.Encode()) }

	search := c.Query("search")
	filterCategory := c.Query("category")
	sortBy := c.Query("sort_by")

	stats, total, err := h.softwareService.ListPaginated(search, filterCategory, sortBy, page, pageSize)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data software")
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	startRow := (page-1)*pageSize + 1

	h.renderTemplate(c, http.StatusOK, "software/list.html", gin.H{
		"title": "Software Catalog", "currentPage": "software",
		"username": username, "role": role,
		"catalog": stats, "filters": gin.H{"search": search, "category": filterCategory, "sort_by": sortBy},
		"page": page, "startRow": startRow, "totalPages": totalPages, "totalItems": total,
		"query": query,
	})
}

func (h *Handler) GetSoftwareCatalogJSON(c *gin.Context) {
	items, err := h.softwareService.GetOtherCatalog()
	if err != nil { h.errJSON(c, http.StatusInternalServerError, "Gagal mengambil data"); return }
	c.JSON(http.StatusOK, items)
}

func buildSoftwareGrid(pcList []repository.PCInstallStatus) [][]repository.PCInstallStatus {
	maxRow, maxCol := 0, 0
	for _, p := range pcList {
		if p.Row > maxRow {
			maxRow = p.Row
		}
		if p.Column > maxCol {
			maxCol = p.Column
		}
	}
	if maxRow < 1 || maxCol < 1 {
		return nil
	}
	grid := make([][]repository.PCInstallStatus, maxRow)
	for i := range grid {
		grid[i] = make([]repository.PCInstallStatus, maxCol)
	}
	for _, p := range pcList {
		if p.Row >= 1 && p.Row <= maxRow && p.Column >= 1 && p.Column <= maxCol {
			grid[p.Row-1][p.Column-1] = p
		}
	}
	return grid
}

func (h *Handler) SoftwareDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	slug := c.Param("slug")
	sw, err := h.softwareService.GetBySlug(slug)
	if err != nil {
		h.errHTML(c, "Software tidak ditemukan")
		return
	}

	pcList, err := h.softwareService.GetPCInstallStatus(sw.ID)
	if err != nil { h.errHTML(c, "Gagal mengambil data PC"); return }

	installedCount := 0
	for _, p := range pcList {
		if p.Installed { installedCount++ }
	}

	h.renderTemplate(c, http.StatusOK, "software/detail.html", gin.H{
		"title": "Detail Software - " + sw.Name, "currentPage": "software",
		"username": username, "role": role,
		"software": sw, "pcGrid": buildSoftwareGrid(pcList),
		"installedCount": installedCount,
		"totalPCs": len(pcList),
	})
}

func (h *Handler) SoftwareEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat mengedit software"); return }

	slug := c.Param("slug")
	sw, err := h.softwareService.GetBySlug(slug)
	if err != nil {
		h.errHTML(c, "Software tidak ditemukan")
		return
	}

	pcList, err := h.softwareService.GetPCInstallStatus(sw.ID)
	if err != nil { h.errHTML(c, "Gagal mengambil data PC"); return }

	installedCount := 0
	for _, p := range pcList {
		if p.Installed { installedCount++ }
	}

	h.renderTemplate(c, http.StatusOK, "software/edit.html", gin.H{
		"title": "Edit Software - " + sw.Name, "currentPage": "software",
		"username": username, "role": role,
		"software": sw, "pcGrid": buildSoftwareGrid(pcList),
		"installedCount": installedCount,
		"totalPCs": len(pcList),
	})
}

func (h *Handler) SoftwareEdit(c *gin.Context) {
	slug := c.Param("slug")
	sw, err := h.softwareService.GetBySlug(slug)
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
	if err := c.ShouldBind(&req); err != nil {
		h.redirectWithError(c, "/software/"+slug+"/edit", "Data tidak valid")
		return
	}

	if req.Name == "" {
		req.Name = sw.Name
	}
	if req.Category == "" {
		req.Category = sw.Category
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	if err := h.softwareService.Update(sw.ID, req.Name, req.Category, req.Description, req.PCIDs, uid, u, r, ip, ua); err != nil {
		h.redirectWithError(c, "/software/"+slug+"/edit", err.Error())
		return
	}

	h.redirectWithSuccess(c, "/software", "Software berhasil diperbarui", "update")
}

func (h *Handler) SoftwareDelete(c *gin.Context) {
	slug := c.Param("slug")
	sw, err := h.softwareService.GetBySlug(slug)
	if err != nil {
		h.redirectWithError(c, "/software", "Software tidak ditemukan")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	if err := h.softwareService.Delete(sw.ID, uid, u, r, ip, ua); err != nil {
		h.redirectWithError(c, "/software", err.Error())
		return
	}
	h.redirectWithSuccess(c, "/software", "Software berhasil dihapus", "delete")
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
	h.redirectWithSuccess(c, "/software", "Software berhasil ditambahkan")
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
	f, err := svc.GenerateExcel(services.ExcelExportConfig{
		SheetName: "Software Catalog",
		Headers:   []string{"No", "Nama Software", "Kategori", "Deskripsi", "PC Terinstall"},
		Data:      data,
		ColumnWidths: map[string]float64{"A": 5, "B": 30, "C": 12, "D": 40, "E": 15},
	})
	if err != nil {
		h.errHTML(c, "Gagal membuat file excel")
		return
	}
	defer f.Close()

	filename := svc.GenerateFilename("software_catalog_export")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	if err := f.Write(c.Writer); err != nil {
		c.Error(err)
	}
}

func (h *Handler) SoftwareBatchDelete(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		h.errJSON(c, http.StatusBadRequest, "Tidak ada item yang dipilih")
		return
	}
	intIDs, err := parseInt64IDs(req.IDs)
	if err != nil {
		h.errJSON(c, http.StatusBadRequest, err.Error())
		return
	}
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	if err := h.softwareService.BatchDelete(intIDs, uid, u, r, ip, ua); err != nil {
		h.errJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Software berhasil dihapus"})
}
