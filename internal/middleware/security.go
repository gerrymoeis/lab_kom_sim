package middleware

import (
	"github.com/gin-gonic/gin"
)

func SecurityHeaders(environment string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")

		if environment == "production" {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"script-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; "+
				"frame-ancestors 'none'")

		c.Next()
	}
}
