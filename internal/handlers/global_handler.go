package handlers

import (
	"errors"
	"net/http"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type GlobalHandler struct {
	cfg               *config.Config
	globalDB          *database.DB
	globalAuthService *services.GlobalAuthService
	labsDB            map[string]*database.DB
}

func NewGlobalHandler(cfg *config.Config, globalDB *database.DB, gas *services.GlobalAuthService, labsDB map[string]*database.DB) *GlobalHandler {
	return &GlobalHandler{
		cfg:               cfg,
		globalDB:          globalDB,
		globalAuthService: gas,
		labsDB:            labsDB,
	}
}

func (h *GlobalHandler) labFromPath(urlPath string) *config.LabConfig {
	for i := range h.cfg.Labs {
		if h.cfg.Labs[i].URLPath == urlPath {
			return &h.cfg.Labs[i]
		}
	}
	return nil
}

func (h *GlobalHandler) render(c *gin.Context, status int, tmpl string, data gin.H) {
	if token := sessions.Default(c).Get("csrf_token"); token != nil {
		data["csrf_token"] = token.(string)
	}
	data["lab"] = ""
	data["basePath"] = ""
	_, username, isSuperAdmin, _ := middleware.GetCurrentUser(c)
	data["username"] = username
	data["is_super_admin"] = isSuperAdmin
	c.HTML(status, tmpl, data)
}

func (h *GlobalHandler) LoginPage(c *gin.Context) {
	session := sessions.Default(c)
	if session.Get("user_id") != nil {
		c.Redirect(http.StatusFound, "/labs")
		return
	}
	token := middleware.NewCSRFToken(session)
	_ = session.Save()
	h.render(c, http.StatusOK, "login.html", gin.H{
		"title":      "Login - Sistem Inventaris Lab",
		"csrf_token": token,
	})
}

func (h *GlobalHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBind(&req); err != nil {
		h.render(c, http.StatusBadRequest, "login.html", gin.H{
			"title": "Login - Sistem Inventaris Lab",
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
		h.render(c, status, "login.html", gin.H{
			"title": "Login - Sistem Inventaris Lab",
			"error": msg,
		})
		return
	}

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
		h.render(c, http.StatusInternalServerError, "login.html", gin.H{
			"title": "Login - Sistem Inventaris Lab",
			"error": "Gagal menyimpan session",
		})
		return
	}

	c.Redirect(http.StatusFound, "/labs")
}

func (h *GlobalHandler) Logout(c *gin.Context) {
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
	c.Redirect(http.StatusFound, "/")
}

func (h *GlobalHandler) LabSelector(c *gin.Context) {
	session := sessions.Default(c)
	if session.Get("user_id") == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	userID, _ := session.Get("user_id").(int)
	username, _ := session.Get("username").(string)
	fullName, _ := session.Get("full_name").(string)
	isSuperAdmin, _ := session.Get("is_super_admin").(bool)

	var labs []config.LabConfig
	if isSuperAdmin {
		labs = h.cfg.Labs
	} else {
		allowedRaw := session.Get("labs")
		if allowedRaw == nil {
			c.Redirect(http.StatusFound, "/login")
			return
		}
		allowed, _ := allowedRaw.([]string)
		allowedSet := make(map[string]bool, len(allowed))
		for _, l := range allowed {
			allowedSet[l] = true
		}
		for _, lab := range h.cfg.Labs {
			if allowedSet[lab.URLPath] {
				labs = append(labs, lab)
			}
		}
	}

	isMainAccount := false
	if !isSuperAdmin {
		var count int
		_ = h.globalDB.QueryRow(
			`SELECT COUNT(*) FROM lab_permissions WHERE user_id = ? AND is_main_account = 1`,
			userID,
		).Scan(&count)
		isMainAccount = count > 0
	}

	h.render(c, http.StatusOK, "lab_selector.html", gin.H{
		"title":         "Pilih Laboratorium",
		"username":      username,
		"fullName":      fullName,
		"isSuperAdmin":  isSuperAdmin,
		"isMainAccount": isMainAccount,
		"labs":          labs,
	})
}

// --- Admin route stubs (Fase 5 will implement full UI) ---

func (h *GlobalHandler) AdminNotImplemented(c *gin.Context) {
	h.render(c, http.StatusOK, "error.html", gin.H{
		"title":   "Fitur dalam Pengembangan",
		"message": "Halaman ini akan tersedia di fase berikutnya.",
	})
}
