package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/services"
	"inventaris-lab-kom/internal/timeutil"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// LabHandlerRegistrar allows hot-registering a per-lab handler at runtime
type LabHandlerRegistrar interface {
	Register(lab string, h *Handler)
}

type GlobalHandler struct {
	cfg               *config.Config
	globalDB          *database.DB
	globalAuthService *services.GlobalAuthService
	LabsDB            map[string]*database.DB
	registrar         LabHandlerRegistrar
	notifier          *services.MultiNotifier
}

func NewGlobalHandler(cfg *config.Config, globalDB *database.DB, gas *services.GlobalAuthService, labsDB map[string]*database.DB, registrar LabHandlerRegistrar, notifier *services.MultiNotifier) *GlobalHandler {
	return &GlobalHandler{
		cfg:               cfg,
		globalDB:          globalDB,
		globalAuthService: gas,
		LabsDB:            labsDB,
		registrar:         registrar,
		notifier:          notifier,
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
	if _, ok := data["lab"]; !ok {
		data["lab"] = ""
	}
	if _, ok := data["basePath"]; !ok {
		data["basePath"] = ""
	}
	_, username, isSuperAdmin, _, _ := middleware.GetCurrentUser(c)
	data["username"] = username
	data["is_super_admin"] = isSuperAdmin
	data["isSuperAdmin"] = isSuperAdmin
	data["is_protected"] = h.isProtected(c)
	data["is_global_admin"] = h.isGlobalAdmin(c)
	data["is_main_account"] = false
	data["navItems"] = loadNavItems("", true)
	data["navBrand"] = "Admin Panel"
	data["navBrandURL"] = "/labs"
	data["profileURL"] = "/labs/profile"
	data["logoutURL"] = "/logout"
	data["isGlobalArea"] = true

	// Inject flash messages from session (FlashReader middleware)
	if msg, exists := c.Get("_flash_error"); exists {
		if _, ok := data["error"]; !ok {
			data["error"] = msg.(string)
		}
	}
	if msg, exists := c.Get("_flash_success"); exists {
		if _, ok := data["success"]; !ok {
			data["success"] = msg.(string)
		}
	}

	c.HTML(status, tmpl, data)
}

func (h *GlobalHandler) isGlobalAdmin(c *gin.Context) bool {
	session := sessions.Default(c)
	val, _ := session.Get("is_global_admin").(bool)
	return val
}

func (h *GlobalHandler) isProtected(c *gin.Context) bool {
	session := sessions.Default(c)
	val, _ := session.Get("is_protected").(bool)
	return val
}

func (h *GlobalHandler) getDefaultCredentials() []models.DefaultCredential {
	creds, _ := h.globalAuthService.GetDefaultPasswordUsers()
	for i := range creds {
		for _, lab := range h.cfg.Labs {
			if lab.URLPath == creds[i].Username {
				creds[i].LabTitle = lab.Title
				break
			}
		}
		if creds[i].IsSuperAdmin {
			creds[i].LabTitle = "Super Admin"
		}
	}
	return creds
}

func (h *GlobalHandler) LoginPage(c *gin.Context) {
	session := sessions.Default(c)
	if session.Get("user_id") != nil {
		isSuperAdmin, _ := session.Get("is_super_admin").(bool)
		if isSuperAdmin {
			c.Redirect(http.StatusFound, "/labs")
			return
		}
		labsRaw := session.Get("labs")
		if labsRaw != nil {
			labs, ok := labsRaw.([]string)
			if ok && len(labs) > 0 {
				c.Redirect(http.StatusFound, "/"+labs[0]+"/dashboard")
				return
			}
		}
		c.Redirect(http.StatusFound, "/labs")
		return
	}
	token := middleware.NewCSRFToken(session)
	_ = session.Save()

	h.render(c, http.StatusOK, "login.html", gin.H{
		"title":             "Login - Sistem Inventaris Lab",
		"csrf_token":        token,
		"defaultCredentials": h.getDefaultCredentials(),
	})
}

func (h *GlobalHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBind(&req); err != nil {
		h.render(c, http.StatusBadRequest, "login.html", gin.H{
			"title":             "Login - Sistem Inventaris Lab",
			"error":             "Username dan password harus diisi",
			"defaultCredentials": h.getDefaultCredentials(),
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
			"title":             "Login - Sistem Inventaris Lab",
			"error":             msg,
			"defaultCredentials": h.getDefaultCredentials(),
		})
		return
	}

	labPaths := h.globalAuthService.GetLabsForUser(user.ID, user.IsSuperAdmin, user.IsGlobalAdmin, h.cfg.Labs)

	session := sessions.Default(c)
	session.Set("user_id", user.ID)
	session.Set("username", user.Username)
	session.Set("full_name", user.FullName)
	session.Set("is_super_admin", user.IsSuperAdmin)
	session.Set("is_protected", user.IsProtected)
	session.Set("is_global_admin", user.IsGlobalAdmin)
	session.Set("role", "admin")
	session.Set("session_token", token)
	session.Set("labs", labPaths)
	middleware.NewCSRFToken(session)
	if err := session.Save(); err != nil {
		h.render(c, http.StatusInternalServerError, "login.html", gin.H{
			"title":             "Login - Sistem Inventaris Lab",
			"error":             "Gagal menyimpan session",
			"defaultCredentials": h.getDefaultCredentials(),
		})
		return
	}

	ip, ua := getRequestContext(c)
	h.logAuthToLabs(user.ID, user.Username, "login", "", ip, ua, labPaths)
	if !user.IsSuperAdmin && len(labPaths) == 1 {
		c.Redirect(http.StatusFound, "/"+labPaths[0]+"/dashboard")
		return
	}
	c.Redirect(http.StatusFound, "/labs")
}

func (h *GlobalHandler) Logout(c *gin.Context) {
	session := sessions.Default(c)
	userID, hasUserID := session.Get("user_id").(int)
	username, _ := session.Get("username").(string)
	labsRaw := session.Get("labs")
	var labPaths []string
	if l, ok := labsRaw.([]string); ok {
		labPaths = l
	}
	ip, ua := getRequestContext(c)

	if hasUserID {
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

	if hasUserID {
		role, _ := session.Get("role").(string)
		h.logAuthToLabs(userID, username, "logout", role, ip, ua, labPaths)
	}
	c.Redirect(http.StatusFound, "/login")
}

func (h *GlobalHandler) logAuthToLabs(userID int, username, action, role, ip, ua string, labPaths []string) {
	for _, labPath := range labPaths {
		db, ok := h.LabsDB[labPath]
		if !ok {
			continue
		}
		status := "success"
		desc := fmt.Sprintf("User '%s' %s", username, action)
		db.Exec(`INSERT INTO activity_logs (user_id, username, user_role, action, entity_type, entity_id, description, old_values, new_values, created_at, ip_address, user_agent, status, error_message) VALUES (?, ?, ?, ?, 'auth', NULL, ?, '', '', ?, ?, ?, ?, '')`,
			userID, username, role, action, desc, timeutil.Now(), ip, ua, status)
	}
}

func (h *GlobalHandler) AdminProfile(c *gin.Context) {
	session := sessions.Default(c)
	userID, ok := session.Get("user_id").(int)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	username, _ := session.Get("username").(string)

	user, err := h.globalAuthService.GetUser(userID)
	if err != nil {
		h.render(c, http.StatusInternalServerError, "user/profile.html", gin.H{
			"title":       "Profile",
			"currentPage": "profile",
			"icon":        "bi-person-gear",
			"basePath":    "/labs",
			"error":       "User tidak ditemukan",
		})
		return
	}

	h.render(c, http.StatusOK, "user/profile.html", gin.H{
		"title":       "Profile",
		"currentPage": "profile",
		"icon":        "bi-person-gear",
		"basePath":    "/labs",
		"username":    username,
		"user":        user,
	})
}

func (h *GlobalHandler) AdminUpdateProfile(c *gin.Context) {
	session := sessions.Default(c)
	userID, ok := session.Get("user_id").(int)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	username := c.PostForm("username")
	fullName := c.PostForm("full_name")

	if username == "" || fullName == "" {
		h.render(c, http.StatusBadRequest, "user/profile.html", gin.H{
			"title":       "Profile",
			"currentPage": "profile",
			"icon":        "bi-person-gear",
			"basePath":    "/labs",
			"error":       "Username dan Nama Lengkap harus diisi",
		})
		return
	}

	exists, _ := h.globalAuthService.GetUserByUsername(username)
	if exists != nil && exists.ID != userID {
		h.render(c, http.StatusBadRequest, "user/profile.html", gin.H{
			"title":       "Profile",
			"currentPage": "profile",
			"icon":        "bi-person-gear",
			"basePath":    "/labs",
			"error":       "Username sudah digunakan",
		})
		return
	}

	currentUser, err := h.globalAuthService.GetUser(userID)
	if err != nil {
		h.render(c, http.StatusInternalServerError, "user/profile.html", gin.H{
			"title":       "Profile",
			"currentPage": "profile",
			"icon":        "bi-person-gear",
			"basePath":    "/labs",
			"error":       "Sesi tidak valid",
		})
		return
	}

	if err := h.globalAuthService.UpdateUser(userID, username, fullName, currentUser.IsSuperAdmin, currentUser.IsGlobalAdmin, currentUser.IsProtected); err != nil {
		h.render(c, http.StatusBadRequest, "user/profile.html", gin.H{
			"title":       "Profile",
			"currentPage": "profile",
			"icon":        "bi-person-gear",
			"basePath":    "/labs",
			"error":       "Gagal update profil",
		})
		return
	}

	session.Set("username", username)
	if err := session.Save(); err != nil {
		h.render(c, http.StatusInternalServerError, "user/profile.html", gin.H{
			"title":       "Profile",
			"currentPage": "profile",
			"icon":        "bi-person-gear",
			"basePath":    "/labs",
			"error":       "Gagal menyimpan session",
		})
		return
	}

	userModel, _ := h.globalAuthService.GetUser(userID)
	h.render(c, http.StatusOK, "user/profile.html", gin.H{
		"title":       "Profile",
		"currentPage": "profile",
		"icon":        "bi-person-gear",
		"basePath":    "/labs",
		"username":    username,
		"user":        userModel,
		"success":     "Profil berhasil diupdate",
	})
}

func (h *GlobalHandler) AdminChangePassword(c *gin.Context) {
	session := sessions.Default(c)
	userID, ok := session.Get("user_id").(int)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	oldPassword := c.PostForm("old_password")
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	if oldPassword == "" || newPassword == "" {
		h.render(c, http.StatusBadRequest, "user/profile.html", gin.H{
			"title":       "Profile",
			"currentPage": "profile",
			"icon":        "bi-person-gear",
			"basePath":    "/labs",
			"error":       "Password lama dan baru harus diisi",
		})
		return
	}

	if newPassword != confirmPassword {
		h.render(c, http.StatusBadRequest, "user/profile.html", gin.H{
			"title":       "Profile",
			"currentPage": "profile",
			"icon":        "bi-person-gear",
			"basePath":    "/labs",
			"error":       "Password baru tidak cocok",
		})
		return
	}

	userModel, err := h.globalAuthService.GetUser(userID)
	if err != nil {
		h.render(c, http.StatusInternalServerError, "user/profile.html", gin.H{
			"title":       "Profile",
			"currentPage": "profile",
			"icon":        "bi-person-gear",
			"basePath":    "/labs",
			"error":       "Sesi tidak valid",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(userModel.Password), []byte(oldPassword)); err != nil {
		h.render(c, http.StatusBadRequest, "user/profile.html", gin.H{
			"title":       "Profile",
			"currentPage": "profile",
			"icon":        "bi-person-gear",
			"basePath":    "/labs",
			"error":       "Password lama salah",
		})
		return
	}

	if err := h.globalAuthService.UpdateUserPassword(userID, newPassword); err != nil {
		h.render(c, http.StatusInternalServerError, "user/profile.html", gin.H{
			"title":       "Profile",
			"currentPage": "profile",
			"icon":        "bi-person-gear",
			"basePath":    "/labs",
			"error":       "Gagal ubah password",
		})
		return
	}

	userModel, _ = h.globalAuthService.GetUser(userID)
	username, _ := session.Get("username").(string)
	h.render(c, http.StatusOK, "user/profile.html", gin.H{
		"title":       "Profile",
		"currentPage": "profile",
		"icon":        "bi-person-gear",
		"basePath":    "/labs",
		"username":    username,
		"user":        userModel,
		"success":     "Password berhasil diubah",
	})
}

func (h *GlobalHandler) LabSelector(c *gin.Context) {
	session := sessions.Default(c)
	if session.Get("user_id") == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	isSuperAdmin, _ := session.Get("is_super_admin").(bool)
	isGlobalAdmin, _ := session.Get("is_global_admin").(bool)
	if !isSuperAdmin && !isGlobalAdmin {
		labsRaw := session.Get("labs")
		if labsRaw != nil {
			labs, ok := labsRaw.([]string)
			if ok && len(labs) > 0 {
				c.Redirect(http.StatusFound, "/"+labs[0]+"/dashboard")
				return
			}
		}
		h.render(c, http.StatusForbidden, "error.html", gin.H{
			"title":       "Akses Ditolak",
			"message":     "Anda tidak memiliki akses ke laboratorium manapun.",
			"currentPage": "",
			"role":        "",
		})
		return
	}

	userID, _ := session.Get("user_id").(int)
	username, _ := session.Get("username").(string)
	fullName, _ := session.Get("full_name").(string)

	var labs []config.LabConfig
	if isSuperAdmin || isGlobalAdmin {
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
	if !isSuperAdmin && !isGlobalAdmin {
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
		"isGlobalAdmin": isGlobalAdmin,
		"labs":          labs,
	})
}


