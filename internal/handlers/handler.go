package handlers

import (
	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

// Handler holds dependencies for all handlers
type Handler struct {
	db                 *database.DB
	cfg                *config.Config
	activityLogService *services.ActivityLogService
	imageService       *services.ImageService
}

// NewHandler creates a new handler instance
func NewHandler(db *database.DB, cfg *config.Config) *Handler {
	return &Handler{
		db:                 db,
		cfg:                cfg,
		activityLogService: services.NewActivityLogService(db),
		imageService:       services.NewImageService(),
	}
}

// getRequestContext extracts IP address and User-Agent from request
func getRequestContext(c *gin.Context) (ipAddress, userAgent string) {
	ipAddress = c.ClientIP()
	userAgent = c.Request.UserAgent()
	return
}
