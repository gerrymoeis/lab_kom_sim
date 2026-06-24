package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-gonic/gin"
)

func (h *GlobalHandler) AdminUserList(c *gin.Context) {
	_, _, _, ok := middleware.GetCurrentUser(c)
	if !ok {
		h.render(c, http.StatusUnauthorized, "error.html", gin.H{
			"title":   "Unauthorized",
			"message": "Silakan login terlebih dahulu",
		})
		return
	}

	users, err := h.globalAuthService.ListUsers()
	if err != nil {
		h.render(c, http.StatusInternalServerError, "admin_users.html", gin.H{
			"title": "Manage Users",
			"error": "Gagal memuat data user",
		})
		return
	}

	h.render(c, http.StatusOK, "admin_users.html", gin.H{
		"title": "Manage Users",
		"users": users,
	})
}

func (h *GlobalHandler) AdminUserCreatePage(c *gin.Context) {
	h.render(c, http.StatusOK, "admin_user_form.html", gin.H{
		"title": "Buat User Baru",
		"user":  nil,
		"labs":  h.cfg.Labs,
	})
}

func (h *GlobalHandler) AdminUserCreate(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	fullName := c.PostForm("full_name")
	isSuperAdmin := c.PostForm("is_super_admin") == "1"

	if username == "" || password == "" {
		h.render(c, http.StatusBadRequest, "admin_user_form.html", gin.H{
			"title": "Buat User Baru",
			"error": "Username dan password harus diisi",
			"labs":  h.cfg.Labs,
		})
		return
	}

	if isSuperAdmin && !h.isProtected(c) {
		h.render(c, http.StatusForbidden, "admin_user_form.html", gin.H{
			"title": "Buat User Baru",
			"error": "Hanya Super Admin yang dapat membuat Super Admin baru",
			"labs":  h.cfg.Labs,
		})
		return
	}

	user, err := h.globalAuthService.CreateUser(username, password, fullName, isSuperAdmin)
	if err != nil {
		h.render(c, http.StatusBadRequest, "admin_user_form.html", gin.H{
			"title": "Buat User Baru",
			"error": "Gagal membuat user: " + err.Error(),
			"labs":  h.cfg.Labs,
		})
		return
	}

	if !isSuperAdmin {
		labs := c.PostFormArray("labs")
		roles := c.PostFormArray("roles")
		perms := make([]struct {
			LabURLPath string
			Role       string
		}, 0, len(labs))
		for i, lab := range labs {
			role := "admin"
			if i < len(roles) && roles[i] != "" {
				role = roles[i]
			}
			perms = append(perms, struct {
				LabURLPath string
				Role       string
			}{lab, role})
		}
		if err := h.globalAuthService.SetUserPermissions(user.ID, perms); err != nil {
			h.render(c, http.StatusInternalServerError, "admin_user_form.html", gin.H{
				"title": "Buat User Baru",
				"error": "User dibuat tetapi gagal set permissions",
				"labs":  h.cfg.Labs,
			})
			return
		}
	}

	c.Redirect(http.StatusFound, "/admin/users")
}

func (h *GlobalHandler) AdminUserEditPage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.AbortWithStatus(404)
		return
	}

	user, err := h.globalAuthService.GetUser(id)
	if err != nil {
		c.AbortWithStatus(404)
		return
	}

	perms, _ := h.globalAuthService.GetPermissions(user.ID)
	permMap := make(map[string]string)
	for _, p := range perms {
		permMap[p.LabURLPath] = p.Role
	}

	h.render(c, http.StatusOK, "admin_user_form.html", gin.H{
		"title":   "Edit User",
		"user":    user,
		"labs":    h.cfg.Labs,
		"permissions": permMap,
	})
}

