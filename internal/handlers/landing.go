package handlers

import (
	"net/http"

	"inventaris-lab-kom/internal/config"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func LandingPage(cfg *config.Config) gin.HandlerFunc {
	store := cookie.NewStore([]byte(cfg.SessionSecret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})

	return func(c *gin.Context) {
		s, _ := store.Get(c.Request, "inventaris_session")
		if s != nil && s.Values["user_id"] != nil && len(cfg.Labs) > 0 {
			c.Redirect(http.StatusFound, "/"+cfg.Labs[0].URLPath+"/dashboard")
			return
		}
		c.HTML(http.StatusOK, "landing.html", gin.H{
			"title": "Sistem Inventaris Laboratorium Komputer",
			"labs":  cfg.Labs,
		})
	}
}
