package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/middleware"
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

func NewHandler(db *database.DB, cfg *config.Config) *Handler {
	return &Handler{
		db:                 db,
		cfg:                cfg,
		activityLogService: services.NewActivityLogService(db),
		imageService:       services.NewImageService(),
	}
}

func getRequestContext(c *gin.Context) (ipAddress, userAgent string) {
	ipAddress = c.ClientIP()
	userAgent = c.Request.UserAgent()
	return
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
	c.HTML(http.StatusInternalServerError, "error.html", gin.H{
		"title": "Error", "message": msg,
	})
}

// redirectWithError redirects with ?error= query parameter
func (h *Handler) redirectWithError(c *gin.Context, url, msg string) {
	c.Redirect(http.StatusFound, url+"?error="+msg)
}

// logActivity logs any activity with request context — unified entry point
func (h *Handler) logActivity(c *gin.Context, action, entityType string, entityID int, oldVals, newVals map[string]interface{}, errMsg string) {
	if id, u, r, ok := h.user(c); ok {
		ip, ua := getRequestContext(c)
		h.activityLogService.LogAction(id, u, r, action, entityType, entityID, oldVals, newVals, ip, ua, errMsg)
	}
}

// convenience wrappers — kept for call-site readability
func (h *Handler) logCreate(c *gin.Context, entityType string, entityID int, vals map[string]interface{}) { h.logActivity(c, "create", entityType, entityID, nil, vals, "") }
func (h *Handler) logUpdate(c *gin.Context, entityType string, entityID int, oldVals, newVals map[string]interface{}) { h.logActivity(c, "update", entityType, entityID, oldVals, newVals, "") }
func (h *Handler) logDelete(c *gin.Context, entityType string, entityID int, oldVals map[string]interface{}) { h.logActivity(c, "delete", entityType, entityID, oldVals, nil, "") }
func (h *Handler) logCreateError(c *gin.Context, entityType string, vals map[string]interface{}, errMsg string) { h.logActivity(c, "create", entityType, 0, nil, vals, errMsg) }
func (h *Handler) logUpdateError(c *gin.Context, entityType string, id int, oldVals map[string]interface{}, errMsg string) { h.logActivity(c, "update", entityType, id, oldVals, nil, errMsg) }
func (h *Handler) logDeleteError(c *gin.Context, entityType string, id int, oldVals map[string]interface{}, errMsg string) { h.logActivity(c, "delete", entityType, id, oldVals, nil, errMsg) }

// valStr returns the string value from a NullString, or "" if invalid
func valStr(ns sql.NullString) string {
	if ns.Valid { return ns.String }
	return ""
}

// valTimePtr returns a *time.Time from a NullTime, or nil if invalid
func valTimePtr(nt sql.NullTime) *time.Time {
	if nt.Valid { return &nt.Time }
	return nil
}

