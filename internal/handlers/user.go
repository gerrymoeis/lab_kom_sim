package handlers

import (
	"net/http"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// UserList renders list of all users (admin only)
func (h *Handler) UserList(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	rows, err := h.db.Query(`
		SELECT id, username, full_name, role, created_at
		FROM users
		ORDER BY created_at DESC
	`)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data user",
		})
		return
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(&user.ID, &user.Username, &user.FullName, &user.Role, &user.CreatedAt)
		if err != nil {
			continue
		}
		users = append(users, user)
	}

	c.HTML(http.StatusOK, "user/list.html", gin.H{
		"title":    "Manajemen User - Sistem Inventaris Lab",
		"username": username,
		"role":     role,
		"users":    users,
	})
}

// UserCreatePage renders user creation form
func (h *Handler) UserCreatePage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	c.HTML(http.StatusOK, "user/create.html", gin.H{
		"title":    "Tambah User Baru - Sistem Inventaris Lab",
		"username": username,
		"role":     role,
	})
}

// UserCreate handles user creation
func (h *Handler) UserCreate(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	fullName := c.PostForm("full_name")
	role := c.PostForm("role")

	if username == "" || password == "" || fullName == "" {
		c.HTML(http.StatusBadRequest, "user/create.html", gin.H{
			"title": "Tambah User Baru",
			"error": "Semua field harus diisi",
		})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "user/create.html", gin.H{
			"title": "Tambah User Baru",
			"error": "Gagal mengenkripsi password",
		})
		return
	}

	_, err = h.db.Exec(`
		INSERT INTO users (username, password, full_name, role)
		VALUES (?, ?, ?, ?)
	`, username, string(hashedPassword), fullName, role)

	if err != nil {
		c.HTML(http.StatusInternalServerError, "user/create.html", gin.H{
			"title": "Tambah User Baru",
			"error": "Gagal menyimpan user. Username mungkin sudah digunakan.",
		})
		return
	}

	c.Redirect(http.StatusFound, "/admin/users")
}

// UserDelete handles user deletion
func (h *Handler) UserDelete(c *gin.Context) {
	id := c.Param("id")
	
	// Prevent deleting own account
	session := sessions.Default(c)
	currentUserID := session.Get("user_id")
	if currentUserID == id {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Tidak dapat menghapus akun sendiri",
		})
		return
	}

	_, err := h.db.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menghapus user",
		})
		return
	}

	c.Redirect(http.StatusFound, "/admin/users")
}

// Profile renders user profile page
func (h *Handler) Profile(c *gin.Context) {
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	var user models.User
	err := h.db.QueryRow(`
		SELECT id, username, full_name, role, created_at
		FROM users WHERE id = ?
	`, userID).Scan(&user.ID, &user.Username, &user.FullName, &user.Role, &user.CreatedAt)

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data profil",
		})
		return
	}

	c.HTML(http.StatusOK, "user/profile.html", gin.H{
		"title":    "Profil - Sistem Inventaris Lab",
		"username": username,
		"role":     role,
		"user":     user,
	})
}

// ChangePassword handles password change
func (h *Handler) ChangePassword(c *gin.Context) {
	userID, _, _, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	oldPassword := c.PostForm("old_password")
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	if oldPassword == "" || newPassword == "" || confirmPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Semua field harus diisi",
		})
		return
	}

	if newPassword != confirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Password baru dan konfirmasi tidak cocok",
		})
		return
	}

	// Get current password hash
	var currentHash string
	err := h.db.QueryRow("SELECT password FROM users WHERE id = ?", userID).Scan(&currentHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengambil data user",
		})
		return
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(oldPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Password lama salah",
		})
		return
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengenkripsi password baru",
		})
		return
	}

	// Update password
	_, err = h.db.Exec(`
		UPDATE users 
		SET password = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, string(newHash), userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengupdate password",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Password berhasil diubah",
	})
}
