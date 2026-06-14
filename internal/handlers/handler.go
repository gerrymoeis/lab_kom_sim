package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	cfg                *config.Config
	activityLogService *services.ActivityLogService
	imageService       *services.ImageService

	authService               *services.AuthService
	userService               *services.UserService
	deviceService             *services.DeviceService
	pcService                 *services.PCService
	deviceLoanService         *services.DeviceLoanService
	deviceUsageService        *services.DeviceUsageService
	deviceInstallationService *services.DeviceInstallationService
	logbookService            *services.LogbookService
	dashboardService          *services.DashboardService
	scheduleService           *services.ScheduleService
	softwareService           *services.SoftwareService
	deviceTypeService         *services.DeviceTypeService
	categoryService           *services.CategoryService
	printService              *services.PrintService
}

func NewHandler(db *database.DB, cfg *config.Config, notifier services.CUDNotifier) *Handler {
	activityLogService := services.NewActivityLogService(db, notifier, cfg.LogRetentionDays, cfg.LogCleanupInterval)
	deviceRepo := repository.NewDeviceRepository(db)
	deviceTypeRepo := repository.NewDeviceTypeRepository(db)
	deviceLoanRepo := repository.NewDeviceLoanRepository(db)
	deviceUsageRepo := repository.NewDeviceUsageRepository(db)
	deviceInstallationRepo := repository.NewDeviceInstallationRepository(db)
	loanExtensionRepo := repository.NewLoanExtensionRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	pcRepo := repository.NewPCRepository(db)
	softwareRepo := repository.NewSoftwareRepository(db)
	logbookRepo := repository.NewLogbookRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)
	userRepo := repository.NewUserRepository(db)
	dashboardRepo := repository.NewDashboardRepository(db)
	stickerTemplateRepo := repository.NewStickerTemplateRepository(db)

	return &Handler{
		cfg:                cfg,
		activityLogService: activityLogService,
		imageService:       services.NewImageService(),

		authService:               services.NewAuthService(userRepo, activityLogService),
		userService:               services.NewUserService(userRepo, activityLogService),
		deviceService:             services.NewDeviceService(deviceRepo, deviceTypeRepo, activityLogService),
		pcService:                 services.NewPCService(pcRepo, activityLogService),
		deviceLoanService:         services.NewDeviceLoanService(deviceLoanRepo, loanExtensionRepo, activityLogService),
		deviceUsageService:        services.NewDeviceUsageService(deviceUsageRepo, activityLogService),
		deviceInstallationService: services.NewDeviceInstallationService(deviceInstallationRepo, activityLogService),
		logbookService:            services.NewLogbookService(logbookRepo, activityLogService),
		dashboardService:          services.NewDashboardService(dashboardRepo),
		scheduleService:           services.NewScheduleService(scheduleRepo, activityLogService),
		softwareService:           services.NewSoftwareService(softwareRepo, activityLogService),
		deviceTypeService:         services.NewDeviceTypeService(deviceTypeRepo, activityLogService),
		categoryService:           services.NewCategoryService(categoryRepo, activityLogService),
		printService:              services.NewPrintService(pcRepo, deviceRepo, stickerTemplateRepo),
	}
}

func getRequestContext(c *gin.Context) (ipAddress, userAgent string) {
	ipAddress = c.ClientIP()
	userAgent = c.Request.UserAgent()
	return
}

func (h *Handler) canAccessProfile(actorUsername string, target *models.User, actorIsSuperAdmin bool) bool {
	if target.IsProtected {
		return actorUsername == target.Username
	}
	return actorUsername == target.Username || actorIsSuperAdmin
}

func CanAccessProfile(actorUsername string, target models.User, actorIsSuperAdmin bool) bool {
	if target.IsProtected {
		return actorUsername == target.Username
	}
	return actorUsername == target.Username || actorIsSuperAdmin
}

func (h *Handler) user(c *gin.Context) (userID int, username, role string, ok bool) {
	userID, username, role, _, ok = middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
	}
	return
}

func (h *Handler) isSuperAdmin(c *gin.Context) bool {
	_, _, _, val, ok := middleware.GetCurrentUser(c)
	return ok && val
}

func (h *Handler) Close() {
	h.activityLogService.Close()
}

func (h *Handler) FlushActivityLogs() {
	h.activityLogService.Flush()
}

func (h *Handler) errJSON(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
}

func (h *Handler) renderTemplate(c *gin.Context, status int, tmpl string, data gin.H) {
	if token := sessions.Default(c).Get("csrf_token"); token != nil {
		data["csrf_token"] = token.(string)
	}
	c.HTML(status, tmpl, data)
}

func (h *Handler) errHTML(c *gin.Context, msg string) {
	_, username, role, _ := h.user(c)
	h.renderTemplate(c, http.StatusInternalServerError, "error.html", gin.H{
		"title": "Error", "message": msg,
		"currentPage": "", "username": username, "role": role,
	})
}

func (h *Handler) redirectWithSuccess(c *gin.Context, rawURL, msg string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		c.Redirect(http.StatusFound, "/")
		return
	}
	q := u.Query()
	q.Set("success", msg)
	u.RawQuery = q.Encode()
	c.Redirect(http.StatusFound, u.String())
}

func (h *Handler) redirectWithError(c *gin.Context, rawURL, msg string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		c.Redirect(http.StatusFound, "/")
		return
	}
	q := u.Query()
	q.Set("error", msg)
	u.RawQuery = q.Encode()
	c.Redirect(http.StatusFound, u.String())
}

func parseInt64IDs(ids []string) ([]int, error) {
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		n, err := strconv.Atoi(id)
		if err != nil {
			return nil, fmt.Errorf("ID tidak valid: %s", id)
		}
		result = append(result, n)
	}
	return result, nil
}
