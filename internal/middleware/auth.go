package middleware

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// AuthRequired middleware checks if user is authenticated
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")

		if userID == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Next()
	}
}

// AdminRequired middleware checks if user is admin
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		role := session.Get("role")

		if role != "admin" {
			c.HTML(http.StatusForbidden, "error.html", gin.H{
				"title":   "Akses Ditolak",
				"message": "Anda tidak memiliki akses ke halaman ini. Hanya admin yang diizinkan.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetCurrentUser gets current user info from session
func GetCurrentUser(c *gin.Context) (userID int, username string, role string, ok bool) {
	session := sessions.Default(c)
	
	userIDVal := session.Get("user_id")
	usernameVal := session.Get("username")
	roleVal := session.Get("role")

	if userIDVal == nil || usernameVal == nil || roleVal == nil {
		return 0, "", "", false
	}

	return userIDVal.(int), usernameVal.(string), roleVal.(string), true
}
