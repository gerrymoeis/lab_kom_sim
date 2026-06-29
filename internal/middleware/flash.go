package middleware

import (
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// FlashReader membaca session flash messages dan menyimpannya di context
// untuk di-injeksi ke template data oleh render functions.
// Harus dipasang SETELAH GlobalSessionMiddleware.
func FlashReader() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		hasFlash := false

		if errs := session.Flashes("error"); len(errs) > 0 {
			msgs := make([]string, len(errs))
			for i, v := range errs {
				msgs[i] = v.(string)
			}
			c.Set("_flash_error", strings.Join(msgs, ", "))
			hasFlash = true
		}
		if succs := session.Flashes("success"); len(succs) > 0 {
			msgs := make([]string, len(succs))
			for i, v := range succs {
				msgs[i] = v.(string)
			}
			c.Set("_flash_success", strings.Join(msgs, ", "))
			hasFlash = true
		}
		if hasFlash {
			session.Save()
		}
		c.Next()
	}
}
