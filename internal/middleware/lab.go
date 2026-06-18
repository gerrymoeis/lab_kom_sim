package middleware

import (
	"strings"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"

	"github.com/gin-gonic/gin"
)

func DBInjector(dbs map[string]*database.DB, labs map[string]config.LabConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		lab := c.Param("lab")
		if lab == "" {
			lab = c.GetString("lab")
		}
		if _, ok := dbs[lab]; !ok {
			c.AbortWithStatus(404)
			return
		}
		c.Set("db", dbs[lab])
		if lc, ok := labs[lab]; ok {
			c.Set("labConfig", lc)
		}
		c.Set("lab", lab)
		c.Next()
	}
}

func LabURL(c *gin.Context, path string) string {
	lab := c.GetString("lab")
	if lab == "" {
		return path
	}
	if path == "" {
		return "/" + lab
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "/" + lab + path
}

func LabRedirect(c *gin.Context, code int, path string) {
	c.Redirect(code, LabURL(c, path))
}

func LabCookieName(lab string) string {
	if lab == "" {
		return "inventaris_session"
	}
	return "inventaris_session_" + lab
}
