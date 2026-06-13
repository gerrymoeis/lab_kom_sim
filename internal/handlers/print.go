package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

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
		defaultPadV = 0.5
	}

	longestPCLabel := ""
	if pcLabels, err := h.printService.GetLabels(services.PrintConfig{Type: "pc"}); err == nil {
		for _, l := range pcLabels {
			if len(l) > len(longestPCLabel) {
				longestPCLabel = l
			}
		}
	}

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
			longestPerPrefix[dt.AssetCodePrefix] = longest
		}
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
		"longestPCLabel":       longestPCLabel,
		"longestDeviceLabel":   longestDeviceLabel,
		"longestPerDeviceType": longestPerPrefix,
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

	if cfg.Type == "device" && cfg.DeviceTypeSlug != "" {
		dt, err := h.deviceTypeService.GetByPrefixSlug(cfg.DeviceTypeSlug)
		if err != nil {
			h.errHTML(c, "Device type tidak ditemukan")
			return
		}
		cfg.DeviceTypeSlug = fmt.Sprintf("%d", dt.ID)
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
		labelName = "pc_label"
		titleName = "PC Label"
	case "device":
		if rawDeviceTypeFilter != "" {
			labelName = "device_" + rawDeviceTypeFilter
			titleName = "Device " + rawDeviceTypeFilter
		} else {
			labelName = "device_asset_code"
			titleName = "Device Asset Code"
		}
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
	// Hapus cache agar selalu fresh
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}
