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

	rows, err := h.db.Query(`SELECT id, username, full_name, role, created_at FROM users ORDER BY created_at DESC`)
	if err != nil { h.errHTML(c, "Gagal mengambil data user"); return }
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if rows.Scan(&u.ID, &u.Username, &u.FullName, &u.Role, &u.CreatedAt) == nil { users = append(users, u) }
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
	var req CreateUserRequest
	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "user/create.html", gin.H{"title": "Tambah User Baru", "error": "Semua field harus diisi"})
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if _, err := h.db.Exec(`INSERT INTO users (username, password, full_name, role) VALUES (?, ?, ?, ?)`, req.Username, string(hash), req.FullName, req.Role); err != nil {
		c.HTML(http.StatusInternalServerError, "user/create.html", gin.H{"title": "Tambah User Baru", "error": "Gagal menyimpan user. Username mungkin sudah digunakan."})
		return
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	h.activityLogService.LogCreate(uid, u, r, "user", 0, map[string]interface{}{"username": req.Username, "full_name": req.FullName, "role": req.Role}, ip, ua)
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
	h.db.QueryRow(`SELECT id, username, full_name, role, created_at FROM users WHERE id = ?`, userID).Scan(&user.ID, &user.Username, &user.FullName, &user.Role, &user.CreatedAt)

	c.HTML(http.StatusOK, "user/profile.html", gin.H{
		"title": "Profil", "currentPage": "profile",
		"username": username, "role": role, "user": user,
		"success": c.Query("success"), "error": c.Query("error"),
	})
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	userID, _, role, ok := h.user(c)
	if !ok { return }

	var req UpdateProfileRequest
	if err := c.ShouldBind(&req); err != nil {
		c.Redirect(http.StatusFound, "/profile?error=Username dan Nama Lengkap harus diisi")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.FullName = strings.TrimSpace(req.FullName)

	var exists int
	h.db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ? AND id != ?`, req.Username, userID).Scan(&exists)
	if exists > 0 {
		c.Redirect(http.StatusFound, "/profile?error=Username sudah digunakan")
		return
	}

	if _, err := h.db.Exec(`UPDATE users SET username = ?, full_name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, req.Username, req.FullName, userID); err != nil {
		c.Redirect(http.StatusFound, "/profile?error=Gagal mengupdate profil")
		return
	}

	sess := sessions.Default(c)
	sess.Set("username", req.Username)
	sess.Set("full_name", req.FullName)
	sess.Save()

	ip, ua := getRequestContext(c)
	h.activityLogService.LogUpdate(userID, req.Username, role, "user", userID,
		map[string]interface{}{"id": userID},
		map[string]interface{}{"username": req.Username, "full_name": req.FullName}, ip, ua)

	c.Redirect(http.StatusFound, "/profile?success=Profil berhasil diupdate")
}

func (h *Handler) ChangePassword(c *gin.Context) {
	userID, _, _, ok := h.user(c)
	if !ok { return }

	var req ChangePasswordRequest
	if err := c.ShouldBind(&req); err != nil {
		c.Redirect(http.StatusFound, "/profile?error=Semua field harus diisi")
		return
	}
	if req.NewPassword != req.ConfirmPassword {
		c.Redirect(http.StatusFound, "/profile?error=Password baru dan konfirmasi tidak cocok")
		return
	}

	var hash string
	if err := h.db.QueryRow("SELECT password FROM users WHERE id = ?", userID).Scan(&hash); err != nil {
		c.Redirect(http.StatusFound, "/profile?error=Gagal mengambil data user")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.OldPassword)) != nil {
		c.Redirect(http.StatusFound, "/profile?error=Password lama salah")
		return
	}

	newHash, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if _, err := h.db.Exec(`UPDATE users SET password = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, string(newHash), userID); err != nil {
		c.Redirect(http.StatusFound, "/profile?error=Gagal mengupdate password")
		return
	}
	c.Redirect(http.StatusFound, "/profile?success=Password berhasil diubah")
}
