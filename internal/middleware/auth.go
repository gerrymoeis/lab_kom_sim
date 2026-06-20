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
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _, isSuperAdmin, ok := GetCurrentUser(c)
		if !ok {
			c.AbortWithStatus(http.StatusUnauthorized)
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
			c.HTML(http.StatusForbidden, "error.html", gin.H{
				"title": "Akses Ditolak", "currentPage": "",
				"message": "Anda tidak memiliki akses ke halaman ini.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// SuperAdminRequired checks if user is a global super admin
func SuperAdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, _, isSuperAdmin, ok := GetCurrentUser(c)
		if !ok || !isSuperAdmin {
			c.HTML(http.StatusForbidden, "error.html", gin.H{
				"title": "Akses Ditolak", "currentPage": "",
				"message": "Hanya super admin yang dapat mengakses halaman ini.",
			})
			c.Abort()
			return
		}
		c.Set("is_super_admin", true)
		c.Next()
	}
}

// GetCurrentUser gets current user info from global session
func GetCurrentUser(c *gin.Context) (userID int, username string, isSuperAdmin bool, ok bool) {
	session := sessions.Default(c)

	userIDVal := session.Get("user_id")
	usernameVal := session.Get("username")

	if userIDVal == nil || usernameVal == nil {
		return 0, "", false, false
	}

	isSuperAdmin, _ = session.Get("is_super_admin").(bool)
	return userIDVal.(int), usernameVal.(string), isSuperAdmin, true
}
