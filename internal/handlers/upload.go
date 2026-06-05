package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

// UploadImage handles immediate image upload and processing for preview.
// When ANDROID=true: client has already compressed the image, save directly.
// When ANDROID=false: save original, then server-side compress + convert to JPEG.
func (h *Handler) UploadImage(c *gin.Context) {
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
	var allowedExts []string
	if h.cfg.Android {
		allowedExts = []string{".jpg", ".jpeg"}
	} else {
		allowedExts = []string{".jpg", ".jpeg", ".png", ".heic", ".heif"}
	}
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

	now := time.Now()
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
		default:
			fileBase = fmt.Sprintf("temp_%s_%s", req.Type, dateStr)
		}
	} else {
		fileBase = fmt.Sprintf("temp_%s_%s", req.Type, dateStr)
	}
	finalFilename := fileBase + ".jpeg"
	finalPath := filepath.Join("uploads", "temp", finalFilename)

	// Ensure temp directory exists
	tempDir := filepath.Join("uploads", "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, UploadResponse{
			Success: false,
			Message: "Gagal membuat direktori temporary",
		})
		return
	}

	if h.cfg.Android {
		if err := c.SaveUploadedFile(file, finalPath); err != nil {
			c.JSON(http.StatusInternalServerError, UploadResponse{
				Success: false,
				Message: "Gagal menyimpan file",
			})
			return
		}
	} else {
		// ANDROID=false: save original, then server-side compress
		tempOriginal := filepath.Join("uploads", "temp", "original_"+fileBase+ext)
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
	}

	// Return success response
	c.JSON(http.StatusOK, UploadResponse{
		Success:    true,
		PreviewURL: "/uploads/temp/" + finalFilename,
		FileRef:    finalFilename,
		Message:    "File berhasil diproses",
	})
}

// DeleteTempFile handles single temp file deletion
func (h *Handler) DeleteTempFile(c *gin.Context) {
	var req CleanupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errJSON(c, http.StatusBadRequest, "Request tidak valid")
		return
	}

	if ref := filepath.Base(req.FileRef); ref != "" && ref != "." && ref != "/" && ref != "\\" {
		tempPath := filepath.Join("uploads", "temp", ref)
		os.Remove(tempPath)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// CleanupTempFiles handles multiple temp files deletion
func (h *Handler) CleanupTempFiles(c *gin.Context) {
	var req CleanupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errJSON(c, http.StatusBadRequest, "Request tidak valid")
		return
	}

	// Cleanup multiple files
	for _, fileRef := range req.FileRefs {
		if ref := filepath.Base(fileRef); ref != "" && ref != "." && ref != "/" && ref != "\\" {
			tempPath := filepath.Join("uploads", "temp", ref)
			os.Remove(tempPath)
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
