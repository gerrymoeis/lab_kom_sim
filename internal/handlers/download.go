package handlers

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"inventaris-lab-kom/internal/repository"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

type zipEntry struct {
	diskPath string
	zipPath  string
}

func (h *Handler) BatchDownloadZIP(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}

	lab := c.GetString("lab")
	entityType := c.Query("type")
	uploadPath := h.cfg.UploadPath

	var entries []zipEntry
	var prefix string

	switch entityType {
	case "pc":
		pcs, err := h.pcService.List(repository.PCFilters{})
		if err != nil {
			h.errJSON(c, http.StatusInternalServerError, "Gagal mengambil data PC")
			return
		}
		for _, pc := range pcs {
			if pc.PhotoSerial != "" {
				entries = append(entries, zipEntry{
					diskPath: filepath.Join(uploadPath, lab, "pc", pc.PhotoSerial),
					zipPath:  "pc_fotos/" + pc.PhotoSerial,
				})
			}
			if pc.PhotoFront != "" {
				entries = append(entries, zipEntry{
					diskPath: filepath.Join(uploadPath, lab, "pc", pc.PhotoFront),
					zipPath:  "pc_fotos/" + pc.PhotoFront,
				})
			}
		}
		prefix = lab + "_foto_pcs"

	case "devices":
		dts, err := h.deviceTypeService.List("", "")
		if err != nil {
			h.errJSON(c, http.StatusInternalServerError, "Gagal mengambil data perangkat")
			return
		}
		for _, dt := range dts {
			if dt.Photo != "" {
				entries = append(entries, zipEntry{
					diskPath: filepath.Join(uploadPath, lab, "device_types", dt.Photo),
					zipPath:  "device_fotos/" + dt.Photo,
				})
			}
		}
		insts, err := h.deviceInstallationService.ExportAll()
		if err == nil {
			for _, inst := range insts {
				if inst.Photo != "" {
					entries = append(entries, zipEntry{
						diskPath: filepath.Join(uploadPath, lab, "device_installations", inst.Photo),
						zipPath:  "device_fotos/" + inst.Photo,
					})
				}
			}
		}
		prefix = lab + "_foto_devices"

	default:
		h.errJSON(c, http.StatusBadRequest, "Tipe tidak valid: gunakan 'pc' atau 'devices'")
		return
	}

	filename := services.GenerateZIPFilename(prefix)
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Cache-Control", "no-cache")

	zw := zip.NewWriter(c.Writer)
	defer zw.Close()

	buf := make([]byte, 32*1024)
	for _, e := range entries {
		fh, err := os.Open(e.diskPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("[WARN] batch download: file not found, skipping: %s", e.diskPath)
				continue
			}
			log.Printf("[WARN] batch download: error reading %s: %v", e.diskPath, err)
			continue
		}
		w, err := zw.Create(e.zipPath)
		if err != nil {
			fh.Close()
			log.Printf("[WARN] batch download: zip create %s: %v", e.zipPath, err)
			continue
		}
		io.CopyBuffer(w, fh, buf)
		fh.Close()
	}
}
