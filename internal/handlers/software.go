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

	stats, err := h.softwareRepo.List(search, filterCategory)
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
	items, err := h.softwareRepo.GetOtherCatalog()
	if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data"}); return }
	c.JSON(http.StatusOK, items)
}

func (h *Handler) SoftwareEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat mengedit software"); return }

	id, _ := strconv.Atoi(c.Param("id"))
	sw, err := h.softwareRepo.GetByID(id)
	if err != nil {
		h.errHTML(c, "Software tidak ditemukan")
		return
	}

	pcList, err := h.softwareRepo.GetPCInstallStatus(id)
	if err != nil { h.errHTML(c, "Gagal mengambil data PC"); return }

	c.HTML(http.StatusOK, "software/edit.html", gin.H{
		"title": "Edit Software - " + sw.Name, "currentPage": "software",
		"username": username, "role": role,
		"software": sw, "pcList": pcList,
	})
}

func (h *Handler) SoftwareEdit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	pcIDs := c.PostFormArray("pc_ids[]")

	if err := h.softwareRepo.UpdateSoftwarePCs(id, pcIDs); err != nil {
		h.redirectWithError(c, "/software", "Gagal mengupdate software PC")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	h.activityLogService.LogUpdate(uid, u, r, "software", 0,
		map[string]interface{}{"software_id": id},
		map[string]interface{}{"pc_ids": pcIDs}, ip, ua)

	c.Redirect(http.StatusFound, "/software")
}

func (h *Handler) SoftwareDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	name, err := h.softwareRepo.GetName(id)
	if err != nil {
		h.redirectWithError(c, "/software", "Software tidak ditemukan")
		return
	}

	if err := h.softwareRepo.Delete(id); err != nil {
		h.redirectWithError(c, "/software", "Gagal menghapus software")
		return
	}

	h.logDelete(c, "software", 0, map[string]interface{}{"name": name})
	c.Redirect(http.StatusFound, "/software")
}

func (h *Handler) SoftwareCreate(c *gin.Context) {
	var req CreateSoftwareRequest
	if err := c.ShouldBind(&req); err != nil {
		h.redirectWithError(c, "/software", "Nama software harus diisi")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Category != "required" { req.Category = "other" }

	if _, err := h.softwareRepo.Create(req.Name, req.Category, req.Description); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			h.logCreateError(c, "software", map[string]interface{}{"name": req.Name, "category": req.Category}, "Duplicate: "+req.Name)
			h.redirectWithError(c, "/software", "Software dengan nama tersebut sudah ada")
			return
		}
		h.logCreateError(c, "software", map[string]interface{}{"name": req.Name, "category": req.Category}, err.Error())
		h.redirectWithError(c, "/software", "Gagal menyimpan software")
		return
	}

	h.logCreate(c, "software", 0, map[string]interface{}{"name": req.Name, "category": req.Category, "description": req.Description})
	c.Redirect(http.StatusFound, "/software")
}

func (h *Handler) SoftwareExport(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok { return }
	if role != "admin" { h.errHTML(c, "Hanya admin yang dapat export data software"); return }

	stats, err := h.softwareRepo.Export()
	if err != nil {
		h.errHTML(c, "Gagal mengambil data software")
		return
	}

	data := [][]interface{}{}
	for i, s := range stats {
		desc := s.Description
		if desc == "" { desc = "-" }
		cat := "Other"
		if s.Category == "required" { cat = "Required" }
		data = append(data, []interface{}{i + 1, s.Name, cat, desc, fmt.Sprintf("%d / %d PC", s.InstalledCount, s.TotalPCs)})
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
