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
			"title": "Login - Sistem Inventaris Lab",
			"error": "Username dan password harus diisi",
		})
		return
	}

	ip, ua := getRequestContext(c)
	userID, fullName, role, token, isSuperAdmin, err := h.authService.Login(req.Username, req.Password, ip, ua)
	if err != nil {
		msg := "Username atau password salah"
		if errors.Is(err, services.ErrAlreadyLoggedIn) {
			msg = "Akun ini sudah login di tempat lain. Silakan logout terlebih dahulu."
		}
		status := http.StatusUnauthorized
		if errors.Is(err, services.ErrAlreadyLoggedIn) {
			status = http.StatusConflict
		}
		h.renderTemplate(c, status, "login.html", gin.H{
			"title": "Login - Sistem Inventaris Lab",
			"error": msg,
		})
		return
	}

	session := sessions.Default(c)
	session.Set("user_id", userID)
	session.Set("username", req.Username)
	session.Set("full_name", fullName)
	session.Set("role", role)
	session.Set("is_super_admin", isSuperAdmin)
	session.Set("session_token", token)
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
	if userID := session.Get("user_id"); userID != nil {
		h.redirect(c, "/dashboard")
		return
	}
	token := middleware.NewCSRFToken(session)
	_ = session.Save()
	h.renderTemplate(c, http.StatusOK, "login.html", gin.H{
		"title":      "Login - Sistem Inventaris Lab",
		"csrf_token": token,
		"lab":        c.GetString("lab"),
	})
}

func (h *Handler) Logout(c *gin.Context) {
	session := sessions.Default(c)
	if userID, ok := session.Get("user_id").(int); ok {
		username, _ := session.Get("username").(string)
		role, _ := session.Get("role").(string)
		ip, ua := getRequestContext(c)
		h.authService.Logout(userID, username, role, ip, ua)
	}
	session.Clear()
	session.Options(sessions.Options{
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   c.Request.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
	if err := session.Save(); err != nil {
		lab := c.GetString("lab")
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     middleware.LabCookieName(lab),
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   c.Request.TLS != nil,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})
	}
	h.redirect(c, "/login")
}

func (h *Handler) Home(c *gin.Context) {
	session := sessions.Default(c)
	if userID := session.Get("user_id"); userID != nil {
		h.redirect(c, "/dashboard")
		return
	}
	h.redirect(c, "/login")
}
