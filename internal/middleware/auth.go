package middleware

import (
	"crypto/subtle"
	"database/sql"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// AuthRequired middleware checks if user is authenticated and session is valid
// db is used to validate session_token for single-session enforcement
func AuthRequired(db any) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		sessionToken := session.Get("session_token")

		if userID == nil || sessionToken == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		queryDB, ok := db.(interface {
			QueryRow(query string, args ...any) *sql.Row
		})
		if ok {
			var dbToken string
			err := queryDB.QueryRow(`SELECT session_token FROM users WHERE id = ?`, userID.(int)).Scan(&dbToken)
			if err != nil || dbToken == "" || subtle.ConstantTimeCompare([]byte(dbToken), []byte(sessionToken.(string))) != 1 {
				session.Clear()
				session.Options(sessions.Options{
					Path:     "/",
					MaxAge:   -1,
					HttpOnly: true,
					Secure:   c.Request.TLS != nil,
					SameSite: http.SameSiteLaxMode,
				})
				if err := session.Save(); err != nil {
					http.SetCookie(c.Writer, &http.Cookie{
						Name:     "inventaris_session",
						Value:    "",
						Path:     "/",
						HttpOnly: true,
						Secure:   c.Request.TLS != nil,
						SameSite: http.SameSiteLaxMode,
						MaxAge:   -1,
					})
				}
				c.Redirect(http.StatusFound, "/login")
				c.Abort()
				return
			}
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
func GetCurrentUser(c *gin.Context) (userID int, username string, role string, isSuperAdmin bool, ok bool) {
	session := sessions.Default(c)
	
	userIDVal := session.Get("user_id")
	usernameVal := session.Get("username")
	roleVal := session.Get("role")

	if userIDVal == nil || usernameVal == nil || roleVal == nil {
		return 0, "", "", false, false
	}

	isSuperAdmin, _ = session.Get("is_super_admin").(bool)
	return userIDVal.(int), usernameVal.(string), roleVal.(string), isSuperAdmin, true
}
