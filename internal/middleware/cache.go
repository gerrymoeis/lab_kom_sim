package middleware

import (
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var versionedFilePattern = regexp.MustCompile(`\.([a-f0-9]{8})\.(css|js)$`)

func CacheControl() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		switch {
		case strings.HasPrefix(path, "/api/"):
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")

	case strings.HasPrefix(path, "/uploads/"):
		after := strings.TrimPrefix(path, "/uploads/")
		if slashIdx := strings.Index(after, "/"); slashIdx >= 0 && strings.HasPrefix(after[slashIdx:], "/temp/") {
			c.Header("Cache-Control", "no-cache, no-store")
		} else {
			c.Header("Cache-Control", "public, max-age=86400")
		}

	case strings.HasPrefix(path, "/static/"):
			if versionedFilePattern.MatchString(path) {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				c.Header("Cache-Control", "public, max-age=86400")
			}

		default:
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}

		c.Next()
	}
}
