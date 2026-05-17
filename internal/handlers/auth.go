package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func generateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// LoginPage renders login page
func (h *Handler) LoginPage(c *gin.Context) {
	// Check if already logged in
	session := sessions.Default(c)
	if userID := session.Get("user_id"); userID != nil {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "Login - Sistem Inventaris Lab",
	})
}

// Login handles login form submission
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"title": "Login - Sistem Inventaris Lab",
			"error": "Username dan password harus diisi",
		})
		return
	}

	// Query user from database
	var userID int
	var hashedPassword, fullName, role string
	err := h.db.QueryRow(`
		SELECT id, password, full_name, role 
		FROM users 
		WHERE username = ?
	`, req.Username).Scan(&userID, &hashedPassword, &fullName, &role)

	if err == sql.ErrNoRows {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"title": "Login - Sistem Inventaris Lab",
			"error": "Username atau password salah",
		})
		return
	}

	if err != nil {
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{
			"title": "Login - Sistem Inventaris Lab",
			"error": "Terjadi kesalahan sistem",
		})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
		ipAddress, userAgent := getRequestContext(c)
		h.activityLogService.LogAuth(
			0, req.Username, "", "login", false,
			ipAddress, userAgent, "Invalid password",
		)
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"title": "Login - Sistem Inventaris Lab",
			"error": "Username atau password salah",
		})
		return
	}

	// Check if account is already logged in elsewhere
	var existingToken string
	h.db.QueryRow(`SELECT session_token FROM users WHERE id = ?`, userID).Scan(&existingToken)
	if existingToken != "" {
		ipAddress, userAgent := getRequestContext(c)
		h.activityLogService.LogAuth(
			userID, req.Username, role, "login", false,
			ipAddress, userAgent, "Account already logged in elsewhere",
		)
		c.HTML(http.StatusConflict, "login.html", gin.H{
			"title": "Login - Sistem Inventaris Lab",
			"error": "Akun sedang digunakan di perangkat lain. Logout terlebih dahulu atau tunggu sesi berakhir.",
		})
		return
	}

	token, err := generateSessionToken()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{
			"title": "Login - Sistem Inventaris Lab",
			"error": "Gagal generate token",
		})
		return
	}

	_, err = h.db.Exec(`UPDATE users SET session_token = ? WHERE id = ?`, token, userID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{
			"title": "Login - Sistem Inventaris Lab",
			"error": "Gagal menyimpan token",
		})
		return
	}

	session := sessions.Default(c)
	session.Set("user_id", userID)
	session.Set("username", req.Username)
	session.Set("full_name", fullName)
	session.Set("role", role)
	session.Set("session_token", token)
	if err := session.Save(); err != nil {
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{
			"title": "Login - Sistem Inventaris Lab",
			"error": "Gagal menyimpan session",
		})
		return
	}
	ipAddress, userAgent := getRequestContext(c)
	h.activityLogService.LogAuth(
		userID, req.Username, role, "login", true,
		ipAddress, userAgent, "",
	)

	c.Redirect(http.StatusFound, "/dashboard")
}

// Logout handles logout
func (h *Handler) Logout(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	username := session.Get("username")
	role := session.Get("role")

	if userID != nil {
		// Clear session token from DB so other login sessions are invalidated
		h.db.Exec(`UPDATE users SET session_token = NULL WHERE id = ?`, userID.(int))

		ipAddress, userAgent := getRequestContext(c)
		if username != nil && role != nil {
			h.activityLogService.LogAuth(
				userID.(int), username.(string), role.(string), "logout", true,
				ipAddress, userAgent, "",
			)
		}
	}

	session.Clear()
	session.Save()
	c.Redirect(http.StatusFound, "/login")
}

// Home redirects to dashboard or login
func (h *Handler) Home(c *gin.Context) {
	session := sessions.Default(c)
	if userID := session.Get("user_id"); userID != nil {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}
	c.Redirect(http.StatusFound, "/login")
}
