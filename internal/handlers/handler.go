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

// Handler holds dependencies for all handlers
type Handler struct {
	cfg                *config.Config
	activityLogService *services.ActivityLogService
	imageService       *services.ImageService

	authService        *services.AuthService
	userService        *services.UserService
	deviceService      *services.DeviceService
	pcService          *services.PCService
	deviceLoanService  *services.DeviceLoanService
	deviceUsageService *services.DeviceUsageService
	logbookService     *services.LogbookService
	dashboardService   *services.DashboardService
	scheduleService    *services.ScheduleService
	softwareService    *services.SoftwareService
	deviceTypeService  *services.DeviceTypeService
}

func NewHandler(db *database.DB, cfg *config.Config) *Handler {
	activityLogService := services.NewActivityLogService(db)
	deviceRepo := repository.NewDeviceRepository(db)
	deviceTypeRepo := repository.NewDeviceTypeRepository(db)
	deviceLoanRepo := repository.NewDeviceLoanRepository(db)
	deviceUsageRepo := repository.NewDeviceUsageRepository(db)
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

		authService:        services.NewAuthService(userRepo, activityLogService),
		userService:        services.NewUserService(userRepo, activityLogService),
		deviceService:      services.NewDeviceService(deviceRepo, deviceTypeRepo, activityLogService),
		pcService:          services.NewPCService(pcRepo, activityLogService),
		deviceLoanService:  services.NewDeviceLoanService(db, deviceLoanRepo, deviceRepo, activityLogService),
		deviceUsageService: services.NewDeviceUsageService(db, deviceUsageRepo, deviceRepo, activityLogService),
		logbookService:     services.NewLogbookService(logbookRepo, activityLogService),
		dashboardService:   services.NewDashboardService(dashboardRepo),
		scheduleService:    services.NewScheduleService(scheduleRepo, activityLogService),
		softwareService:    services.NewSoftwareService(softwareRepo, activityLogService),
		deviceTypeService:  services.NewDeviceTypeService(deviceTypeRepo, activityLogService),
	}
}

func getRequestContext(c *gin.Context) (ipAddress, userAgent string) {
	ipAddress = c.ClientIP()
	userAgent = c.Request.UserAgent()
	return
}

// canAccessProfile checks cross-profile access rules
// Primary accounts (admin, rekan) cannot access each other
// Non-primary accounts can only access their own profile
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

// user gets current user info and redirects to login if not authenticated
func (h *Handler) user(c *gin.Context) (userID int, username, role string, ok bool) {
	userID, username, role, ok = middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
	}
	return
}

// errJSON sends a JSON error response
func (h *Handler) errJSON(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
}

// errHTML renders error.html with the given message
func (h *Handler) errHTML(c *gin.Context, msg string) {
	_, username, role, _ := h.user(c)
	c.HTML(http.StatusInternalServerError, "error.html", gin.H{
		"title": "Error", "message": msg,
		"currentPage": "", "username": username, "role": role,
	})
}

// redirectWithError redirects with ?error= query parameter
func (h *Handler) redirectWithError(c *gin.Context, url, msg string) {
	c.Redirect(http.StatusFound, url+"?error="+msg)
}


