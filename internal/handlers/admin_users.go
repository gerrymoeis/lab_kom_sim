package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"html/template"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func (h *GlobalHandler) isSelf(c *gin.Context, target *models.GlobalUser) bool {
	session := sessions.Default(c)
	currentID, _ := session.Get("user_id").(int)
	return currentID == target.ID
}

func (h *GlobalHandler) canViewUser(c *gin.Context, target *models.GlobalUser) bool {
	if target.IsSuperAdmin || target.IsProtected {
		_, username, _, _, _ := middleware.GetCurrentUser(c)
		return username == target.Username
	}
	return true
}

func (h *GlobalHandler) canEditUser(c *gin.Context, target *models.GlobalUser) bool {
	if h.isProtected(c) {
		return true
	}
	if target.IsProtected {
		return false
	}
	if target.IsSuperAdmin {
		return false
	}
	if target.IsGlobalAdmin {
		return false
	}
	var mainCount int
	h.globalDB.QueryRow(`SELECT COUNT(*) FROM lab_permissions WHERE user_id = ? AND is_main_account = 1`, target.ID).Scan(&mainCount)
	if mainCount > 0 {
		return false
	}
	if h.isGlobalAdmin(c) {
		return true
	}
	_, _, isSuperAdmin, _, _ := middleware.GetCurrentUser(c)
	return isSuperAdmin
}

func (h *GlobalHandler) canDeleteUser(c *gin.Context, target *models.GlobalUser) bool {
	return h.canEditUser(c, target) && !h.isSelf(c, target)
}

func (h *GlobalHandler) AdminUserDetail(c *gin.Context) {
	targetUsername := c.Param("username")

	user, err := h.globalAuthService.GetUserByUsername(targetUsername)
	if err != nil {
		c.Redirect(http.StatusFound, "/labs/admin/users")
		return
	}

	if !h.canViewUser(c, user) {
		users, _ := h.globalAuthService.ListUsers()
		h.render(c, http.StatusForbidden, "admin/users.html", gin.H{
			"title":       "Manage Users",
			"currentPage": "users",
			"icon":        "bi-people",
			"error":       "Tidak dapat mengakses profil user ini",
			"users":       users,
			"filters":     map[string]string{},
		})
		return
	}

	var mainCount int
	h.globalDB.QueryRow(`SELECT COUNT(*) FROM lab_permissions WHERE user_id = ? AND is_main_account = 1`, user.ID).Scan(&mainCount)

	h.render(c, http.StatusOK, "user/detail.html", gin.H{
		"title":               "Detail User - " + user.Username,
		"currentPage":         "users",
		"user":                user,
		"targetIsMainAccount": mainCount > 0,
	})
}

