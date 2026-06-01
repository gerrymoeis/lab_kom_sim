package server

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/handlers"
	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/services"
	"inventaris-lab-kom/internal/timeutil"

	"github.com/gin-gonic/gin"
)

type NavItem struct {
	Page, Icon, Label, URL string
}

type Category struct {
	Value, Label string
}

type PCStatusInfo struct {
	Status, BadgeClass, Icon, Color, VisLabel string
}

func loadCategories() []Category {
	return []Category{
		{"peripheral", "Peripheral"}, {"network", "Network"},
		{"consumable", "Consumable"}, {"power", "Power"},
		{"display", "Display"}, {"printer", "Printer"},
		{"audio", "Audio"}, {"tools", "Tools"},
		{"server", "Server"}, {"security", "Security"},
		{"stationery", "Stationery"},
	}
}

var pcStatusMap = map[string]PCStatusInfo{
	"normal":  {"normal", "success", "bi-check-circle-fill", "text-success", "Normal"},
	"warning": {"warning", "warning", "bi-exclamation-triangle-fill", "text-warning", "Warning"},
	"broken":  {"broken", "danger", "bi-x-circle-fill", "text-danger", "Rusak"},
}

func getPCStatusInfo(status string) PCStatusInfo {
	if s, ok := pcStatusMap[status]; ok {
		return s
	}
	return pcStatusMap["normal"]
}

type PlacementInfo struct {
	Placement, BadgeClass, Icon, VisLabel string
}

var pcPlacementMap = map[string]PlacementInfo{
	"dipakai":  {"dipakai", "primary", "bi-check-lg", "Dipakai"},
	"cadangan": {"cadangan", "secondary", "bi-box-seam", "Cadangan"},
}

func getPCPlacementInfo(placement string) PlacementInfo {
	if p, ok := pcPlacementMap[placement]; ok {
		return p
	}
	return pcPlacementMap["dipakai"]
}

func loadNavItems(currentPage, role string) []NavItem {
	items := []NavItem{
		{"dashboard", "bi-grid-3x3-gap", "Dashboard", "/dashboard"},
		{"pc", "bi-pc-display", "PC", "/pc"},
		{"devices", "bi-hdd-rack", "Perangkat", "/devices"},
		{"software", "bi-app-indicator", "Software", "/software"},
		{"schedules", "bi-calendar-event", "Jadwal", "/schedules"},
		{"logbook", "bi-journal-text", "Logbook", "/logbook"},
	}
	if role == "admin" {
		items = append(items,
			NavItem{"users", "bi-people", "Users", "/admin/users"},
			NavItem{"activity_logs", "bi-clock-history", "Activity Logs", "/admin/activity-logs"},
		)
	}
	return items
}

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
		"navItems":        func(currentPage, role string) []NavItem { return loadNavItems(currentPage, role) },
		"allCategories":   func() []Category { return loadCategories() },
		"pcStatusInfo":    func(status string) PCStatusInfo { return getPCStatusInfo(status) },
		"pcPlacementInfo": func(placement string) PlacementInfo { return getPCPlacementInfo(placement) },
		"formatPCLabel": func(pc models.PC) string {
			if pc.Label != "" { return pc.Label }
			return "-"
		},
		"localTime": func(t interface{}) interface{} {
			switch v := t.(type) {
			case time.Time:
				if v.IsZero() { return v }
				return v.In(timeutil.Location())
			case *time.Time:
				if v == nil || v.IsZero() { return v }
				return v.In(timeutil.Location())
			}
			return t
		},
		"tzCode": func() string { return timeutil.Code() },
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

func sourceMapBlocker() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasSuffix(c.Request.URL.Path, ".map") {
			c.AbortWithStatus(404)
			return
		}
		c.Next()
	}
}

