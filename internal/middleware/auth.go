package middleware

import (
	"crypto/subtle"
	"net/http"

	"inventaris-lab-kom/internal/database"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		sessionToken := session.Get("session_token")

		if userID == nil || sessionToken == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		gdbVal, exists := c.Get("globalDB")
		if !exists {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		globalDB := gdbVal.(*database.DB)

		var dbToken string
		err := globalDB.QueryRow(
			`SELECT session_token FROM global_users WHERE id = ?`, userID.(int)).Scan(&dbToken)
		if err != nil || dbToken == "" ||
			subtle.ConstantTimeCompare([]byte(dbToken), []byte(sessionToken.(string))) != 1 {

			session.Clear()
			session.Options(sessions.Options{
				Path:     "/",
				MaxAge:   -1,
				HttpOnly: true,
				Secure:   c.Request.TLS != nil,
				SameSite: http.SameSiteLaxMode,
			})
			_ = session.Save()
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Next()
	}
}

// AdminRequired checks if user has admin role for current lab (or is super admin)
// Redirects unauthorized users to lab dashboard instead of showing 403.
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _, isSuperAdmin, _, ok := GetCurrentUser(c)
		if !ok {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		if isSuperAdmin {
			c.Next()
			return
		}

		lab := c.GetString("lab")
		gdb := c.MustGet("globalDB").(*database.DB)

		var role string
		err := gdb.QueryRow(
			`SELECT role FROM lab_permissions WHERE user_id = ? AND lab_url_path = ?`,
			userID, lab).Scan(&role)
		if err != nil || role != "admin" {
			LabRedirect(c, http.StatusFound, "/dashboard")
			c.Abort()
			return
		}

		c.Next()
	}
}

// SuperAdminRequired checks if user is a global super admin or global admin biasa
// Redirects non-SA/non-GAB users to their appropriate dashboard instead of showing 403.
func SuperAdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _, isSuperAdmin, isGlobalAdmin, ok := GetCurrentUser(c)
		if !ok {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		if !isSuperAdmin && !isGlobalAdmin {
			labsRaw := sessions.Default(c).Get("labs")
			if labsRaw != nil {
				if labs, ok := labsRaw.([]string); ok && len(labs) > 0 {
					c.Redirect(http.StatusFound, "/"+labs[0]+"/dashboard")
					c.Abort()
					return
				}
			}
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Set("is_super_admin", isSuperAdmin)
		c.Set("is_global_admin", isGlobalAdmin)
		c.Next()
	}
}

// GetCurrentUser gets current user info from global session
func GetCurrentUser(c *gin.Context) (userID int, username string, isSuperAdmin bool, isGlobalAdmin bool, ok bool) {
	session := sessions.Default(c)

	userIDVal := session.Get("user_id")
	usernameVal := session.Get("username")

	if userIDVal == nil || usernameVal == nil {
		return 0, "", false, false, false
	}

	isSuperAdmin, _ = session.Get("is_super_admin").(bool)
	isGlobalAdmin, _ = session.Get("is_global_admin").(bool)
	return userIDVal.(int), usernameVal.(string), isSuperAdmin, isGlobalAdmin, true
}
