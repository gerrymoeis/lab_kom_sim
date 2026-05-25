package handlers

import (
	"fmt"
	"log"
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
	log.Printf("[DEBUG-UPLOAD] ===== UploadImage CALLED =====")
	log.Printf("[DEBUG-UPLOAD] Content-Type: %s", c.Request.Header.Get("Content-Type"))
	log.Printf("[DEBUG-UPLOAD] Content-Length: %d", c.Request.ContentLength)

	file, err := c.FormFile("image")
	if err != nil {
		log.Printf("[DEBUG-UPLOAD] ERROR: c.FormFile(\"image\") failed: %v", err)
		c.JSON(http.StatusBadRequest, UploadResponse{
			Success: false,
			Message: "File tidak ditemukan",
		})
		return
	}
	log.Printf("[DEBUG-UPLOAD] FormFile received: name=%s size=%d", file.Filename, file.Size)

	var req UploadImageRequest
	c.ShouldBind(&req)
	log.Printf("[DEBUG-UPLOAD] UploadImageRequest: type=%q pc_number=%q", req.Type, req.PCNumber)

	log.Printf("[DEBUG-UPLOAD] ANDROID mode: %v", h.cfg.Android)

	// Validate file size (max 5MB)
	if file.Size > 5*1024*1024 {
		log.Printf("[DEBUG-UPLOAD] ERROR: file too large: %d", file.Size)
		c.JSON(http.StatusBadRequest, UploadResponse{
			Success: false,
			Message: "File terlalu besar (max 5MB)",
		})
		return
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	log.Printf("[DEBUG-UPLOAD] File extension: %q", ext)
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
		log.Printf("[DEBUG-UPLOAD] ERROR: extension %q not allowed", ext)
		c.JSON(http.StatusBadRequest, UploadResponse{
			Success: false,
			Message: "Format file tidak didukung. Gunakan JPEG, PNG, atau HEIC",
		})
		return
	}

	now := time.Now()
	var fileBase string
	if req.PCNumber != "" {
		fileBase = fmt.Sprintf("pc_%s_%s_%s", req.PCNumber, req.Type, now.Format("150405_02012006"))
	} else {
		fileBase = fmt.Sprintf("temp_%s_%s", req.Type, now.Format("150405_02012006"))
	}
	finalFilename := fileBase + ".jpeg"
	finalPath := filepath.Join("uploads", "temp", finalFilename)
	log.Printf("[DEBUG-UPLOAD] fileBase=%q finalFilename=%q finalPath=%q", fileBase, finalFilename, finalPath)

	// Ensure temp directory exists
	tempDir := filepath.Join("uploads", "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Printf("[DEBUG-UPLOAD] ERROR creating temp dir: %v", err)
		c.JSON(http.StatusInternalServerError, UploadResponse{
			Success: false,
			Message: "Gagal membuat direktori temporary",
		})
		return
	}

	if h.cfg.Android {
		log.Printf("[DEBUG-UPLOAD] ANDROID=true: saving file directly to %s", finalPath)
		if err := c.SaveUploadedFile(file, finalPath); err != nil {
			log.Printf("[DEBUG-UPLOAD] ERROR saving file: %v", err)
			c.JSON(http.StatusInternalServerError, UploadResponse{
				Success: false,
				Message: "Gagal menyimpan file",
			})
			return
		}
		log.Printf("[DEBUG-UPLOAD] ANDROID=true: file saved successfully")
	} else {
		log.Printf("[DEBUG-UPLOAD] ANDROID=false: saving original to temp, then compressing")
		// ANDROID=false: save original, then server-side compress
		tempOriginal := filepath.Join("uploads", "temp", "original_"+fileBase+ext)
		if err := c.SaveUploadedFile(file, tempOriginal); err != nil {
			log.Printf("[DEBUG-UPLOAD] ERROR saving original file: %v", err)
			c.JSON(http.StatusInternalServerError, UploadResponse{
				Success: false,
				Message: "Gagal menyimpan file",
			})
			return
		}
		log.Printf("[DEBUG-UPLOAD] Original saved: %s", tempOriginal)

		maxDimension := 1280
		if req.Type == "front" {
			maxDimension = 1920
		}
		log.Printf("[DEBUG-UPLOAD] Starting CompressAndSave: original=%s dest=%s maxDim=%d", tempOriginal, finalPath, maxDimension)

		if err := h.imageService.CompressAndSave(tempOriginal, finalPath, maxDimension); err != nil {
			log.Printf("[DEBUG-UPLOAD] ERROR CompressAndSave: %v", err)
			os.Remove(tempOriginal)
			c.JSON(http.StatusInternalServerError, UploadResponse{
				Success: false,
				Message: "Gagal memproses gambar",
			})
			return
		}
		log.Printf("[DEBUG-UPLOAD] CompressAndSave done, removing original")
		os.Remove(tempOriginal)
	}

	log.Printf("[DEBUG-UPLOAD] SUCCESS: returning file_ref=%s", finalFilename)
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
		log.Printf("[DEBUG-UPLOAD] DeleteTempFile: invalid request: %v", err)
		h.errJSON(c, http.StatusBadRequest, "Request tidak valid")
		return
	}

	log.Printf("[DEBUG-UPLOAD] DeleteTempFile: file_ref=%q", req.FileRef)
	if req.FileRef != "" {
		tempPath := filepath.Join("uploads", "temp", req.FileRef)
		if err := os.Remove(tempPath); err != nil {
			log.Printf("[DEBUG-UPLOAD] DeleteTempFile: remove %s: %v (ignored)", tempPath, err)
		} else {
			log.Printf("[DEBUG-UPLOAD] DeleteTempFile: removed %s", tempPath)
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// CleanupTempFiles handles multiple temp files deletion
func (h *Handler) CleanupTempFiles(c *gin.Context) {
	var req CleanupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[DEBUG-UPLOAD] CleanupTempFiles: invalid request: %v", err)
		h.errJSON(c, http.StatusBadRequest, "Request tidak valid")
		return
	}

	log.Printf("[DEBUG-UPLOAD] CleanupTempFiles: %d files", len(req.FileRefs))
	// Cleanup multiple files
	for _, fileRef := range req.FileRefs {
		if fileRef != "" {
			tempPath := filepath.Join("uploads", "temp", fileRef)
			if err := os.Remove(tempPath); err != nil {
				log.Printf("[DEBUG-UPLOAD] CleanupTempFiles: remove %s: %v (ignored)", tempPath, err)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}