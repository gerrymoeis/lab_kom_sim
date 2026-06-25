package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

type printDeviceTypeItem struct {
	Prefix string
	Name   string
}

type printCategoryGroup struct {
	Name        string
	DeviceTypes []printDeviceTypeItem
}

type printUsageTypeGroup struct {
	UsageType  string
	Label      string
	Categories []printCategoryGroup
}

type templateForm struct {
	Name       string  `json:"name" binding:"required,max=100"`
	StickerType string `json:"sticker_type" binding:"required,oneof=pc device"`
	FontSizeCM  float64 `json:"font_size_cm" binding:"required,min=0.3,max=5.0"`
	PaddingHCM float64 `json:"padding_h_cm" binding:"required,min=0.1,max=5.0"`
	PaddingVCM float64 `json:"padding_v_cm" binding:"required,min=0.1,max=5.0"`
}

func (h *Handler) PrintForm(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	deviceTypes, err := h.deviceTypeService.List("", "")
	if err != nil {
		h.errHTML(c, "Gagal memuat device types: "+err.Error())
		return
	}

	tipe := c.DefaultQuery("type", "pc")

	var defaultFontSize, defaultPadH, defaultPadV float64
	if tipe == "device" {
		defaultFontSize = 0.5
		defaultPadH = 0.3
		defaultPadV = 0.3
	} else {
		defaultFontSize = 1.0
		defaultPadH = 0.5
		defaultPadV = 0.8
	}

	pcMahasiswaLabels, pcSpesialLabels, longestPCLabel, _ := h.printService.GetPCLabelGroups()

	longestDeviceLabel := ""
	if devLabels, err := h.printService.GetLabels(services.PrintConfig{Type: "device"}); err == nil {
		for _, l := range devLabels {
			if len(l) > len(longestDeviceLabel) {
				longestDeviceLabel = l
			}
		}
	}

	longestPerPrefix := make(map[string]string)
	for _, dt := range deviceTypes {
		labels, err := h.printService.GetLabels(services.PrintConfig{
			Type:           "device",
			DeviceTypeSlug: fmt.Sprintf("%d", dt.ID),
		})
		if err != nil {
			continue
		}
		longest := ""
		for _, l := range labels {
			if len(l) > len(longest) {
				longest = l
			}
		}
		if longest != "" {
			longestPerPrefix[dt.LabelPrefix] = longest
		}
	}

	usageTypeLabels := map[string]string{
		"loanable":    "Dipinjamkan",
		"consumable":  "Habis Pakai",
		"installable": "Instalasi",
	}
	groupMap := make(map[string]map[string][]models.DeviceType)
	for _, dt := range deviceTypes {
		if groupMap[dt.UsageType] == nil {
			groupMap[dt.UsageType] = make(map[string][]models.DeviceType)
		}
		groupMap[dt.UsageType][dt.CategoryName] = append(groupMap[dt.UsageType][dt.CategoryName], dt)
	}
	usageOrder := []string{"loanable", "consumable", "installable"}
	var groupedTypes []printUsageTypeGroup
	for _, ut := range usageOrder {
		cats, ok := groupMap[ut]
		if !ok {
			continue
		}
		g := printUsageTypeGroup{
			UsageType: ut,
			Label:     usageTypeLabels[ut],
		}
		catNames := make([]string, 0, len(cats))
		for cn := range cats {
			catNames = append(catNames, cn)
		}
		sort.Strings(catNames)
		for _, cn := range catNames {
			c := printCategoryGroup{Name: cn}
			for _, dt := range cats[cn] {
				c.DeviceTypes = append(c.DeviceTypes, printDeviceTypeItem{
					Prefix: dt.LabelPrefix,
					Name:   dt.Name,
				})
			}
			g.Categories = append(g.Categories, c)
		}
		groupedTypes = append(groupedTypes, g)
	}

	h.renderTemplate(c, http.StatusOK, "print/form.html", gin.H{
		"title":                "Print Stiker Label",
		"currentPage":          "print",
		"username":             username,
		"role":                 role,
		"deviceTypes":          deviceTypes,
		"selectedType":         tipe,
		"defaultFontSize":      defaultFontSize,
		"defaultPaddingH":      defaultPadH,
		"defaultPaddingV":      defaultPadV,
		"pcMahasiswaLabels":    pcMahasiswaLabels,
		"pcSpesialLabels":      pcSpesialLabels,
		"longestPCLabel":       longestPCLabel,
		"longestDeviceLabel":   longestDeviceLabel,
		"longestPerDeviceType": longestPerPrefix,
		"groupedDeviceTypes":   groupedTypes,
	})
}

