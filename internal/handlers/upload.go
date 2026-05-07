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

// UploadImage handles immediate image upload and processing for preview
func (h *Handler) UploadImage(c *gin.Context) {
	// Get form data
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, UploadResponse{
			Success: false,
			Message: "No file uploaded",
		})
		return
	}

	imageType := c.PostForm("type") // "serial" or "front"
	pcNumber := c.PostForm("pc_number")

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

	// Generate unique filename
	now := time.Now()
	var finalFilename string
	if pcNumber != "" {
		finalFilename = fmt.Sprintf("pc_%s_%s_%s.jpeg", pcNumber, imageType, now.Format("150405_02012006"))
	} else {
		finalFilename = fmt.Sprintf("temp_%s_%s.jpeg", imageType, now.Format("150405_02012006"))
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
	if imageType == "front" {
		maxDimension = 1920
	}

	if err := h.imageService.CompressAndSave(tempOriginal, finalPath, maxDimension); err != nil {
		os.Remove(tempOriginal)
		c.JSON(http.StatusInternalServerError, UploadResponse{
			Success: false,
			Message: "Gagal memproses gambar: " + err.Error(),
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