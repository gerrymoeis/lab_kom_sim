package main

import (
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/handlers"
	"inventaris-lab-kom/internal/middleware"

	"github.com/gin-gonic/gin"
)

// loadTemplates loads all HTML templates from the templates directory
func loadTemplates(templatesDir string) (*template.Template, error) {
	// Create template with custom functions
	templ := template.New("").Funcs(template.FuncMap{
		"add": func(a, b int) int {
			return a + b
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

	// Initialize database
	db, err := database.InitDB(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := database.RunMigrations(db); err != nil {
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
		protected.GET("/pc/:id", h.PCDetail)
		protected.GET("/pc/:id/edit", h.PCEditPage)
		protected.POST("/pc/:id/edit", h.PCEdit)
		protected.POST("/pc/:id/delete", h.PCDelete)

		// Device Management
		protected.GET("/devices", h.DeviceList)
		protected.GET("/devices/create", h.DeviceCreatePage)
		protected.POST("/devices/create", h.DeviceCreate)
		protected.GET("/devices/:id/edit", h.DeviceEditPage)
		protected.POST("/devices/:id/edit", h.DeviceEdit)
		protected.POST("/devices/:id/delete", h.DeviceDelete)

		// Software Tracking
		protected.GET("/software", h.SoftwareList)
		protected.POST("/software/create", h.SoftwareCreate)

		// OCR Logbook
		protected.GET("/logbook", h.LogbookList)
		protected.GET("/logbook/upload", h.LogbookUploadPage)
		protected.POST("/logbook/upload", h.LogbookUpload)
		protected.POST("/logbook/save", h.LogbookSave)
		protected.GET("/logbook/export", h.LogbookExport)
		protected.GET("/logbook/export-preview", h.LogbookExportPreview)

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
	}

	// Create uploads directory if not exists
	if err := os.MkdirAll("uploads", 0755); err != nil {
		log.Printf("Warning: Failed to create uploads directory: %v", err)
	}

	// Start server
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	log.Printf("🚀 Server starting on http://%s", addr)
	log.Printf("📊 Environment: %s", cfg.Environment)
	log.Printf("💾 Database: %s", cfg.DatabasePath)
	
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
