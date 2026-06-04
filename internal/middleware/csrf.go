package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		tokenRaw := session.Get("csrf_token")
		token, ok := tokenRaw.(string)
		if !ok || token == "" {
			b := make([]byte, 32)
			if _, err := rand.Read(b); err == nil {
				token = hex.EncodeToString(b)
				session.Set("csrf_token", token)
				session.Save()
			}
		}

		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		clientToken := c.PostForm("_csrf")
		if clientToken == "" {
			clientToken = c.GetHeader("X-CSRF-Token")
		}

		if clientToken == "" || clientToken != token {
			if strings.Contains(c.GetHeader("Accept"), "application/json") {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "CSRF token mismatch"})
			} else {
				c.AbortWithStatus(http.StatusForbidden)
			}
			return
		}

		c.Next()
	}
}
