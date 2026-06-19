package middleware

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func GlobalSessionMiddleware(secret string, secure bool) gin.HandlerFunc {
	store := cookie.NewStore([]byte(secret))

	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	return sessions.Sessions("inventaris_session", store)
}
