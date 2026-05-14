package main

import (
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"time"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/handlers"
	"inventaris-lab-kom/internal/middleware"

	"github.com/gin-gonic/gin"
)

// cleanupTempFiles removes temporary files older than 1 hour
func cleanupTempFiles() {
	tempDir := filepath.Join("uploads", "temp")
	cutoff := time.Now().Add(-1 * time.Hour)
	
	err := filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on error
		}
		
		if !info.IsDir() && info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err == nil {
				log.Printf("Cleaned up temp file: %s", path)
			}
		}
		
		return nil
	})
	
	if err != nil {
		log.Printf("Warning: Failed to cleanup temp files: %v", err)
	}
}

// loadTemplates loads all HTML templates from the templates directory
func loadTemplates(templatesDir string) (*template.Template, error) {
	// Create template with custom functions
	templ := template.New("").Funcs(template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"iterate": func(count int) []int {
			result := make([]int, count)
			for i := 0; i < count; i++ {
				result[i] = i
			}
			return result
		},
	})
	
	err := filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories
		if info.IsDir() {
			return nil
		}
		
		// Only process .html files
		if filepath.Ext(path) == ".html" {
			_, err = templ.ParseFiles(path)
			if err != nil {
				return err
			}
		}
		
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	return templ, nil
}

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize database (PostgreSQL if DATABASE_URL set, else SQLite)
	isPostgres := cfg.DatabaseURL != ""
	db, err := database.InitDB(cfg.DatabasePath, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := database.RunMigrations(db, isPostgres); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Seed default admin user if not exists
	if err := database.SeedDefaultUser(db); err != nil {
		log.Printf("Warning: Failed to seed default user: %v", err)
	}

	// Set Gin mode
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize Gin router
	router := gin.Default()

	// Load HTML templates - Walk through all subdirectories
	templ, err := loadTemplates("web/templates")
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}
	router.SetHTMLTemplate(templ)

	// Serve static files
	router.Static("/static", "./web/static")
	router.Static("/uploads", "./uploads")

	// Initialize session middleware
	sessionMiddleware := middleware.SessionMiddleware(cfg.SessionSecret)
	router.Use(sessionMiddleware)

	// Initialize handlers
	h := handlers.NewHandler(db, cfg)

	// Public routes
	public := router.Group("/")
	{
		public.GET("/", h.Home)
		public.GET("/login", h.LoginPage)
		public.POST("/login", h.Login)
		public.GET("/logout", h.Logout)
	}

	// Protected routes (require authentication)
	protected := router.Group("/")
	protected.Use(middleware.AuthRequired())
	{
		// Dashboard
		protected.GET("/dashboard", h.Dashboard)

		// PC Management
		protected.GET("/pc", h.PCList)
		protected.GET("/pc/create", h.PCCreatePage)
		protected.POST("/pc/create", h.PCCreate)
		protected.GET("/pc/export", h.PCExport)
		protected.GET("/pc/:pc_number", h.PCDetail)
		protected.GET("/pc/:pc_number/edit", h.PCEditPage)
		protected.POST("/pc/:pc_number/edit", h.PCEdit)
		protected.POST("/pc/:pc_number/delete", h.PCDelete)

		// Device Management
		protected.GET("/devices", h.DeviceList)
		protected.GET("/devices/create", h.DeviceCreatePage)
		protected.POST("/devices/create", h.DeviceCreate)
		protected.GET("/devices/export", h.DeviceExport)
		protected.GET("/devices/:id", h.DeviceDetail)
		protected.GET("/devices/:id/edit", h.DeviceEditPage)
		protected.POST("/devices/:id/edit", h.DeviceEdit)
		protected.POST("/devices/:id/delete", h.DeviceDelete)

		// Device Types
		protected.GET("/device-types", h.DeviceTypeList)
		protected.GET("/device-types/create", h.DeviceTypeCreatePage)
		protected.POST("/device-types/create", h.DeviceTypeCreate)
		protected.GET("/device-types/:id", h.DeviceTypeDetail)
		protected.GET("/device-types/:id/edit", h.DeviceTypeEditPage)
		protected.POST("/device-types/:id/edit", h.DeviceTypeEdit)
		protected.POST("/device-types/:id/delete", h.DeviceTypeDelete)

		// Device Loans
		protected.GET("/device-loans", h.DeviceLoanList)
		protected.GET("/device-loans/create", h.DeviceLoanCreatePage)
		protected.POST("/device-loans/create", h.DeviceLoanCreate)
		protected.GET("/device-loans/:id/edit", h.DeviceLoanEditPage)
		protected.POST("/device-loans/:id/edit", h.DeviceLoanEdit)
		protected.POST("/device-loans/:id/delete", h.DeviceLoanDelete)

		// Device Usages
		protected.GET("/device-usages", h.DeviceUsageList)
		protected.GET("/device-usages/create", h.DeviceUsageCreatePage)
		protected.POST("/device-usages/create", h.DeviceUsageCreate)
		protected.GET("/device-usages/:id/edit", h.DeviceUsageEditPage)
		protected.POST("/device-usages/:id/edit", h.DeviceUsageEdit)
		protected.POST("/device-usages/:id/delete", h.DeviceUsageDelete)

		// Software Tracking
		protected.GET("/software", h.SoftwareList)
		protected.POST("/software/create", h.SoftwareCreate)
		protected.GET("/software/export", h.SoftwareExport)
		protected.GET("/software/catalog.json", h.GetSoftwareCatalogJSON)

		// OCR Logbook
		protected.GET("/logbook", h.LogbookList)
		protected.GET("/logbook/upload", h.LogbookUploadPage)
		protected.POST("/logbook/upload", h.LogbookUpload)
		protected.POST("/logbook/save", h.LogbookSave)
		protected.GET("/logbook/export", h.LogbookExport)
		protected.GET("/logbook/export-preview", h.LogbookExportPreview)
		// Manual CRUD for logbook
		protected.GET("/logbook/create", h.LogbookCreatePage)
		protected.POST("/logbook/create", h.LogbookCreate)
		protected.GET("/logbook/:id/edit", h.LogbookEditPage)
		protected.POST("/logbook/:id/edit", h.LogbookEdit)
		protected.POST("/logbook/:id/delete", h.LogbookDelete)

		// Experiment: OCR Finance Table (Testing only)
		protected.GET("/experiment/ocr", h.ExperimentOCRPage)
		protected.POST("/experiment/ocr/upload", h.ExperimentOCRUpload)

		// User Management (Admin only)
		admin := protected.Group("/admin")
		admin.Use(middleware.AdminRequired())
		{
			admin.GET("/users", h.UserList)
			admin.GET("/users/create", h.UserCreatePage)
			admin.POST("/users/create", h.UserCreate)
			admin.POST("/users/:id/delete", h.UserDelete)
			
			// Activity Logs (Admin only)
			admin.GET("/activity-logs", h.ActivityLogList)
			admin.GET("/activity-logs/export", h.ActivityLogExport)
		}

		// Profile
		protected.GET("/profile", h.Profile)
		protected.POST("/profile/password", h.ChangePassword)
	}

	// API routes (for HTMX/AJAX)
	api := router.Group("/api")
	api.Use(middleware.AuthRequired())
	{
		api.GET("/pc/status", h.PCStatusAPI)
		api.POST("/pc/:id/status", h.UpdatePCStatusAPI)
		api.POST("/upload-image", h.UploadImage) // Image upload endpoint
		api.POST("/delete-temp-file", h.DeleteTempFile) // Single temp file cleanup
		api.POST("/cleanup-temp-files", h.CleanupTempFiles) // Multiple temp files cleanup
		api.GET("/devices/next-asset-code", h.GetNextAssetCode) // Get next asset code for device
	}

	// Create uploads directory if not exists
	if err := os.MkdirAll("uploads", 0755); err != nil {
		log.Printf("Warning: Failed to create uploads directory: %v", err)
	}
	
	// Create temp directory if not exists
	if err := os.MkdirAll("uploads/temp", 0755); err != nil {
		log.Printf("Warning: Failed to create temp directory: %v", err)
	}

	// Start cleanup goroutine for temporary files
	go func() {
		for {
			time.Sleep(30 * time.Minute) // Run every 30 minutes
			cleanupTempFiles()
		}
	}()

	// Start server
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	log.Printf("🚀 Server starting on http://%s", addr)
	log.Printf("📊 Environment: %s", cfg.Environment)
	log.Printf("💾 Database: %s", cfg.DatabasePath)
	
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