func (h *GlobalHandler) AdminUserList(c *gin.Context) {
	_, _, _, _, ok := middleware.GetCurrentUser(c)
	if !ok {
		h.render(c, http.StatusUnauthorized, "error.html", gin.H{
			"title":   "Unauthorized",
			"message": "Silakan login terlebih dahulu",
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := h.cfg.DefaultPageSize
	search := c.Query("search")
	sortBy := c.Query("sort_by")
	sortOrder := c.Query("sort_order")

	values, _ := url.ParseQuery(c.Request.URL.RawQuery)
	delete(values, "page")
	values.Del("success")
	values.Del("error")
	values.Del("toast")
	var query interface{} = ""
	if len(values) > 0 {
		query = template.URL("&" + values.Encode())
	}

	users, total, err := h.globalAuthService.ListUsersPaginated(search, sortBy, sortOrder, page, pageSize)
	if err != nil {
		h.render(c, http.StatusInternalServerError, "admin/users.html", gin.H{
			"title":       "Manage Users",
			"currentPage": "users",
			"icon":        "bi-people",
			"error":       "Gagal memuat data user",
			"filters":     map[string]string{},
		})
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	startRow := (page-1)*pageSize + 1

	h.render(c, http.StatusOK, "admin/users.html", gin.H{
		"title":       "Manage Users",
		"currentPage": "users",
		"icon":        "bi-people",
		"users":       users,
		"page":        page,
		"startRow":    startRow,
		"totalPages":  totalPages,
		"totalItems":  total,
		"query":       query,
		"filters":     map[string]string{"search": search, "sort_by": sortBy, "sort_order": sortOrder},
	})
}

func (h *GlobalHandler) AdminUserCreatePage(c *gin.Context) {
	h.render(c, http.StatusOK, "admin/user_form.html", gin.H{
		"title":       "Buat User Baru",
		"currentPage": "users",
		"icon":        "bi-person-plus",
		"user":        nil,
		"labs":        h.cfg.Labs,
	})
}

func (h *GlobalHandler) AdminUserCreate(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	fullName := c.PostForm("full_name")
	isGlobalAdmin := c.PostForm("is_global_admin") == "1"
	isProtected := c.PostForm("is_protected") == "1"

	if username == "" || password == "" {
		h.render(c, http.StatusBadRequest, "admin/user_form.html", gin.H{
			"title":       "Buat User Baru",
			"currentPage": "users",
			"icon":        "bi-person-plus",
			"error":       "Username dan password harus diisi",
			"labs":        h.cfg.Labs,
		})
		return
	}

	if isGlobalAdmin && !h.isProtected(c) {
		h.render(c, http.StatusForbidden, "admin/user_form.html", gin.H{
			"title":       "Buat User Baru",
			"currentPage": "users",
			"icon":        "bi-person-plus",
			"error":       "Hanya Super Admin yang dapat membuat Global Admin Biasa",
			"labs":        h.cfg.Labs,
		})
		return
	}

	if isProtected && !h.isProtected(c) {
		h.render(c, http.StatusForbidden, "admin/user_form.html", gin.H{
			"title":       "Buat User Baru",
			"currentPage": "users",
			"icon":        "bi-person-plus",
			"error":       "Hanya Super Admin (root) yang dapat membuat user protected",
			"labs":        h.cfg.Labs,
		})
		return
	}

	user, err := h.globalAuthService.CreateUser(username, password, fullName, false, isGlobalAdmin, isProtected)
	if err != nil {
		h.render(c, http.StatusBadRequest, "admin/user_form.html", gin.H{
			"title":       "Buat User Baru",
			"currentPage": "users",
			"icon":        "bi-person-plus",
			"error":       "Gagal membuat user: " + err.Error(),
			"labs":        h.cfg.Labs,
		})
		return
	}

	if isGlobalAdmin {
		perms := make([]struct {
			LabURLPath string
			Role       string
		}, len(h.cfg.Labs))
		for i, lab := range h.cfg.Labs {
			perms[i] = struct {
				LabURLPath string
				Role       string
			}{lab.URLPath, "admin"}
		}
		if err := h.globalAuthService.SetUserPermissions(user.ID, perms); err != nil {
			h.render(c, http.StatusInternalServerError, "admin/user_form.html", gin.H{
				"title":       "Buat User Baru",
				"currentPage": "users",
				"icon":        "bi-person-plus",
				"error":       "User dibuat tetapi gagal set permissions",
				"labs":        h.cfg.Labs,
			})
			return
		}
	} else {
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
			h.render(c, http.StatusInternalServerError, "admin/user_form.html", gin.H{
				"title":       "Buat User Baru",
				"currentPage": "users",
				"icon":        "bi-person-plus",
				"error":       "User dibuat tetapi gagal set permissions",
				"labs":        h.cfg.Labs,
			})
			return
		}
	}

	c.Redirect(http.StatusFound, "/labs/admin/users")
}

func (h *GlobalHandler) AdminUserEditPage(c *gin.Context) {
	targetUsername := c.Param("username")

	user, err := h.globalAuthService.GetUserByUsername(targetUsername)
	if err != nil {
		c.AbortWithStatus(404)
		return
	}

	perms, _ := h.globalAuthService.GetPermissions(user.ID)
	permMap := make(map[string]string)
	for _, p := range perms {
		permMap[p.LabURLPath] = p.Role
	}

	h.render(c, http.StatusOK, "admin/user_form.html", gin.H{
		"title":       "Edit User",
		"currentPage": "users",
		"icon":        "bi-pencil",
		"user":        user,
		"labs":        h.cfg.Labs,
		"permissions": permMap,
	})
}

func (h *GlobalHandler) AdminUserEdit(c *gin.Context) {
	targetUsername := c.Param("username")

	targetUser, err := h.globalAuthService.GetUserByUsername(targetUsername)
	if err != nil {
		c.AbortWithStatus(404)
		return
	}
	id := targetUser.ID

	if targetUser.IsSuperAdmin && !h.isProtected(c) {
		h.render(c, http.StatusForbidden, "admin/user_form.html", gin.H{
			"title":       "Edit User",
			"currentPage": "users",
			"icon":        "bi-pencil",
			"error":       "Hanya Super Admin yang dapat mengedit Super Admin",
		})
		return
	}

	username := c.PostForm("username")
	fullName := c.PostForm("full_name")
	newPassword := c.PostForm("new_password")
	isGlobalAdmin := c.PostForm("is_global_admin") == "1"
	isProtected := c.PostForm("is_protected") == "1"

	if isGlobalAdmin && !h.isProtected(c) {
		h.render(c, http.StatusForbidden, "admin/user_form.html", gin.H{
			"title":       "Edit User",
			"currentPage": "users",
			"icon":        "bi-pencil",
			"error":       "Hanya Super Admin yang dapat mengubah status Global Admin Biasa",
		})
		return
	}

	if isProtected != targetUser.IsProtected && !h.isProtected(c) {
		h.render(c, http.StatusForbidden, "admin/user_form.html", gin.H{
			"title":       "Edit User",
			"currentPage": "users",
			"icon":        "bi-pencil",
			"error":       "Hanya Super Admin (root) yang dapat mengubah status protected",
		})
		return
	}

	if username == "" {
		h.render(c, http.StatusBadRequest, "admin/user_form.html", gin.H{
			"title":       "Edit User",
			"currentPage": "users",
			"icon":        "bi-pencil",
			"error":       "Username harus diisi",
		})
		return
	}

	if err := h.globalAuthService.UpdateUser(id, username, fullName, targetUser.IsSuperAdmin, isGlobalAdmin, isProtected); err != nil {
		h.render(c, http.StatusBadRequest, "admin/user_form.html", gin.H{
			"title":       "Edit User",
			"currentPage": "users",
			"icon":        "bi-pencil",
			"error":       "Gagal update user: " + err.Error(),
		})
		return
	}

	if newPassword != "" {
		if err := h.globalAuthService.UpdateUserPassword(id, newPassword); err != nil {
			h.render(c, http.StatusBadRequest, "admin/user_form.html", gin.H{
				"title":       "Edit User",
				"currentPage": "users",
				"icon":        "bi-pencil",
				"error":       "Gagal update password: " + err.Error(),
			})
			return
		}
	}

	if isGlobalAdmin {
		perms := make([]struct {
			LabURLPath string
			Role       string
		}, len(h.cfg.Labs))
		for i, lab := range h.cfg.Labs {
			perms[i] = struct {
				LabURLPath string
				Role       string
			}{lab.URLPath, "admin"}
		}
		if err := h.globalAuthService.SetUserPermissions(id, perms); err != nil {
			h.render(c, http.StatusInternalServerError, "admin/user_form.html", gin.H{
				"title":       "Edit User",
				"currentPage": "users",
				"icon":        "bi-pencil",
				"error":       "Gagal update permissions",
			})
			return
		}
	} else {
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
			h.render(c, http.StatusInternalServerError, "admin/user_form.html", gin.H{
				"title":       "Edit User",
				"currentPage": "users",
				"icon":        "bi-pencil",
				"error":       "Gagal update permissions",
			})
			return
		}
	}

	c.Redirect(http.StatusFound, "/labs/admin/users")
}

func (h *GlobalHandler) AdminUserDelete(c *gin.Context) {
	targetUsername := c.Param("username")

	targetUser, err := h.globalAuthService.GetUserByUsername(targetUsername)
	if err != nil {
		c.AbortWithStatus(404)
		return
	}
	id := targetUser.ID

	session := sessions.Default(c)
	currentUserID, _ := session.Get("user_id").(int)
	if currentUserID == id {
		users, _ := h.globalAuthService.ListUsers()
		h.render(c, http.StatusForbidden, "admin/users.html", gin.H{
			"title":       "Manage Users",
			"currentPage": "users",
			"icon":        "bi-people",
			"error":       "Tidak dapat menghapus akun Anda sendiri",
			"users":       users,
			"filters":     map[string]string{},
		})
		return
	}

	var mainCount int
	h.globalDB.QueryRow(`SELECT COUNT(*) FROM lab_permissions WHERE user_id = ? AND is_main_account = 1`, id).Scan(&mainCount)
	if mainCount > 0 {
		users, _ := h.globalAuthService.ListUsers()
		h.render(c, http.StatusForbidden, "admin/users.html", gin.H{
			"title":       "Manage Users",
			"currentPage": "users",
			"icon":        "bi-people",
			"error":       "User ini adalah akun utama lab dan tidak bisa dihapus",
			"users":       users,
			"filters":     map[string]string{},
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
		} else if errors.Is(err, services.ErrCannotDeleteGlobalAdmin) {
			errMsg = "Tidak dapat menghapus Global Admin Biasa"
		}
		h.render(c, http.StatusForbidden, "admin/users.html", gin.H{
			"title":       "Manage Users",
			"currentPage": "users",
			"icon":        "bi-people",
			"error":       errMsg,
			"users":       users,
			"filters":     map[string]string{},
		})
		return
	}

	c.Redirect(http.StatusFound, "/labs/admin/users")
}


