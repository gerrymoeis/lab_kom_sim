package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func NewCSRFToken(session sessions.Session) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	token := hex.EncodeToString(b)
	session.Set("csrf_token", token)
	return token
}

func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		tokenRaw := session.Get("csrf_token")
		token, ok := tokenRaw.(string)
		if !ok || token == "" {
			token = NewCSRFToken(session)
			if token == "" {
				c.Next()
				return
			}
			// Error tidak menyebabkan abort — token sudah ada in-memory untuk request ini.
			// LoginPage handler bertanggung jawab untuk eksplisit save pada GET /login.
			_ = session.Save()
		}

		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		clientToken := c.PostForm("_csrf")
		if clientToken == "" {
			clientToken = c.GetHeader("X-CSRF-Token")
		}

		if clientToken == "" || subtle.ConstantTimeCompare([]byte(clientToken), []byte(token)) != 1 {
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
