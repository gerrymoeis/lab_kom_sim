package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

// ExperimentOCRPage renders experiment OCR upload page
func (h *Handler) ExperimentOCRPage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	c.HTML(http.StatusOK, "experiment/ocr.html", gin.H{
		"title":    "Experiment: OCR Finance Table - Sistem Inventaris Lab",
		"username": username,
		"role":     role,
	})
}

// ExperimentOCRUpload handles experiment OCR file upload and processing
func (h *Handler) ExperimentOCRUpload(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Get uploaded file
	file, err := c.FormFile("finance_image")
	if err != nil {
		c.HTML(http.StatusBadRequest, "experiment/ocr.html", gin.H{
			"title":    "Experiment: OCR Finance Table - Sistem Inventaris Lab",
			"username": username,
			"role":     role,
			"error":    "Gagal mengambil file. Pastikan Anda memilih file gambar.",
		})
		return
	}

	// Validate file type
	ext := filepath.Ext(file.Filename)
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		c.HTML(http.StatusBadRequest, "experiment/ocr.html", gin.H{
			"title":    "Experiment: OCR Finance Table - Sistem Inventaris Lab",
			"username": username,
			"role":     role,
			"error":    "Format file tidak didukung. Gunakan JPG atau PNG.",
		})
		return
	}

	// Create upload directory if not exists
	uploadDir := filepath.Join("uploads", "experiment")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal membuat direktori upload",
		})
		return
	}

	// Save file with timestamp
	filename := fmt.Sprintf("finance_%d%s", time.Now().Unix(), ext)
	filepath := filepath.Join(uploadDir, filename)
	
	if err := c.SaveUploadedFile(file, filepath); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal menyimpan file",
		})
		return
	}

	// Get Gemini API key from config
	apiKey := h.cfg.GeminiAPIKey
	if apiKey == "" {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gemini API key tidak dikonfigurasi. Silakan tambahkan GEMINI_API_KEY di file .env",
		})
		return
	}

	// Process OCR
	ocrService := services.NewOCRService(apiKey, h.cfg.OpenRouterAPIKey)
	result, err := ocrService.ExtractFinanceTableFromImage(filepath)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": fmt.Sprintf("Gagal memproses OCR: %v", err),
		})
		return
	}

	// Redirect to preview page with extracted data
	c.HTML(http.StatusOK, "experiment/preview.html", gin.H{
		"title":      "Preview Hasil OCR Finance - Sistem Inventaris Lab",
		"username":   username,
		"role":       role,
		"entries":    result.Entries,
		"raw_text":   result.RawText,
		"success":    result.Success,
		"error":      result.Error,
		"source_file": filename,
	})
}
