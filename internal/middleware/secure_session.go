package middleware

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func IsHTTPS(c *gin.Context) bool {
	return c.Request.TLS != nil ||
		c.Request.Header.Get("X-Forwarded-Proto") == "https"
}

func SecureSessionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		if session == nil {
			c.Next()
			return
		}

		session.Options(sessions.Options{
			Path:     "/",
			MaxAge:   86400 * 7,
			HttpOnly: true,
			Secure:   IsHTTPS(c),
			SameSite: http.SameSiteLaxMode,
		})

		c.Next()
	}
}
