package server

import (
	"inventaris-lab-kom/internal/handlers"

	"github.com/gin-gonic/gin"
)

type HandlerAdapter struct {
	handlers map[string]*handlers.Handler
}

func NewHandlerAdapter(h map[string]*handlers.Handler) *HandlerAdapter {
	return &HandlerAdapter{handlers: h}
}

func (a *HandlerAdapter) Register(lab string, h *handlers.Handler) {
	a.handlers[lab] = h
}

func (a *HandlerAdapter) Handle(fn func(*handlers.Handler, *gin.Context)) gin.HandlerFunc {
	return func(c *gin.Context) {
		lab := c.GetString("lab")
		if h, ok := a.handlers[lab]; ok {
			fn(h, c)
		} else {
			c.AbortWithStatus(404)
		}
	}
}
