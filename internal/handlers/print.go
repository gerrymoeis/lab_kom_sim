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
	defaultFontSize := 2.0
	defaultPadH := 2.0
	defaultPadV := 1.0
	if tipe == "device" {
		defaultFontSize = 1.5
		defaultPadH = 1.5
		defaultPadV = 0.8
	}

	h.renderTemplate(c, http.StatusOK, "print/form.html", gin.H{
		"title":            "Print Stiker Label",
		"currentPage":      "print",
		"username":         username,
		"role":             role,
		"deviceTypes":      deviceTypes,
		"selectedType":     tipe,
		"defaultFontSize":  defaultFontSize,
		"defaultPaddingH":  defaultPadH,
		"defaultPaddingV":  defaultPadV,
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

	if cfg.Type == "device" && cfg.DeviceTypeSlug != "" {
		dt, err := h.deviceTypeService.GetBySlug(cfg.DeviceTypeSlug)
		if err != nil {
			h.errHTML(c, "Device type tidak ditemukan")
			return
		}
		cfg.DeviceTypeSlug = fmt.Sprintf("%d", dt.ID)
	}

	fontSize, err := strconv.ParseFloat(c.DefaultQuery("font_size", "2.0"), 64)
	if err != nil || fontSize < 0.5 || fontSize > 5.0 {
		h.errHTML(c, "Font size harus antara 0.5 - 5.0 cm")
		return
	}
	cfg.FontSizeCM = fontSize

	padH, err := strconv.ParseFloat(c.DefaultQuery("padding_h", "1.5"), 64)
	if err != nil || padH < 0.3 || padH > 5.0 {
		h.errHTML(c, "Padding horizontal harus antara 0.3 - 5.0 cm")
		return
	}
	cfg.PaddingHCM = padH

	padV, err := strconv.ParseFloat(c.DefaultQuery("padding_v", "1.0"), 64)
	if err != nil || padV < 0.3 || padV > 5.0 {
		h.errHTML(c, "Padding vertical harus antara 0.3 - 5.0 cm")
		return
	}
	cfg.PaddingVCM = padV

	numSheets, err := strconv.Atoi(c.DefaultQuery("num_sheets", "1"))
	if err != nil || numSheets < 1 || numSheets > 100 {
		h.errHTML(c, "Jumlah lembar harus antara 1 - 100")
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

	pdfBytes, err := h.printService.GenerateStickerPDF(cfg)
	if err != nil {
		h.errHTML(c, "Gagal generate PDF: "+err.Error())
		return
	}

	filename := fmt.Sprintf("stiker-%s-%s.pdf", cfg.Type, services.FormatPrintTimestamp())
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filename))
	// Hapus cache agar selalu fresh
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}
