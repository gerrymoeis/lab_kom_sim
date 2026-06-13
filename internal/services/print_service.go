package services

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	gofpdf "github.com/lvillar/gofpdf"

	"inventaris-lab-kom/internal/repository"
)

type PrintConfig struct {
	Type           string  // "pc" atau "device"
	DeviceTypeSlug string  // slug device type (jika type="device")
	FontSizeCM     float64 // ukuran font dalam cm
	PaddingHCM     float64 // padding horizontal dalam cm
	PaddingVCM     float64 // padding vertical dalam cm
	PaperSize      string  // "A4", "F4", "A3"
	NumSheets      int     // jumlah copy (1..100)
	PDFTitle       string  // judul untuk metadata PDF
}

type PrintService struct {
	pcRepo     *repository.PCRepository
	deviceRepo *repository.DeviceRepository
}

func NewPrintService(pcRepo *repository.PCRepository, deviceRepo *repository.DeviceRepository) *PrintService {
	return &PrintService{pcRepo: pcRepo, deviceRepo: deviceRepo}
}

func (s *PrintService) GetLabels(cfg PrintConfig) ([]string, error) {
	switch cfg.Type {
	case "pc":
		pcs, err := s.pcRepo.List(repository.PCFilters{Placement: "dipakai"})
		if err != nil {
			return nil, fmt.Errorf("query pc: %w", err)
		}
		sort.Slice(pcs, func(i, j int) bool { return pcs[i].Label < pcs[j].Label })
		labels := make([]string, 0, len(pcs))
		for _, pc := range pcs {
			if pc.Label != "" {
				labels = append(labels, strings.ToUpper(pc.Label))
			}
		}
		return labels, nil

	case "device":
		filters := repository.DeviceFilters{DeviceTypeID: cfg.DeviceTypeSlug}
		devices, err := s.deviceRepo.List(filters)
		if err != nil {
			return nil, fmt.Errorf("query devices: %w", err)
		}
		sort.Slice(devices, func(i, j int) bool { return devices[i].AssetCode < devices[j].AssetCode })
		labels := make([]string, 0, len(devices))
		for _, d := range devices {
			if d.AssetCode != "" {
				labels = append(labels, d.AssetCode)
			}
		}
		return labels, nil

	default:
		return nil, fmt.Errorf("tipe tidak dikenal: %s", cfg.Type)
	}
}

var printPaperSizes = map[string][2]float64{
	"A4": {21.0, 29.7},
	"F4": {21.0, 33.0},
	"A3": {29.7, 42.0},
}

func (s *PrintService) GenerateStickerPDF(cfg PrintConfig) ([]byte, error) {
	labels, err := s.GetLabels(cfg)
	if err != nil {
		return nil, fmt.Errorf("get labels: %w", err)
	}
	if len(labels) == 0 {
		return nil, fmt.Errorf("tidak ada data untuk di-print")
	}

	fontPt := cfg.FontSizeCM * 28.35
	margin := 0.5
	gap := 0.3

	paperSize, ok := printPaperSizes[cfg.PaperSize]
	if !ok {
		paperSize = printPaperSizes["A4"]
	}
	printableW := paperSize[0] - 2*margin
	printableH := paperSize[1] - 2*margin
	if printableW <= 0 || printableH <= 0 {
		return nil, fmt.Errorf("ukuran kertas terlalu kecil")
	}

	pdf := gofpdf.New("P", "cm", "", "")
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetMargins(margin, margin, margin)

	if cfg.PDFTitle != "" {
		pdf.SetTitle(cfg.PDFTitle, true)
	}

	pdf.SetFont("Helvetica", "B", fontPt)
	pdf.SetCellMargin(0)

	maxTextW := 0.0
	for _, label := range labels {
		w := pdf.GetStringWidth(label)
		if w > maxTextW {
			maxTextW = w
		}
	}

	stickerW := maxTextW + 2*cfg.PaddingHCM
	stickerH := cfg.FontSizeCM + 2*cfg.PaddingVCM

	if stickerW > printableW || stickerH > printableH {
		return nil, fmt.Errorf("stiker (%.1f×%.1f cm) terlalu besar untuk kertas %s (%.1f×%.1f cm)",
			stickerW, stickerH, cfg.PaperSize, printableW, printableH)
	}

	cols := int(math.Floor((printableW + gap) / (stickerW + gap)))
	rows := int(math.Floor((printableH + gap) / (stickerH + gap)))
	if cols < 1 || rows < 1 {
		return nil, fmt.Errorf("stiker terlalu besar untuk kertas %s", cfg.PaperSize)
	}

	perPage := cols * rows
	pagesForAllData := int(math.Ceil(float64(len(labels)) / float64(perPage)))
	if pagesForAllData < 1 {
		pagesForAllData = 1
	}
	totalPages := pagesForAllData * cfg.NumSheets

	totalGridW := float64(cols)*stickerW + float64(cols-1)*gap
	totalGridH := float64(rows)*stickerH + float64(rows-1)*gap
	offsetX := (printableW - totalGridW) / 2
	offsetY := (printableH - totalGridH) / 2

	for page := 0; page < totalPages; page++ {
		pdf.AddPageFormat("", gofpdf.SizeType{Wd: paperSize[0], Ht: paperSize[1]})

		startIdx := (page % pagesForAllData) * perPage
		endIdx := startIdx + perPage
		if endIdx > len(labels) {
			endIdx = len(labels)
		}

		idx := startIdx
		for r := 0; r < rows && idx < endIdx; r++ {
			for c := 0; c < cols && idx < endIdx; c++ {
				label := labels[idx]
				idx++

				x := margin + offsetX + float64(c)*(stickerW+gap)
				y := margin + offsetY + float64(r)*(stickerH+gap)

				pdf.SetLineWidth(0.05)
				pdf.SetDrawColor(60, 60, 60)
				pdf.SetFillColor(255, 255, 255)
				pdf.Rect(x, y, stickerW, stickerH, "FD")

				labelW := pdf.GetStringWidth(label)
				textX := x + (stickerW-labelW)/2
				textY := y + cfg.PaddingVCM + 0.059*cfg.FontSizeCM
				pdf.SetXY(textX, textY)
				pdf.CellFormat(labelW, cfg.FontSizeCM, label, "", 0, "L", false, 0, "")
			}
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("output pdf: %w", err)
	}
	return buf.Bytes(), nil
}

func FormatPrintTimestamp() string {
	return time.Now().Format("20060102_150405")
}