func SetupRouter(db *database.DB, cfg *config.Config, notifier services.CUDNotifier) (*gin.Engine, func()) {
	router := gin.New()
	router.Use(sourceMapBlocker())
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	templ, err := LoadTemplates("web/templates")
	if err != nil {
		panic(fmt.Sprintf("Failed to load templates: %v", err))
	}
	router.SetHTMLTemplate(templ)

	router.Static("/static", "./web/static")
	router.Static("/uploads", "./uploads")

	sessionMiddleware := middleware.SessionMiddleware(cfg.SessionSecret)
	router.Use(sessionMiddleware)

	// writeFlushMiddleware ensures all pending async writes are flushed
	// before the response is sent, preventing stale data on POST-then-redirect.
	writeFlushMiddleware := func() gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Next()
			if c.Request.Method == "POST" {
				db.Flush()
			}
		}
	}

	h := handlers.NewHandler(db, cfg, notifier)

	router.GET("/healthz", func(c *gin.Context) {
		c.String(200, "ok")
	})
	router.GET("/readyz", func(c *gin.Context) {
		if err := db.Ping(); err != nil {
			c.String(503, "not ready")
			return
		}
		c.String(200, "ready")
	})

	public := router.Group("/")
	{
		public.GET("/", h.Home)
		public.GET("/login", h.LoginPage)
		public.POST("/login", h.Login)
		public.GET("/logout", h.Logout)
	}

	protected := router.Group("/")
	protected.Use(middleware.AuthRequired(db), writeFlushMiddleware())
	{
		protected.GET("/dashboard", h.Dashboard)
		protected.GET("/pc", h.PCList)
		protected.GET("/pc/create", h.PCCreatePage)
		protected.POST("/pc/create", h.PCCreate)
		protected.GET("/pc/export", h.PCExport)
		protected.GET("/pc/:label", h.PCDetail)
		protected.GET("/pc/:label/edit", h.PCEditPage)
		protected.POST("/pc/:label/edit", h.PCEdit)
		protected.POST("/pc/:label/delete", h.PCDelete)

		protected.GET("/devices", h.DeviceList)
		protected.GET("/devices/create", h.DeviceCreatePage)
		protected.POST("/devices/create", h.DeviceCreate)
		protected.POST("/devices/batch-create", h.DeviceBatchCreate)
		protected.GET("/devices/:id", h.DeviceDetail)
		protected.GET("/devices/:id/edit", h.DeviceEditPage)
		protected.POST("/devices/:id/edit", h.DeviceEdit)
		protected.POST("/devices/:id/delete", h.DeviceDelete)

		protected.GET("/device-types/:id/edit", h.DeviceTypeEditPage)
		protected.POST("/device-types/:id/edit", h.DeviceTypeEdit)
		protected.POST("/device-types/:id/delete", h.DeviceTypeDelete)

		protected.GET("/device-loans", func(c *gin.Context) { c.Redirect(http.StatusFound, "/devices?tab=loans") })
		protected.GET("/device-loans/create", h.DeviceLoanCreatePage)
		protected.POST("/device-loans/create", h.DeviceLoanCreate)
		protected.GET("/device-loans/:id/edit", h.DeviceLoanEditPage)
		protected.POST("/device-loans/:id/edit", h.DeviceLoanEdit)
		protected.POST("/device-loans/:id/delete", h.DeviceLoanDelete)

		protected.GET("/device-usages", func(c *gin.Context) { c.Redirect(http.StatusFound, "/devices?tab=usages") })
		protected.GET("/device-usages/create", h.DeviceUsageCreatePage)
		protected.POST("/device-usages/create", h.DeviceUsageCreate)
		protected.GET("/device-usages/:id/edit", h.DeviceUsageEditPage)
		protected.POST("/device-usages/:id/edit", h.DeviceUsageEdit)
		protected.POST("/device-usages/:id/delete", h.DeviceUsageDelete)
		protected.POST("/device-loans/:id/extend", h.DeviceLoanExtend)
		protected.GET("/installations", func(c *gin.Context) { c.Redirect(http.StatusFound, "/devices?tab=installations") })
		protected.GET("/installations/create", h.DeviceInstallationCreatePage)
		protected.GET("/installations/:id", h.DeviceInstallationDetail)
		protected.POST("/installations/create", h.DeviceInstallationCreate)
		protected.GET("/installations/:id/edit", h.DeviceInstallationEditPage)
		protected.POST("/installations/:id/edit", h.DeviceInstallationEdit)
		protected.POST("/installations/:id/delete", h.DeviceInstallationDelete)
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
		protected.GET("/software/:id", h.SoftwareDetail)
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
		protected.GET("/logbook/:id", h.LogbookDetail)
		protected.GET("/logbook/:id/edit", h.LogbookEditPage)
		protected.POST("/logbook/:id/edit", h.LogbookEdit)
		protected.POST("/logbook/:id/delete", h.LogbookDelete)

		admin := protected.Group("/admin")
		admin.Use(middleware.AdminRequired())
		{
			admin.GET("/users", h.UserList)
			admin.GET("/users/create", h.UserCreatePage)
			admin.POST("/users/create", h.UserCreate)
			admin.GET("/users/:id", h.UserDetail)
			admin.GET("/users/:id/edit", h.UserEditPage)
			admin.POST("/users/:id/edit", h.UserEdit)
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
		api.GET("/pc/layout", h.PCGetLayout)
		api.POST("/pc/swap", h.PCSwap)
		api.POST("/pc/replace", h.PCReplace)
		api.POST("/pc/move-row", h.PCMoveRowToCadangan)
		api.POST("/pc/move", h.PCMove)
		api.POST("/pc/place", h.PCPlace)
		api.POST("/upload-image", h.UploadImage)
		api.POST("/delete-temp-file", h.DeleteTempFile)
		api.POST("/cleanup-temp-files", h.CleanupTempFiles)
		api.GET("/devices/next-asset-code", h.GetNextAssetCode)
	}

	return router, func() { h.Close() }
}
