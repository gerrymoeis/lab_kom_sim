package handlers

import (
	"net/http"
	"strconv"
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

// getID parses a URL param as int, rendering error.html on failure
func (h *Handler) getID(c *gin.Context, param string) (int, bool) {
	id, err := parseIntParam(c, param)
	if err != nil {
		h.errHTML(c, "Invalid ID")
		return 0, false
	}
	return id, true
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

// logCreate logs a successful create activity with request context
func (h *Handler) logCreate(c *gin.Context, entityType string, entityID int, vals map[string]interface{}) {
	if id, u, r, ok := h.user(c); ok {
		ip, ua := getRequestContext(c)
		h.activityLogService.LogCreate(id, u, r, entityType, entityID, vals, ip, ua)
	}
}

// logUpdate logs a successful update activity with request context
func (h *Handler) logUpdate(c *gin.Context, entityType string, entityID int, oldVals, newVals map[string]interface{}) {
	if id, u, r, ok := h.user(c); ok {
		ip, ua := getRequestContext(c)
		h.activityLogService.LogUpdate(id, u, r, entityType, entityID, oldVals, newVals, ip, ua)
	}
}

// logDelete logs a successful delete activity with request context
func (h *Handler) logDelete(c *gin.Context, entityType string, entityID int, oldVals map[string]interface{}) {
	if id, u, r, ok := h.user(c); ok {
		ip, ua := getRequestContext(c)
		h.activityLogService.LogDelete(id, u, r, entityType, entityID, oldVals, ip, ua)
	}
}

// logCreateError logs a failed create activity with request context
func (h *Handler) logCreateError(c *gin.Context, entityType string, vals map[string]interface{}, errMsg string) {
	if id, u, r, ok := h.user(c); ok {
		ip, ua := getRequestContext(c)
		h.activityLogService.LogCreate(id, u, r, entityType, 0, vals, ip, ua, errMsg)
	}
}

// logUpdateError logs a failed update activity with request context
func (h *Handler) logUpdateError(c *gin.Context, entityType string, id int, oldVals map[string]interface{}, errMsg string) {
	if uid, u, r, ok := h.user(c); ok {
		ip, ua := getRequestContext(c)
		h.activityLogService.LogUpdate(uid, u, r, entityType, id, oldVals, map[string]interface{}{"error": errMsg}, ip, ua, errMsg)
	}
}

// logDeleteError logs a failed delete activity with request context
func (h *Handler) logDeleteError(c *gin.Context, entityType string, id int, oldVals map[string]interface{}, errMsg string) {
	if uid, u, r, ok := h.user(c); ok {
		ip, ua := getRequestContext(c)
		h.activityLogService.LogDelete(uid, u, r, entityType, id, oldVals, ip, ua, errMsg)
	}
}

// successJSON sends a success JSON response with optional message
func (h *Handler) successJSON(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, gin.H{"success": true, "message": msg})
}

// parseDate parses a date string in YYYY-MM-DD format
func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// parseIntParam parses an int from a gin URL param
func parseIntParam(c *gin.Context, param string) (int, error) {
	return strconv.Atoi(c.Param(param))
}

