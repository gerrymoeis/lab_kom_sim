package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"inventaris-lab-kom/internal/services"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func (h *Handler) UserList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	users, err := h.userRepo.List()
	if err != nil { h.errHTML(c, "Gagal mengambil data user"); return }

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
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)

	if err := h.userService.CreateUser(uid, u, r, req.Username, req.Password, req.FullName, req.Role, ip, ua); err != nil {
		c.HTML(http.StatusInternalServerError, "user/create.html", gin.H{"title": "Tambah User Baru", "error": "Gagal menyimpan user. Username mungkin sudah digunakan."})
		return
	}
	c.Redirect(http.StatusFound, "/admin/users")
}

func (h *Handler) UserDelete(c *gin.Context) {
	targetID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.redirectWithError(c, "/admin/users", "ID tidak valid")
		return
	}
	sess := sessions.Default(c)
	currentUserID, _ := sess.Get("user_id").(int)
	u, _ := sess.Get("username").(string)
	r, _ := sess.Get("role").(string)
	ip, ua := getRequestContext(c)

	if err := h.userService.DeleteUser(currentUserID, targetID, u, r, ip, ua); err != nil {
		msg := "Gagal menghapus user"
		if errors.Is(err, services.ErrSelfDelete) { msg = "Tidak dapat menghapus akun sendiri" }
		if errors.Is(err, services.ErrProtectedDelete) { msg = "Tidak dapat menghapus akun admin utama" }
		if errors.Is(err, services.ErrUserNotFound) { msg = "User tidak ditemukan" }
		h.redirectWithError(c, "/admin/users", msg)
		return
	}
	c.Redirect(http.StatusFound, "/admin/users")
}

func (h *Handler) Profile(c *gin.Context) {
	userID, username, role, ok := h.user(c)
	if !ok { return }

	user, err := h.userRepo.GetByID(userID)
	if err != nil {
		h.redirectWithError(c, "/profile", "User tidak ditemukan")
		return
	}
	c.HTML(http.StatusOK, "user/profile.html", gin.H{
		"title": "Profil", "currentPage": "profile",
		"username": username, "role": role, "user": user,
		"success": c.Query("success"), "error": c.Query("error"),
	})
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
	if err := c.ShouldBind(&req); err != nil {
		c.Redirect(http.StatusFound, "/profile?error=Username dan Nama Lengkap harus diisi")
		return
	}
	userID, u, r, ok := h.user(c)
	if !ok { return }
	ip, ua := getRequestContext(c)

	newUsername, newFullName, err := h.userService.UpdateProfile(userID, req.Username, req.FullName, u, r, ip, ua)
	if err != nil {
		msg := "Gagal mengupdate profil"
		if errors.Is(err, services.ErrUsernameTaken) { msg = "Username sudah digunakan" }
		c.Redirect(http.StatusFound, "/profile?error="+msg)
		return
	}

	sess := sessions.Default(c)
	sess.Set("username", newUsername)
	sess.Set("full_name", newFullName)
	sess.Save()
	c.Redirect(http.StatusFound, "/profile?success=Profil berhasil diupdate")
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBind(&req); err != nil {
		c.Redirect(http.StatusFound, "/profile?error=Semua field harus diisi")
		return
	}
	userID, _, _, ok := h.user(c)
	if !ok { return }

	if err := h.userService.ChangePassword(userID, req.OldPassword, req.NewPassword, req.ConfirmPassword); err != nil {
		msg := "Gagal mengubah password"
		if errors.Is(err, services.ErrPasswordMismatch) { msg = "Password baru dan konfirmasi tidak cocok" }
		if errors.Is(err, services.ErrWrongPassword) { msg = "Password lama salah" }
		c.Redirect(http.StatusFound, "/profile?error="+msg)
		return
	}
	c.Redirect(http.StatusFound, "/profile?success=Password berhasil diubah")
}
