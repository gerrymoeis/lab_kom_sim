package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"inventaris-lab-kom/internal/services"
	"inventaris-lab-kom/internal/timeutil"
)

// UploadResponse represents the response from image upload
type UploadResponse struct {
	Success    bool   `json:"success"`
	PreviewURL string `json:"preview_url,omitempty"`
	FileRef    string `json:"file_ref,omitempty"`
	Message    string `json:"message,omitempty"`
}

// CleanupRequest represents the request for temp file cleanup
type CleanupRequest struct {
	FileRef  string   `json:"file_ref,omitempty"`
	FileRefs []string `json:"file_refs,omitempty"`
}

// UploadImage handles image upload, validates, compresses to JPEG, and saves to temp/.
func (h *Handler) UploadImage(c *gin.Context) {
	if !h.requireAdmin(c) { return }

	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, UploadResponse{
			Success: false,
			Message: "File tidak ditemukan",
		})
		return
	}

	var req UploadImageRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, UploadResponse{
			Success: false,
			Message: "Parameter tidak valid",
		})
		return
	}

	// Validate file size (max 5MB)
	if file.Size > 5*1024*1024 {
		c.JSON(http.StatusBadRequest, UploadResponse{
			Success: false,
			Message: "File terlalu besar (max 5MB)",
		})
		return
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := []string{".jpg", ".jpeg", ".png", ".heic", ".heif"}
	isAllowed := false
	for _, allowed := range allowedExts {
		if ext == allowed {
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		c.JSON(http.StatusBadRequest, UploadResponse{
			Success: false,
			Message: "Format file tidak didukung. Gunakan JPEG, PNG, atau HEIC",
		})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, UploadResponse{
			Success: false,
			Message: "Gagal membaca file",
		})
		return
	}
	buf := make([]byte, 512)
	if _, err := f.Read(buf); err != nil && err != io.EOF {
		f.Close()
		c.JSON(http.StatusBadRequest, UploadResponse{
			Success: false,
			Message: "Gagal membaca file",
		})
		return
	}
	f.Close()
	mimeType := http.DetectContentType(buf)
	isImage := strings.HasPrefix(mimeType, "image/")
	// HEIC/HEIF is not detected by Go's built-in MIME sniffer; validate via extension
	if !isImage && ext != ".heic" && ext != ".heif" {
		c.JSON(http.StatusBadRequest, UploadResponse{
			Success: false,
			Message: "File harus berupa gambar",
		})
		return
	}

	now := timeutil.Now()
	dateStr := now.Format("020106") // DDMMYY

	if strings.ContainsAny(req.Label, "/\\") || strings.ContainsAny(req.Type, "/\\") {
		c.JSON(http.StatusBadRequest, UploadResponse{
			Success: false,
			Message: "Parameter tidak valid",
		})
		return
	}

	var fileBase string
	if req.Label != "" {
		label := strings.ToLower(req.Label)
		switch req.Type {
		case "serial":
			fileBase = fmt.Sprintf("%s_serial_%s", label, dateStr)
		case "front":
			fileBase = fmt.Sprintf("%s_front_%s", label, dateStr)
		case "device_type":
			fileBase = fmt.Sprintf("%s_%s", label, dateStr)
		case "installation":
			fileBase = fmt.Sprintf("instalasi_%s_%s", label, dateStr)
		case "logbook":
			fileBase = fmt.Sprintf("logbook_%s", dateStr)
		default:
			fileBase = fmt.Sprintf("temp_%s_%s", req.Type, dateStr)
		}
	} else {
		fileBase = fmt.Sprintf("temp_%s_%s", req.Type, dateStr)
	}
	finalFilename := fileBase + ".jpeg"
	lab := c.GetString("lab")
	finalPath := filepath.Join(h.cfg.UploadPath, lab, "temp", finalFilename)

	// Ensure temp directory exists
	tempDir := filepath.Join(h.cfg.UploadPath, lab, "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, UploadResponse{
			Success: false,
			Message: "Gagal membuat direktori temporary",
		})
		return
	}

	// Save original, then server-side compress + convert to JPEG
	tempOriginal := filepath.Join(h.cfg.UploadPath, lab, "temp", "original_"+fileBase+ext)
	if err := c.SaveUploadedFile(file, tempOriginal); err != nil {
		c.JSON(http.StatusInternalServerError, UploadResponse{
			Success: false,
			Message: "Gagal menyimpan file",
		})
		return
	}

	maxDimension := 1280
	switch req.Type {
	case "front":
		maxDimension = 1920
	case "device_type":
		maxDimension = 1024
	}

	if err := h.imageService.CompressAndSave(tempOriginal, finalPath, maxDimension); err != nil {
		os.Remove(tempOriginal)
		c.JSON(http.StatusInternalServerError, UploadResponse{
			Success: false,
			Message: "Gagal memproses gambar",
		})
		return
	}
	os.Remove(tempOriginal)

	// Return success response
	c.JSON(http.StatusOK, UploadResponse{
		Success:    true,
		PreviewURL: "/uploads/" + lab + "/temp/" + finalFilename,
		FileRef:    finalFilename,
		Message:    "File berhasil diproses",
	})
}

