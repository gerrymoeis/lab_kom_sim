package server

import (
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

func CleanupTempFiles() {
	filepath.Walk(filepath.Join("uploads", "temp"),
		func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && info.ModTime().Before(time.Now().Add(-1*time.Hour)) {
				os.Remove(path)
			}
			return nil
		})
}

func LoadTemplates(templatesDir string) (*template.Template, error) {
	templ := template.New("").Funcs(template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"iterate": func(count int) []int {
			r := make([]int, count)
			for i := 0; i < count; i++ { r[i] = i }
			return r
		},
	})
	err := filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil { return err }
		if info.IsDir() || filepath.Ext(path) != ".html" { return nil }
		relPath, _ := filepath.Rel(templatesDir, path)
		content, err := os.ReadFile(path)
		if err != nil { return err }
		_, err = templ.New(filepath.ToSlash(relPath)).Parse(string(content))
		return err
	})
	return templ, err
}

func SetupRouter(db *database.DB, cfg *config.Config) *gin.Engine {
	router := gin.Default()

	templ, err := LoadTemplates("web/templates")
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}
	router.SetHTMLTemplate(templ)

	router.Static("/static", "./web/static")
	router.Static("/uploads", "./uploads")

	sessionMiddleware := middleware.SessionMiddleware(cfg.SessionSecret)
	router.Use(sessionMiddleware)

	h := handlers.NewHandler(db, cfg)

	public := router.Group("/")
	{
		public.GET("/", h.Home)
		public.GET("/login", h.LoginPage)
		public.POST("/login", h.Login)
		public.GET("/logout", h.Logout)
	}

	protected := router.Group("/")
	protected.Use(middleware.AuthRequired(db))
	{
		protected.GET("/dashboard", h.Dashboard)
		protected.GET("/pc", h.PCList)
		protected.GET("/pc/create", h.PCCreatePage)
		protected.POST("/pc/create", h.PCCreate)
		protected.GET("/pc/export", h.PCExport)
		protected.GET("/pc/:pc_number", h.PCDetail)
		protected.GET("/pc/:pc_number/edit", h.PCEditPage)
		protected.POST("/pc/:pc_number/edit", h.PCEdit)
		protected.POST("/pc/:pc_number/delete", h.PCDelete)

		protected.GET("/devices", h.DeviceList)
		protected.GET("/devices/create", h.DeviceCreatePage)
		protected.POST("/devices/create", h.DeviceCreate)
		protected.GET("/devices/export", h.DeviceExport)
		protected.GET("/devices/:id", h.DeviceDetail)
		protected.GET("/devices/:id/edit", h.DeviceEditPage)
		protected.POST("/devices/:id/edit", h.DeviceEdit)
		protected.POST("/devices/:id/delete", h.DeviceDelete)

		protected.GET("/device-types", h.DeviceTypeList)
		protected.GET("/device-types/create", h.DeviceTypeCreatePage)
		protected.POST("/device-types/create", h.DeviceTypeCreate)
		protected.GET("/device-types/:id", h.DeviceTypeDetail)
		protected.GET("/device-types/:id/edit", h.DeviceTypeEditPage)
		protected.POST("/device-types/:id/edit", h.DeviceTypeEdit)
		protected.POST("/device-types/:id/delete", h.DeviceTypeDelete)

		protected.GET("/device-loans", h.DeviceLoanList)
		protected.GET("/device-loans/create", h.DeviceLoanCreatePage)
		protected.POST("/device-loans/create", h.DeviceLoanCreate)
		protected.GET("/device-loans/:id/edit", h.DeviceLoanEditPage)
		protected.POST("/device-loans/:id/edit", h.DeviceLoanEdit)
		protected.POST("/device-loans/:id/delete", h.DeviceLoanDelete)

		protected.GET("/device-usages", h.DeviceUsageList)
		protected.GET("/device-usages/create", h.DeviceUsageCreatePage)
		protected.POST("/device-usages/create", h.DeviceUsageCreate)
		protected.GET("/device-usages/:id/edit", h.DeviceUsageEditPage)
		protected.POST("/device-usages/:id/edit", h.DeviceUsageEdit)
		protected.POST("/device-usages/:id/delete", h.DeviceUsageDelete)
		protected.POST("/device-usages/:id/availability", h.DeviceUsageUpdateAvailability)

		protected.GET("/schedules", h.ScheduleList)
		protected.GET("/schedules/create", h.ScheduleCreatePage)
		protected.POST("/schedules/create", h.ScheduleCreate)
		protected.GET("/schedules/:id/edit", h.ScheduleEditPage)
		protected.POST("/schedules/:id/edit", h.ScheduleEdit)
		protected.POST("/schedules/:id/delete", h.ScheduleDelete)

		protected.GET("/software", h.SoftwareList)
		protected.POST("/software/create", h.SoftwareCreate)
		protected.GET("/software/export", h.SoftwareExport)
		protected.GET("/software/catalog.json", h.GetSoftwareCatalogJSON)
		protected.GET("/software/:id/edit", h.SoftwareEditPage)
		protected.POST("/software/:id/edit", h.SoftwareEdit)
		protected.POST("/software/:id/delete", h.SoftwareDelete)

		protected.GET("/logbook", h.LogbookList)
		protected.GET("/logbook/upload", h.LogbookUploadPage)
		protected.POST("/logbook/upload", h.LogbookUpload)
		protected.POST("/logbook/save", h.LogbookSave)
		protected.GET("/logbook/export", h.LogbookExport)
		protected.GET("/logbook/export-preview", h.LogbookExportPreview)
		protected.GET("/logbook/create", h.LogbookCreatePage)
		protected.POST("/logbook/create", h.LogbookCreate)
		protected.GET("/logbook/:id/edit", h.LogbookEditPage)
		protected.POST("/logbook/:id/edit", h.LogbookEdit)
		protected.POST("/logbook/:id/delete", h.LogbookDelete)

		admin := protected.Group("/admin")
		admin.Use(middleware.AdminRequired())
		{
			admin.GET("/users", h.UserList)
			admin.GET("/users/create", h.UserCreatePage)
			admin.POST("/users/create", h.UserCreate)
			admin.POST("/users/:id/delete", h.UserDelete)
			admin.GET("/activity-logs", h.ActivityLogList)
			admin.GET("/activity-logs/export", h.ActivityLogExport)
		}

		protected.GET("/profile", h.Profile)
		protected.POST("/profile", h.UpdateProfile)
		protected.POST("/profile/password", h.ChangePassword)
	}

	api := router.Group("/api")
	api.Use(middleware.AuthRequired(db))
	{
		api.GET("/pc/status", h.PCStatusAPI)
		api.POST("/pc/:id/status", h.UpdatePCStatusAPI)
		api.POST("/upload-image", h.UploadImage)
		api.POST("/delete-temp-file", h.DeleteTempFile)
		api.POST("/cleanup-temp-files", h.CleanupTempFiles)
		api.GET("/devices/next-asset-code", h.GetNextAssetCode)
	}

	return router
}
