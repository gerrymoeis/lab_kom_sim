package handlers

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func (h *Handler) Home(c *gin.Context) {
	session := sessions.Default(c)
	if session.Get("user_id") != nil {
		h.redirect(c, "/dashboard")
		return
	}
	h.redirect(c, "/login")
}