func (h *GlobalHandler) AdminUserEdit(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.AbortWithStatus(404)
		return
	}

	targetUser, err := h.globalAuthService.GetUser(id)
	if err != nil {
		c.AbortWithStatus(404)
		return
	}

	if targetUser.IsSuperAdmin && !h.isProtected(c) {
		h.render(c, http.StatusForbidden, "admin_user_form.html", gin.H{
			"title": "Edit User",
			"error": "Hanya Super Admin yang dapat mengedit Super Admin",
		})
		return
	}

	username := c.PostForm("username")
	fullName := c.PostForm("full_name")
	isSuperAdmin := c.PostForm("is_super_admin") == "1"
	newPassword := c.PostForm("new_password")

	if username == "" {
		h.render(c, http.StatusBadRequest, "admin_user_form.html", gin.H{
			"title": "Edit User",
			"error": "Username harus diisi",
		})
		return
	}

	if err := h.globalAuthService.UpdateUser(id, username, fullName, isSuperAdmin); err != nil {
		h.render(c, http.StatusBadRequest, "admin_user_form.html", gin.H{
			"title": "Edit User",
			"error": "Gagal update user: " + err.Error(),
		})
		return
	}

	if newPassword != "" {
		if err := h.globalAuthService.UpdateUserPassword(id, newPassword); err != nil {
			h.render(c, http.StatusBadRequest, "admin_user_form.html", gin.H{
				"title": "Edit User",
				"error": "Gagal update password: " + err.Error(),
			})
			return
		}
	}

	if !isSuperAdmin {
		labs := c.PostFormArray("labs")
		roles := c.PostFormArray("roles")
		perms := make([]struct {
			LabURLPath string
			Role       string
		}, 0, len(labs))
		for i, lab := range labs {
			role := "admin"
			if i < len(roles) && roles[i] != "" {
				role = roles[i]
			}
			perms = append(perms, struct {
				LabURLPath string
				Role       string
			}{lab, role})
		}
		if err := h.globalAuthService.SetUserPermissions(id, perms); err != nil {
			h.render(c, http.StatusInternalServerError, "admin_user_form.html", gin.H{
				"title": "Edit User",
				"error": "Gagal update permissions",
			})
			return
		}
	}

	c.Redirect(http.StatusFound, "/admin/users")
}

func (h *GlobalHandler) AdminUserDelete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.AbortWithStatus(404)
		return
	}

	// Check if user is main account in any lab
	var mainCount int
	h.globalDB.QueryRow(`SELECT COUNT(*) FROM lab_permissions WHERE user_id = ? AND is_main_account = 1`, id).Scan(&mainCount)
	if mainCount > 0 {
		users, _ := h.globalAuthService.ListUsers()
		h.render(c, http.StatusForbidden, "admin_users.html", gin.H{
			"title": "Manage Users",
			"error": "User ini adalah akun utama lab dan tidak bisa dihapus",
			"users": users,
		})
		return
	}

	if err := h.globalAuthService.DeleteUser(id); err != nil {
		users, _ := h.globalAuthService.ListUsers()
		errMsg := "Gagal menghapus user"
		if errors.Is(err, services.ErrProtectedUser) {
			errMsg = "User ini tidak bisa dihapus (akun protected)"
		} else if errors.Is(err, services.ErrCannotDeleteSuperAdmin) {
			errMsg = "Tidak dapat menghapus super admin"
		}
		h.render(c, http.StatusForbidden, "admin_users.html", gin.H{
			"title": "Manage Users",
			"error": errMsg,
			"users": users,
		})
		return
	}

	c.Redirect(http.StatusFound, "/admin/users")
}

func (h *GlobalHandler) AdminUserPermissionsSave(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.AbortWithStatus(404)
		return
	}

	user, err := h.globalAuthService.GetUser(id)
	if err != nil {
		c.AbortWithStatus(404)
		return
	}

	if user.IsSuperAdmin {
		c.Redirect(http.StatusFound, "/admin/users")
		return
	}

	labs := c.PostFormArray("labs")
	roles := c.PostFormArray("roles")
	perms := make([]struct {
		LabURLPath string
		Role       string
	}, 0, len(labs))
	for i, lab := range labs {
		role := "admin"
		if i < len(roles) && roles[i] != "" {
			role = roles[i]
		}
		perms = append(perms, struct {
			LabURLPath string
			Role       string
		}{lab, role})
	}
	if err := h.globalAuthService.SetUserPermissions(id, perms); err != nil {
		h.render(c, http.StatusInternalServerError, "admin_user_permissions.html", gin.H{
			"title": "Permissions - " + user.Username,
			"user":  user,
			"labs":  h.cfg.Labs,
			"error": "Gagal menyimpan permissions",
		})
		return
	}

	c.Redirect(http.StatusFound, "/admin/users")
}

func (h *GlobalHandler) AdminUserPermissions(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.AbortWithStatus(404)
		return
	}

	user, err := h.globalAuthService.GetUser(id)
	if err != nil {
		c.AbortWithStatus(404)
		return
	}

	perms, _ := h.globalAuthService.GetPermissions(user.ID)
	permMap := make(map[string]string)
	for _, p := range perms {
		permMap[p.LabURLPath] = p.Role
	}

	h.render(c, http.StatusOK, "admin_user_permissions.html", gin.H{
		"title":       "Permissions - " + user.Username,
		"user":        user,
		"labs":        h.cfg.Labs,
		"permissions": permMap,
	})
}
