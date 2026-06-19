package handlers

import (
	"errors"
	"net/http"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBind(&req); err != nil {
		h.renderTemplate(c, http.StatusBadRequest, "login.html", gin.H{
			"title": "Login - " + c.GetString("lab"),
			"error": "Username dan password harus diisi",
		})
		return
	}

	user, token, err := h.globalAuthService.Login(req.Username, req.Password)
	if err != nil {
		msg := "Username atau password salah"
		if errors.Is(err, services.ErrAlreadyLoggedIn) {
			msg = "Akun ini sudah login di perangkat lain. Silakan logout terlebih dahulu."
		}
		status := http.StatusUnauthorized
		if errors.Is(err, services.ErrAlreadyLoggedIn) {
			status = http.StatusConflict
		}
		h.renderTemplate(c, status, "login.html", gin.H{
			"title": "Login - " + c.GetString("lab"),
			"error": msg,
		})
		return
	}

	// Cache user's accessible labs in session
	labPaths := h.globalAuthService.GetLabsForUser(user.ID, user.IsSuperAdmin, h.cfg.Labs)

	session := sessions.Default(c)
	session.Set("user_id", user.ID)
	session.Set("username", user.Username)
	session.Set("full_name", user.FullName)
	session.Set("is_super_admin", user.IsSuperAdmin)
	session.Set("session_token", token)
	session.Set("labs", labPaths)
	middleware.NewCSRFToken(session)
	if err := session.Save(); err != nil {
		h.renderTemplate(c, http.StatusInternalServerError, "login.html", gin.H{
			"title": "Login - Sistem Inventaris Lab",
			"error": "Gagal menyimpan session",
		})
		return
	}
	h.redirect(c, "/dashboard")
}

func (h *Handler) LoginPage(c *gin.Context) {
	session := sessions.Default(c)
	if session.Get("user_id") != nil {
		h.redirect(c, "/")
		return
	}
	token := middleware.NewCSRFToken(session)
	_ = session.Save()
	h.renderTemplate(c, http.StatusOK, "login.html", gin.H{
		"title":      "Login - " + c.GetString("lab"),
		"csrf_token": token,
	})
}

func (h *Handler) Logout(c *gin.Context) {
	session := sessions.Default(c)
	if userID, ok := session.Get("user_id").(int); ok {
		h.globalAuthService.Logout(userID)
	}

	session.Options(sessions.Options{
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   c.Request.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
	session.Clear()
	_ = session.Save()

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "inventaris_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   c.Request.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	h.redirect(c, "/login")
}

func (h *Handler) Home(c *gin.Context) {
	session := sessions.Default(c)
	if session.Get("user_id") != nil {
		h.redirect(c, "/dashboard")
		return
	}
	h.redirect(c, "/login")
}
