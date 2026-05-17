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

// UploadImage handles immediate image upload and processing for preview
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
	c.ShouldBind(&req)

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

	now := time.Now()
	var finalFilename string
	if req.PCNumber != "" {
		finalFilename = fmt.Sprintf("pc_%s_%s_%s.jpeg", req.PCNumber, req.Type, now.Format("150405_02012006"))
	} else {
		finalFilename = fmt.Sprintf("temp_%s_%s.jpeg", req.Type, now.Format("150405_02012006"))
	}

	// Paths
	tempOriginal := filepath.Join("uploads", "temp", "original_"+finalFilename+ext)
	finalPath := filepath.Join("uploads", "temp", finalFilename) // Temp location first

	// Ensure temp directory exists
	tempDir := filepath.Join("uploads", "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, UploadResponse{
			Success: false,
			Message: "Gagal membuat direktori temporary",
		})
		return
	}

	// Save original file temporarily
	if err := c.SaveUploadedFile(file, tempOriginal); err != nil {
		c.JSON(http.StatusInternalServerError, UploadResponse{
			Success: false,
			Message: "Gagal menyimpan file",
		})
		return
	}

	// Compress and convert using existing ImageService
	maxDimension := 1280
	if req.Type == "front" {
		maxDimension = 1920
	}

	if err := h.imageService.CompressAndSave(tempOriginal, finalPath, maxDimension); err != nil {
		os.Remove(tempOriginal)
		c.JSON(http.StatusInternalServerError, UploadResponse{
			Success: false,
			Message: "Gagal memproses gambar",
		})
		return
	}

	// Cleanup original temp file
	os.Remove(tempOriginal)

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

	if req.FileRef != "" {
		tempPath := filepath.Join("uploads", "temp", req.FileRef)
		os.Remove(tempPath) // Silent removal, no error if file doesn't exist
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
		if fileRef != "" {
			tempPath := filepath.Join("uploads", "temp", fileRef)
			os.Remove(tempPath) // Silent removal, no error if file doesn't exist
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}