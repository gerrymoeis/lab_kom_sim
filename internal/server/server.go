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
	"inventaris-lab-kom/internal/versioner"

	"github.com/gin-contrib/gzip"
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
		{"print", "bi-printer", "Print Stiker", "/print"},
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

func LoadTemplates(templatesDir string, staticURL func(string) string) (*template.Template, error) {
	templ := template.New("").Funcs(template.FuncMap{
		"staticURL": staticURL,
		"add":       func(a, b int) int { return a + b },
		"sub":       func(a, b int) int { return a - b },
		"iterate": func(count int) []int {
			r := make([]int, count)
			for i := 0; i < count; i++ { r[i] = i }
			return r
		},
		"lower":           func(s string) string { return strings.ToLower(s) },
		"navItems":        func(currentPage, role string) []NavItem { return loadNavItems(currentPage, role) },
		"allCategories":   func() []Category { return loadCategories() },
		"pcStatusInfo":    func(status string) PCStatusInfo { return getPCStatusInfo(status) },
		"pcPlacementInfo": func(placement string) PlacementInfo { return getPCPlacementInfo(placement) },
		"isSpecialLabel": func(label, placement string) bool {
			if placement != "dipakai" { return false }
			if len(label) < 4 || !strings.HasPrefix(label, "pc-") { return false }
			for _, c := range label[3:] {
				if c >= '0' && c <= '9' { continue }
				return true
			}
			return false
		},
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
		"canAccessUser": func(actorUsername string, targetUser models.User, actorIsSuperAdmin bool) bool {
			return handlers.CanAccessProfile(actorUsername, targetUser, actorIsSuperAdmin)
		},
		"currentDateAfter": func(t time.Time) bool { return time.Now().After(t) },
		"daysBetween": func(start, end *time.Time) string {
			if start == nil || end == nil { return "-" }
			return fmt.Sprintf("%d", int(end.Sub(*start).Hours()/24)+1)
		},
	})
	err := filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil { return err }
		if info.IsDir() || filepath.Ext(path) != ".html" { return nil }
		relPath, _ := filepath.Rel(templatesDir, path)
		if strings.HasPrefix(filepath.ToSlash(relPath), "public/") { return nil }
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

func SetupRouter(dbs map[string]*database.DB, cfg *config.Config, notifier services.CUDNotifier) (*gin.Engine, func(), func()) {
	router := gin.New()
	router.MaxMultipartMemory = 6 << 20
	router.Use(sourceMapBlocker())
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.CacheControl())
	router.Use(middleware.SecurityHeaders(cfg.Environment))
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	v, err := versioner.New("./web/static")
	if err != nil {
		panic(fmt.Sprintf("Failed to init versioner: %v", err))
	}

	templ, err := LoadTemplates("web/templates", v.URL)
	if err != nil {
		panic(fmt.Sprintf("Failed to load templates: %v", err))
	}
	router.SetHTMLTemplate(templ)

	router.GET("/static/*filepath", v.Handler())
	router.Static("/uploads", "./uploads")

	sessionMiddleware := middleware.SessionMiddleware(cfg.SessionSecret, cfg.CookieSecure)
	router.Use(sessionMiddleware)

	labCfgs := make(map[string]config.LabConfig)
	if cfg.Labs != nil {
		for i := range cfg.Labs {
			labCfgs[cfg.Labs[i].Name] = cfg.Labs[i]
		}
	}

	handlersMap := make(map[string]*handlers.Handler, len(dbs))
	for labName, db := range dbs {
		handlersMap[labName] = handlers.NewHandler(db, cfg, notifier)
	}
	adapter := NewHandlerAdapter(handlersMap)

	writeFlushMiddleware := func() gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Next()
			if c.Request.Method == "POST" {
				if dbVal, exists := c.Get("db"); exists {
					if db, ok := dbVal.(*database.DB); ok {
						db.Flush()
					}
				}
			}
		}
	}

	readyzDB := func() *database.DB {
		for _, db := range dbs {
			return db
		}
		return nil
	}()

	router.GET("/healthz", func(c *gin.Context) {
		c.String(200, "ok")
	})
	router.GET("/readyz", func(c *gin.Context) {
		if readyzDB == nil || readyzDB.Ping() != nil {
			c.String(503, "not ready")
			return
		}
		c.String(200, "ready")
	})

	labGroup := router.Group("/:lab")
	labGroup.Use(middleware.DBInjector(dbs, labCfgs))
	{
		public := labGroup.Group("/")
		public.Use(middleware.CSRF())
		{
			public.GET("/", adapter.Handle((*handlers.Handler).Home))
			public.GET("/login", adapter.Handle((*handlers.Handler).LoginPage))
			public.POST("/login", adapter.Handle((*handlers.Handler).Login))
			public.POST("/logout", adapter.Handle((*handlers.Handler).Logout))
		}

		protected := labGroup.Group("/")
		protected.Use(middleware.AuthRequired(), middleware.CSRF(), writeFlushMiddleware())
		{
			protected.GET("/dashboard", adapter.Handle((*handlers.Handler).Dashboard))
			protected.GET("/pc", adapter.Handle((*handlers.Handler).PCList))
			protected.GET("/pc/create", adapter.Handle((*handlers.Handler).PCCreatePage))
			protected.POST("/pc/create", adapter.Handle((*handlers.Handler).PCCreate))
			protected.GET("/pc/export", adapter.Handle((*handlers.Handler).PCExport))
			protected.GET("/pc/:label", adapter.Handle((*handlers.Handler).PCDetail))
			protected.GET("/pc/:label/edit", adapter.Handle((*handlers.Handler).PCEditPage))
			protected.POST("/pc/:label/edit", adapter.Handle((*handlers.Handler).PCEdit))
			protected.POST("/pc/:label/delete", adapter.Handle((*handlers.Handler).PCDelete))
			protected.POST("/pc/batch-delete", adapter.Handle((*handlers.Handler).PCBatchDelete))

			protected.GET("/devices", adapter.Handle((*handlers.Handler).DeviceList))
			protected.GET("/devices/export", adapter.Handle((*handlers.Handler).DeviceExport))
			protected.GET("/devices/create", adapter.Handle((*handlers.Handler).DeviceCreatePage))
			protected.POST("/devices/create", adapter.Handle((*handlers.Handler).DeviceCreate))
			protected.POST("/devices/batch-create", adapter.Handle((*handlers.Handler).DeviceBatchCreate))
			protected.GET("/devices/:slug/:typeSlug/:assetCode", adapter.Handle((*handlers.Handler).DeviceDetail))
			protected.GET("/devices/:slug/edit", adapter.Handle((*handlers.Handler).DeviceEditPage))
			protected.POST("/devices/:slug/edit", adapter.Handle((*handlers.Handler).DeviceEdit))
			protected.POST("/devices/:slug/delete", adapter.Handle((*handlers.Handler).DeviceDelete))
			protected.POST("/devices/batch-delete", adapter.Handle((*handlers.Handler).DeviceBatchDelete))

			protected.GET("/device-types/:slug", adapter.Handle((*handlers.Handler).DeviceTypeDetail))
			protected.GET("/device-types/:slug/edit", adapter.Handle((*handlers.Handler).DeviceTypeEditPage))
			protected.POST("/device-types/:slug/edit", adapter.Handle((*handlers.Handler).DeviceTypeEdit))
			protected.POST("/device-types/:slug/delete", adapter.Handle((*handlers.Handler).DeviceTypeDelete))
			protected.POST("/device-types/batch-delete", adapter.Handle((*handlers.Handler).DeviceTypeBatchDelete))
			protected.GET("/categories/:slug", adapter.Handle((*handlers.Handler).CategoryDetail))
			protected.GET("/categories/:slug/edit", adapter.Handle((*handlers.Handler).CategoryEditPage))
			protected.POST("/categories/:slug/edit", adapter.Handle((*handlers.Handler).CategoryEdit))
			protected.POST("/categories/:slug/delete", adapter.Handle((*handlers.Handler).CategoryDelete))
			protected.POST("/categories/batch-delete", adapter.Handle((*handlers.Handler).CategoryBatchDelete))

			protected.GET("/device-loans", func(c *gin.Context) { middleware.LabRedirect(c, http.StatusFound, "/devices?tab=loans") })
			protected.GET("/device-loans/create", adapter.Handle((*handlers.Handler).DeviceLoanCreatePage))
			protected.POST("/device-loans/create", adapter.Handle((*handlers.Handler).DeviceLoanCreate))
			protected.GET("/device-loans/:id", adapter.Handle((*handlers.Handler).DeviceLoanDetail))
			protected.GET("/device-loans/:id/edit", adapter.Handle((*handlers.Handler).DeviceLoanEditPage))
			protected.POST("/device-loans/:id/edit", adapter.Handle((*handlers.Handler).DeviceLoanEdit))
			protected.POST("/device-loans/:id/delete", adapter.Handle((*handlers.Handler).DeviceLoanDelete))
			protected.POST("/device-loans/batch-delete", adapter.Handle((*handlers.Handler).DeviceLoanBatchDelete))

			protected.GET("/device-usages", func(c *gin.Context) { middleware.LabRedirect(c, http.StatusFound, "/devices?tab=usages") })
			protected.GET("/device-usages/create", adapter.Handle((*handlers.Handler).DeviceUsageCreatePage))
			protected.POST("/device-usages/create", adapter.Handle((*handlers.Handler).DeviceUsageCreate))
			protected.GET("/device-usages/:id", adapter.Handle((*handlers.Handler).DeviceUsageDetail))
			protected.GET("/device-usages/:id/edit", adapter.Handle((*handlers.Handler).DeviceUsageEditPage))
			protected.POST("/device-usages/:id/edit", adapter.Handle((*handlers.Handler).DeviceUsageEdit))
			protected.POST("/device-usages/:id/delete", adapter.Handle((*handlers.Handler).DeviceUsageDelete))
			protected.POST("/device-usages/batch-delete", adapter.Handle((*handlers.Handler).DeviceUsageBatchDelete))
			protected.POST("/device-loans/:id/extend", adapter.Handle((*handlers.Handler).DeviceLoanExtend))
			protected.GET("/installations", func(c *gin.Context) { middleware.LabRedirect(c, http.StatusFound, "/devices?tab=installations") })
			protected.GET("/installations/create", adapter.Handle((*handlers.Handler).DeviceInstallationCreatePage))
			protected.GET("/installations/:id", adapter.Handle((*handlers.Handler).DeviceInstallationDetail))
			protected.POST("/installations/create", adapter.Handle((*handlers.Handler).DeviceInstallationCreate))
			protected.GET("/installations/:id/edit", adapter.Handle((*handlers.Handler).DeviceInstallationEditPage))
			protected.POST("/installations/:id/edit", adapter.Handle((*handlers.Handler).DeviceInstallationEdit))
			protected.POST("/installations/:id/delete", adapter.Handle((*handlers.Handler).DeviceInstallationDelete))
			protected.POST("/installations/batch-delete", adapter.Handle((*handlers.Handler).DeviceInstallationBatchDelete))
			protected.GET("/schedules", adapter.Handle((*handlers.Handler).ScheduleList))
			protected.GET("/schedules/create", adapter.Handle((*handlers.Handler).ScheduleCreatePage))
			protected.POST("/schedules/create", adapter.Handle((*handlers.Handler).ScheduleCreate))
			protected.GET("/schedules/:id/edit", adapter.Handle((*handlers.Handler).ScheduleEditPage))
			protected.POST("/schedules/:id/edit", adapter.Handle((*handlers.Handler).ScheduleEdit))
			protected.POST("/schedules/:id/delete", adapter.Handle((*handlers.Handler).ScheduleDelete))
			protected.POST("/schedules/batch-delete", adapter.Handle((*handlers.Handler).ScheduleBatchDelete))

			protected.GET("/software", adapter.Handle((*handlers.Handler).SoftwareList))
			protected.POST("/software/create", adapter.Handle((*handlers.Handler).SoftwareCreate))
			protected.GET("/software/export", adapter.Handle((*handlers.Handler).SoftwareExport))
			protected.GET("/software/catalog.json", adapter.Handle((*handlers.Handler).GetSoftwareCatalogJSON))
			protected.GET("/software/:slug", adapter.Handle((*handlers.Handler).SoftwareDetail))
			protected.GET("/software/:slug/edit", adapter.Handle((*handlers.Handler).SoftwareEditPage))
			protected.POST("/software/:slug/edit", adapter.Handle((*handlers.Handler).SoftwareEdit))
			protected.POST("/software/:slug/delete", adapter.Handle((*handlers.Handler).SoftwareDelete))
			protected.POST("/software/batch-delete", adapter.Handle((*handlers.Handler).SoftwareBatchDelete))

			protected.GET("/logbook", adapter.Handle((*handlers.Handler).LogbookList))
			protected.GET("/logbook/upload", adapter.Handle((*handlers.Handler).LogbookUploadPage))
			protected.POST("/logbook/upload", adapter.Handle((*handlers.Handler).LogbookUpload))
			protected.POST("/logbook/save", adapter.Handle((*handlers.Handler).LogbookSave))
			protected.GET("/logbook/export", adapter.Handle((*handlers.Handler).LogbookExport))
			protected.GET("/logbook/export-preview", adapter.Handle((*handlers.Handler).LogbookExportPreview))
			protected.GET("/logbook/create", adapter.Handle((*handlers.Handler).LogbookCreatePage))
			protected.POST("/logbook/create", adapter.Handle((*handlers.Handler).LogbookCreate))
			protected.GET("/logbook/:id", adapter.Handle((*handlers.Handler).LogbookDetail))
			protected.GET("/logbook/:id/edit", adapter.Handle((*handlers.Handler).LogbookEditPage))
			protected.POST("/logbook/:id/edit", adapter.Handle((*handlers.Handler).LogbookEdit))
			protected.POST("/logbook/:id/delete", adapter.Handle((*handlers.Handler).LogbookDelete))
			protected.POST("/logbook/batch-delete", adapter.Handle((*handlers.Handler).LogbookBatchDelete))

			admin := protected.Group("/admin")
			admin.Use(middleware.AdminRequired())
			{
				admin.GET("/users", adapter.Handle((*handlers.Handler).UserList))
				admin.GET("/users/create", adapter.Handle((*handlers.Handler).UserCreatePage))
				admin.POST("/users/create", adapter.Handle((*handlers.Handler).UserCreate))
				admin.GET("/users/:username", adapter.Handle((*handlers.Handler).UserDetail))
				admin.GET("/users/:username/edit", adapter.Handle((*handlers.Handler).UserEditPage))
				admin.POST("/users/:username/edit", adapter.Handle((*handlers.Handler).UserEdit))
				admin.POST("/users/:username/delete", adapter.Handle((*handlers.Handler).UserDelete))
				admin.POST("/users/batch-delete", adapter.Handle((*handlers.Handler).UserBatchDelete))
				admin.GET("/activity-logs", adapter.Handle((*handlers.Handler).ActivityLogList))
				admin.GET("/activity-logs/export", adapter.Handle((*handlers.Handler).ActivityLogExport))
			}

			protected.GET("/print", adapter.Handle((*handlers.Handler).PrintForm))
			protected.GET("/print/generate", adapter.Handle((*handlers.Handler).PrintGeneratePDF))

			protected.GET("/profile", adapter.Handle((*handlers.Handler).Profile))
			protected.POST("/profile", adapter.Handle((*handlers.Handler).UpdateProfile))
			protected.POST("/profile/password", adapter.Handle((*handlers.Handler).ChangePassword))
		}

		api := labGroup.Group("/api")
		api.Use(middleware.AuthRequired(), middleware.CSRF(), writeFlushMiddleware())
		{
			api.GET("/pc/status", adapter.Handle((*handlers.Handler).PCStatusAPI))
			api.POST("/pc/:label/status", adapter.Handle((*handlers.Handler).UpdatePCStatusAPI))
			api.GET("/pc/layout", adapter.Handle((*handlers.Handler).PCGetLayout))
			api.POST("/pc/swap", adapter.Handle((*handlers.Handler).PCSwap))
			api.POST("/pc/replace", adapter.Handle((*handlers.Handler).PCReplace))
			api.POST("/pc/move-row", adapter.Handle((*handlers.Handler).PCMoveRowToCadangan))
			api.POST("/pc/move-to-cadangan", adapter.Handle((*handlers.Handler).PCMoveToCadangan))
			api.POST("/pc/move", adapter.Handle((*handlers.Handler).PCMove))
			api.POST("/pc/place", adapter.Handle((*handlers.Handler).PCPlace))
			api.POST("/upload-image", adapter.Handle((*handlers.Handler).UploadImage))
			api.POST("/delete-temp-file", adapter.Handle((*handlers.Handler).DeleteTempFile))
			api.POST("/cleanup-temp-files", adapter.Handle((*handlers.Handler).CleanupTempFiles))
			api.GET("/devices/next-asset-code", adapter.Handle((*handlers.Handler).GetNextAssetCode))
			api.GET("/devices/next-asset-codes", adapter.Handle((*handlers.Handler).GetNextAssetCodes))
			api.GET("/sticker-templates", adapter.Handle((*handlers.Handler).StickerTemplateList))
			api.POST("/sticker-templates", adapter.Handle((*handlers.Handler).StickerTemplateCreate))
			api.PUT("/sticker-templates/:id", adapter.Handle((*handlers.Handler).StickerTemplateUpdate))
			api.DELETE("/sticker-templates/:id", adapter.Handle((*handlers.Handler).StickerTemplateDelete))
		}
	}

	closeAll := func() {
		for _, h := range handlersMap {
			h.Close()
		}
	}
	flushAll := func() {
		for _, h := range handlersMap {
			h.FlushActivityLogs()
		}
	}
	return router, closeAll, flushAll
}
