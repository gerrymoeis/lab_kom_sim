package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"inventaris-lab-kom/internal/models"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) UserList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	var users []models.User
	if err := h.db.X.Select(&users, `SELECT id, username, full_name, role, created_at FROM users ORDER BY created_at DESC`); err != nil {
		h.errHTML(c, "Gagal mengambil data user")
		return
	}

	c.HTML(http.StatusOK, "user/list.html", gin.H{
		"title": "Manajemen User", "currentPage": "users",
		"username": username, "role": role, "users": users,
		"error": c.Query("error"),
	})
}

func (h *Handler) UserCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	c.HTML(http.StatusOK, "user/create.html", gin.H{
		"title": "Tambah User Baru", "currentPage": "users",
		"username": username, "role": role,
	})
}

func (h *Handler) UserCreate(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	fullName := c.PostForm("full_name")
	role := c.PostForm("role")

	if username == "" || password == "" || fullName == "" {
		c.HTML(http.StatusBadRequest, "user/create.html", gin.H{"title": "Tambah User Baru", "error": "Semua field harus diisi"})
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if _, err := h.db.Exec(`INSERT INTO users (username, password, full_name, role) VALUES (?, ?, ?, ?)`, username, string(hash), fullName, role); err != nil {
		c.HTML(http.StatusInternalServerError, "user/create.html", gin.H{"title": "Tambah User Baru", "error": "Gagal menyimpan user. Username mungkin sudah digunakan."})
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	h.activityLogService.LogCreate(uid, u, r, "user", 0, map[string]interface{}{"username": username, "full_name": fullName, "role": role}, ip, ua)
	c.Redirect(http.StatusFound, "/admin/users")
}

func (h *Handler) UserDelete(c *gin.Context) {
	idStr := c.Param("id")
	targetID, err := strconv.Atoi(idStr)
	if err != nil {
		h.redirectWithError(c, "/admin/users", "ID tidak valid")
		return
	}

	sess := sessions.Default(c)
	currentUserID, _ := sess.Get("user_id").(int)
	if currentUserID == targetID {
		h.redirectWithError(c, "/admin/users", "Tidak dapat menghapus akun sendiri")
		return
	}

	var uname string
	if err := h.db.QueryRow(`SELECT username FROM users WHERE id = ?`, targetID).Scan(&uname); err != nil {
		h.redirectWithError(c, "/admin/users", "User tidak ditemukan")
		return
	}
	if uname == "admin" || uname == "rekan" {
		h.redirectWithError(c, "/admin/users", "Tidak dapat menghapus akun admin utama")
		return
	}

	if _, err := h.db.Exec("DELETE FROM users WHERE id = ?", targetID); err != nil {
		h.redirectWithError(c, "/admin/users", "Gagal menghapus user")
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	h.activityLogService.LogDelete(uid, u, r, "user", targetID, map[string]interface{}{"deleted_username": uname}, ip, ua)
	c.Redirect(http.StatusFound, "/admin/users")
}

func (h *Handler) Profile(c *gin.Context) {
	userID, username, role, ok := h.user(c)
	if !ok { return }

	var user models.User
	h.db.X.Get(&user, `SELECT id, username, full_name, role, created_at FROM users WHERE id = ?`, userID)

	c.HTML(http.StatusOK, "user/profile.html", gin.H{
		"title": "Profil", "currentPage": "profile",
		"username": username, "role": role, "user": user,
		"success": c.Query("success"), "error": c.Query("error"),
	})
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	userID, _, role, ok := h.user(c)
	if !ok { return }

	newUsername := strings.TrimSpace(c.PostForm("username"))
	newFullName := strings.TrimSpace(c.PostForm("full_name"))

	if newUsername == "" || newFullName == "" {
		c.Redirect(http.StatusFound, "/profile?error=Username dan Nama Lengkap harus diisi")
		return
	}

	var exists int
	h.db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ? AND id != ?`, newUsername, userID).Scan(&exists)
	if exists > 0 {
		c.Redirect(http.StatusFound, "/profile?error=Username sudah digunakan")
		return
	}

	if _, err := h.db.Exec(`UPDATE users SET username = ?, full_name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, newUsername, newFullName, userID); err != nil {
		c.Redirect(http.StatusFound, "/profile?error=Gagal mengupdate profil")
		return
	}

	sess := sessions.Default(c)
	sess.Set("username", newUsername)
	sess.Set("full_name", newFullName)
	sess.Save()

	ip, ua := getRequestContext(c)
	h.activityLogService.LogUpdate(userID, newUsername, role, "user", userID,
		map[string]interface{}{"id": userID},
		map[string]interface{}{"username": newUsername, "full_name": newFullName}, ip, ua)

	c.Redirect(http.StatusFound, "/profile?success=Profil berhasil diupdate")
}

func (h *Handler) ChangePassword(c *gin.Context) {
	userID, _, _, ok := h.user(c)
	if !ok { return }

	oldPw := c.PostForm("old_password")
	newPw := c.PostForm("new_password")
	confirmPw := c.PostForm("confirm_password")

	if oldPw == "" || newPw == "" || confirmPw == "" {
		c.Redirect(http.StatusFound, "/profile?error=Semua field harus diisi")
		return
	}
	if newPw != confirmPw {
		c.Redirect(http.StatusFound, "/profile?error=Password baru dan konfirmasi tidak cocok")
		return
	}

	var hash string
	if err := h.db.QueryRow("SELECT password FROM users WHERE id = ?", userID).Scan(&hash); err != nil {
		c.Redirect(http.StatusFound, "/profile?error=Gagal mengambil data user")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(oldPw)) != nil {
		c.Redirect(http.StatusFound, "/profile?error=Password lama salah")
		return
	}

	newHash, _ := bcrypt.GenerateFromPassword([]byte(newPw), bcrypt.DefaultCost)
	if _, err := h.db.Exec(`UPDATE users SET password = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, string(newHash), userID); err != nil {
		c.Redirect(http.StatusFound, "/profile?error=Gagal mengupdate password")
		return
	}
	c.Redirect(http.StatusFound, "/profile?success=Password berhasil diubah")
}