// DeleteTempFile handles single temp file deletion
func (h *Handler) DeleteTempFile(c *gin.Context) {
	if !h.requireAdmin(c) { return }

	var req CleanupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errJSON(c, http.StatusBadRequest, "Request tidak valid")
		return
	}

	lab := c.GetString("lab")
	if ref := filepath.Base(req.FileRef); ref != "" && ref != "." && ref != "/" && ref != "\\" {
		tempPath := filepath.Join(h.cfg.UploadPath, lab, "temp", ref)
		os.Remove(tempPath)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// CleanupTempFiles handles multiple temp files deletion
func (h *Handler) CleanupTempFiles(c *gin.Context) {
	if !h.requireAdmin(c) { return }

	var req CleanupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errJSON(c, http.StatusBadRequest, "Request tidak valid")
		return
	}

	lab := c.GetString("lab")
	// Cleanup multiple files
	for _, fileRef := range req.FileRefs {
		if ref := filepath.Base(fileRef); ref != "" && ref != "." && ref != "/" && ref != "\\" {
			tempPath := filepath.Join(h.cfg.UploadPath, lab, "temp", ref)
			os.Remove(tempPath)
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

type ClearPhotoRequest struct {
	EntityType string `json:"type"`       // "pc", "device_type", "device_installation"
	Identifier string `json:"identifier"` // label (PC), slug (device_type), or id (device_installation)
	PhotoField string `json:"photo"`      // "serial" or "front" (only for PC)
}

func (h *Handler) ClearPhoto(c *gin.Context) {
	if !h.requireAdmin(c) { return }

	var req ClearPhotoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errJSON(c, http.StatusBadRequest, "Request tidak valid")
		return
	}

	lab := c.GetString("lab")
	uid, u, r, ok := h.user(c)
	if !ok { return }
	ip, ua := getRequestContext(c)

	switch req.EntityType {
	case "pc":
		pc, err := h.pcService.GetByLabel(req.Identifier)
		if err != nil || pc == nil {
			h.errJSON(c, http.StatusNotFound, "PC tidak ditemukan")
			return
		}
		var filename string
		field := "photo_" + req.PhotoField
		switch req.PhotoField {
		case "serial":
			filename = pc.PhotoSerial
		case "front":
			filename = pc.PhotoFront
		default:
			h.errJSON(c, http.StatusBadRequest, "Field photo tidak valid")
			return
		}
		services.DeleteFile(h.cfg.UploadPath, lab, "pc", filename)
		if err := h.pcService.ClearPhoto(req.Identifier, field); err != nil {
			h.errJSON(c, http.StatusInternalServerError, "Gagal menghapus foto")
			return
		}

	case "device_type":
		dt, err := h.deviceTypeService.GetByLabelSlug(req.Identifier)
		if err != nil || dt == nil {
			h.errJSON(c, http.StatusNotFound, "Tipe perangkat tidak ditemukan")
			return
		}
		services.DeleteFile(h.cfg.UploadPath, lab, "device_types", dt.Photo)
		if err := h.deviceTypeService.ClearPhoto(dt.ID, uid, u, r, ip, ua); err != nil {
			h.errJSON(c, http.StatusInternalServerError, "Gagal menghapus foto")
			return
		}

	case "device_installation":
		id, err := strconv.Atoi(req.Identifier)
		if err != nil {
			h.errJSON(c, http.StatusBadRequest, "ID instalasi tidak valid")
			return
		}
		inst, err := h.deviceInstallationService.GetByID(id)
		if err != nil || inst == nil {
			h.errJSON(c, http.StatusNotFound, "Instalasi tidak ditemukan")
			return
		}
		services.DeleteFile(h.cfg.UploadPath, lab, "device_installations", inst.Photo)
		if err := h.deviceInstallationService.ClearPhoto(id, uid, u, r, ip, ua); err != nil {
			h.errJSON(c, http.StatusInternalServerError, "Gagal menghapus foto")
			return
		}

	default:
		h.errJSON(c, http.StatusBadRequest, "Tipe entity tidak dikenal")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Foto berhasil dihapus"})
}
