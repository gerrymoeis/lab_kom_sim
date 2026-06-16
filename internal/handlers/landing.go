package handlers

import (
	"net/http"

	"inventaris-lab-kom/internal/config"

	"github.com/gin-gonic/gin"
)

func LandingPage(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, lab := range cfg.Labs {
			if _, err := c.Request.Cookie("inventaris_session_" + lab.Name); err == nil {
				c.Redirect(http.StatusFound, "/"+lab.Name+"/dashboard")
				return
			}
		}
		c.HTML(http.StatusOK, "landing.html", gin.H{
			"title": "Sistem Inventaris Laboratorium Komputer",
			"labs":  cfg.Labs,
		})
	}
}
