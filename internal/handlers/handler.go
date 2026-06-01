package handlers

import (
	"net/http"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/repository"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	cfg                *config.Config
	activityLogService *services.ActivityLogService
	imageService       *services.ImageService

	authService              *services.AuthService
	userService              *services.UserService
	deviceService            *services.DeviceService
	pcService                *services.PCService
	deviceLoanService        *services.DeviceLoanService
	deviceUsageService       *services.DeviceUsageService
	deviceInstallationService *services.DeviceInstallationService
	logbookService           *services.LogbookService
	dashboardService         *services.DashboardService
	scheduleService          *services.ScheduleService
	softwareService          *services.SoftwareService
	deviceTypeService        *services.DeviceTypeService
	categoryService          *services.CategoryService
}

func NewHandler(db *database.DB, cfg *config.Config, notifier services.CUDNotifier) *Handler {
	activityLogService := services.NewActivityLogService(db, notifier)
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

	return &Handler{
		cfg:                cfg,
		activityLogService: activityLogService,
		imageService:       services.NewImageService(),

		authService:              services.NewAuthService(userRepo, activityLogService),
		userService:              services.NewUserService(userRepo, activityLogService),
		deviceService:            services.NewDeviceService(deviceRepo, deviceTypeRepo, activityLogService),
		pcService:                services.NewPCService(pcRepo, activityLogService),
		deviceLoanService:        services.NewDeviceLoanService(deviceLoanRepo, loanExtensionRepo, activityLogService),
		deviceUsageService:       services.NewDeviceUsageService(deviceUsageRepo, activityLogService),
		deviceInstallationService: services.NewDeviceInstallationService(deviceInstallationRepo, activityLogService),
		logbookService:           services.NewLogbookService(logbookRepo, activityLogService),
		dashboardService:         services.NewDashboardService(dashboardRepo),
		scheduleService:          services.NewScheduleService(scheduleRepo, activityLogService),
		softwareService:          services.NewSoftwareService(softwareRepo, activityLogService),
		deviceTypeService:        services.NewDeviceTypeService(deviceTypeRepo, activityLogService),
		categoryService:          services.NewCategoryService(categoryRepo, activityLogService),
	}
}

func getRequestContext(c *gin.Context) (ipAddress, userAgent string) {
	ipAddress = c.ClientIP()
	userAgent = c.Request.UserAgent()
	return
}

func (h *Handler) canAccessProfile(actorUsername, targetUsername string) bool {
	isActorPrimary := actorUsername == "admin" || actorUsername == "rekan"
	isTargetPrimary := targetUsername == "admin" || targetUsername == "rekan"

	if isActorPrimary && isTargetPrimary && actorUsername != targetUsername {
		return false
	}
	if !isActorPrimary && actorUsername != targetUsername {
		return false
	}
	return true
}

func (h *Handler) user(c *gin.Context) (userID int, username, role string, ok bool) {
	userID, username, role, ok = middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
	}
	return
}

func (h *Handler) Close() {
	h.activityLogService.Close()
}

func (h *Handler) errJSON(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
}

func (h *Handler) errHTML(c *gin.Context, msg string) {
	_, username, role, _ := h.user(c)
	c.HTML(http.StatusInternalServerError, "error.html", gin.H{
		"title": "Error", "message": msg,
		"currentPage": "", "username": username, "role": role,
	})
}

func (h *Handler) redirectWithError(c *gin.Context, url, msg string) {
	c.Redirect(http.StatusFound, url+"?error="+msg)
}