func (h *Handler) PrintGeneratePDF(c *gin.Context) {
	_, _, _, ok := h.user(c)
	if !ok {
		return
	}

	cfg := services.PrintConfig{
		Type:           c.DefaultQuery("type", "pc"),
		DeviceTypeSlug: c.Query("device_type"),
		PaperSize:      c.DefaultQuery("paper_size", "A4"),
	}

	rawDeviceTypeFilter := c.Query("device_type")

	if cfg.Type == "device" && rawDeviceTypeFilter != "" {
		prefixes := strings.Split(rawDeviceTypeFilter, ",")
		var ids []string
		for _, p := range prefixes {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			dt, err := h.deviceTypeService.GetByLabelSlug(p)
			if err != nil {
				h.errHTML(c, "Device type tidak ditemukan: "+p)
				return
			}
			ids = append(ids, fmt.Sprintf("%d", dt.ID))
		}
		if len(ids) == 0 {
			h.errHTML(c, "Device type tidak valid")
			return
		}
		cfg.DeviceTypeSlug = strings.Join(ids, ",")
	}

	if cfg.Type == "pc" {
		rawPCLabels := c.Query("pc_labels")
		if rawPCLabels != "" {
			for _, l := range strings.Split(rawPCLabels, ",") {
				l = strings.TrimSpace(l)
				if l != "" {
					cfg.PCLabels = append(cfg.PCLabels, l)
				}
			}
		}
	}

	fontSize, err := strconv.ParseFloat(c.DefaultQuery("font_size", "0.5"), 64)
	if err != nil || fontSize < 0.3 || fontSize > 5.0 {
		h.errHTML(c, "Font size harus antara 0.3 - 5.0 cm")
		return
	}
	cfg.FontSizeCM = fontSize

	padH, err := strconv.ParseFloat(c.DefaultQuery("padding_h", "0.3"), 64)
	if err != nil || padH < 0.1 || padH > 5.0 {
		h.errHTML(c, "Padding horizontal harus antara 0.1 - 5.0 cm")
		return
	}
	cfg.PaddingHCM = padH

	padV, err := strconv.ParseFloat(c.DefaultQuery("padding_v", "0.3"), 64)
	if err != nil || padV < 0.1 || padV > 5.0 {
		h.errHTML(c, "Padding vertical harus antara 0.1 - 5.0 cm")
		return
	}
	cfg.PaddingVCM = padV

	numSheets, err := strconv.Atoi(c.DefaultQuery("num_sheets", "1"))
	if err != nil || numSheets < 1 || numSheets > 100 {
		h.errHTML(c, "Jumlah copy harus antara 1 - 100")
		return
	}
	cfg.NumSheets = numSheets

	validPaper := map[string]bool{"A4": true, "F4": true, "A3": true}
	if !validPaper[cfg.PaperSize] {
		h.errHTML(c, "Ukuran kertas tidak valid")
		return
	}

	validType := map[string]bool{"pc": true, "device": true}
	if !validType[cfg.Type] {
		h.errHTML(c, "Tipe tidak valid")
		return
	}

	var labelName, titleName string
	switch cfg.Type {
	case "pc":
		if len(cfg.PCLabels) > 0 {
			if len(cfg.PCLabels) >= 5 {
				labelName = fmt.Sprintf("pc_%d", len(cfg.PCLabels))
			} else if len(cfg.PCLabels) >= 2 {
				labelName = "pc_" + strings.Join(cfg.PCLabels, "+")
			} else {
				labelName = "pc_" + cfg.PCLabels[0]
			}
		} else {
			labelName = "pc"
		}
		titleName = "PC"
	case "device":
		if rawDeviceTypeFilter != "" {
			prefixes := strings.Split(rawDeviceTypeFilter, ",")
			cleaned := make([]string, 0, len(prefixes))
			for _, p := range prefixes {
				p = strings.TrimSpace(p)
				if p != "" {
					cleaned = append(cleaned, p)
				}
			}
			if len(cleaned) >= 5 {
				labelName = fmt.Sprintf("device_%d", len(cleaned))
			} else if len(cleaned) >= 2 {
				labelName = "device_" + strings.Join(cleaned, "+")
			} else {
				labelName = "device_" + cleaned[0]
			}
		} else {
			labelName = "device"
		}
		titleName = "Device"
	}
	cfg.PDFTitle = "Stiker " + titleName

	pdfBytes, err := h.printService.GenerateStickerPDF(cfg)
	if err != nil {
		h.errHTML(c, "Gagal generate PDF: "+err.Error())
		return
	}

	filename := fmt.Sprintf("stiker_%s_%s.pdf", labelName, services.FormatPrintTimestamp())
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filename))
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

func (h *Handler) StickerTemplateList(c *gin.Context) {
	_, _, _, ok := h.user(c)
	if !ok {
		return
	}
	stickerType := c.DefaultQuery("type", "pc")
	if stickerType != "pc" && stickerType != "device" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type harus 'pc' atau 'device'"})
		return
	}
	templates, err := h.printService.ListTemplates(stickerType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat template: " + err.Error()})
		return
	}
	if templates == nil {
		templates = []models.StickerTemplate{}
	}
	c.Header("Cache-Control", "no-cache")
	c.JSON(http.StatusOK, templates)
}

func (h *Handler) StickerTemplateCreate(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok {
		return
	}
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Hanya admin yang dapat menyimpan template"})
		return
	}
	var form templateForm
	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data tidak valid: " + err.Error()})
		return
	}
	if err := h.printService.SaveTemplate(form.Name, form.StickerType, form.FontSizeCM, form.PaddingHCM, form.PaddingVCM); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan template: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "message": "Template berhasil disimpan"})
}

func (h *Handler) StickerTemplateUpdate(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok {
		return
	}
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Hanya admin yang dapat mengubah template"})
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID tidak valid"})
		return
	}
	var form templateForm
	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data tidak valid: " + err.Error()})
		return
	}
	if err := h.printService.UpdateTemplate(id, form.Name, form.FontSizeCM, form.PaddingHCM, form.PaddingVCM); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengupdate template: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Template berhasil diupdate"})
}

func (h *Handler) StickerTemplateDelete(c *gin.Context) {
	_, _, role, ok := h.user(c)
	if !ok {
		return
	}
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Hanya admin yang dapat menghapus template"})
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID tidak valid"})
		return
	}
	if err := h.printService.DeleteTemplate(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus template: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Template berhasil dihapus"})
}
