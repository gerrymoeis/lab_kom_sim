package handlers

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strconv"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/services"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) UserList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 { page = 1 }
	pageSize := h.cfg.DefaultPageSize
	search := c.Query("search")
	roleFilter := c.Query("role")
	sortBy := c.Query("sort_by")
	sortOrder := c.Query("sort_order")

	values, _ := url.ParseQuery(c.Request.URL.RawQuery)
	delete(values, "page")
	values.Del("success")
	values.Del("error")
	values.Del("toast")
	var query interface{} = ""
	if len(values) > 0 { query = template.URL("&" + values.Encode()) }

	users, total, err := h.userService.ListPaginated(search, roleFilter, sortBy, sortOrder, page, pageSize, "", "", "")
	if err != nil { h.errHTML(c, "Gagal mengambil data user"); return }

	// Get main account IDs and super admin usernames from global DB
	mainAccountIDs := make(map[int]bool)
	superAdminUsernames := make(map[string]bool)
	if gdbVal, ok := c.Get("globalDB"); ok {
		if gdb, ok := gdbVal.(*database.DB); ok {
			lab := c.GetString("lab")
			rows, _ := gdb.Query(`SELECT user_id FROM lab_permissions WHERE lab_url_path = ? AND is_main_account = 1`, lab)
			if rows != nil {
				for rows.Next() {
					var id int
					rows.Scan(&id)
					mainAccountIDs[id] = true
				}
				rows.Close()
			}
			rows, _ = gdb.Query(`SELECT username FROM global_users WHERE is_super_admin = 1`)
			if rows != nil {
				for rows.Next() {
					var u string
					rows.Scan(&u)
					superAdminUsernames[u] = true
				}
				rows.Close()
			}
		}
	}

	totalPages := (total + pageSize - 1) / pageSize
	startRow := (page-1)*pageSize + 1

	// Precompute canAccess for each user using instance method (has global DB fallback)
	isSuperAdmin := h.isSuperAdmin(c)
	isMainAccount := h.isMainAccount(c)
	canAccess := make(map[int]bool)
	for i := range users {
		canAccess[users[i].ID] = h.canAccessProfile(username, &users[i], isSuperAdmin, isMainAccount)
	}

	h.renderTemplate(c, http.StatusOK, "user/list.html", gin.H{
		"title": "Manajemen User", "currentPage": "users",
		"username": username, "role": role, "users": users,
		"isSuperAdmin": isSuperAdmin,
		"isMainAccount": isMainAccount,
		"mainAccountIDs": mainAccountIDs,
		"superAdminUsernames": superAdminUsernames,
		"canAccess": canAccess,
		"page": page, "startRow": startRow, "totalPages": totalPages, "totalItems": total,
		"query": query, "filters": gin.H{"search": search, "role": roleFilter, "sort_by": sortBy, "sort_order": sortOrder},
	})
}

func (h *Handler) UserCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }
	h.renderTemplate(c, http.StatusOK, "user/create.html", gin.H{
		"title": "Tambah User Baru", "currentPage": "users",
		"username": username, "role": role,
	})
}

func (h *Handler) UserCreate(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBind(&req); err != nil {
		h.renderTemplate(c, http.StatusBadRequest, "user/create.html", gin.H{"title": "Tambah User Baru", "error": "Semua field harus diisi", "currentPage": "users"})
		return
	}

	if !h.isSuperAdmin(c) && !h.isMainAccount(c) {
		h.renderTemplate(c, http.StatusForbidden, "user/create.html", gin.H{"title": "Tambah User Baru", "error": "Hanya super admin atau akun utama yang dapat menambah user", "currentPage": "users"})
		return
	}

	// Create in global_users + add lab_permission so user can login globally
	globalUser, err := h.globalAuthService.CreateUser(req.Username, req.Password, req.FullName, false)
	if err != nil {
		h.renderTemplate(c, http.StatusInternalServerError, "user/create.html", gin.H{"title": "Tambah User Baru", "error": "Gagal membuat user. Username mungkin sudah digunakan.", "currentPage": "users"})
		return
	}
	lab := c.GetString("lab")
	h.globalAuthService.SetUserPermissions(globalUser.ID, []struct {
		LabURLPath string
		Role       string
	}{{LabURLPath: lab, Role: "admin"}})

	// Sync to per-lab users table with the same global ID
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	dbVal, _ := c.Get("db")
	if db, ok := dbVal.(*database.DB); ok {
		db.Exec(`INSERT INTO users (id, username, password, full_name, role, is_protected, is_super_admin)
			VALUES (?, ?, ?, ?, 'admin', 0, 0)
			ON CONFLICT(id) DO UPDATE SET
				username = excluded.username,
				full_name = excluded.full_name`,
			globalUser.ID, req.Username, string(hash), req.FullName)
	}

	// Activity log
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	h.activityLogService.LogCreate(uid, u, r, "user", globalUser.ID, map[string]any{
		"username": req.Username, "full_name": req.FullName, "role": "admin",
	}, ip, ua)

	h.redirectWithSuccess(c, "/admin/users", "User berhasil ditambahkan")
}

func (h *Handler) UserDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	targetUsername := c.Param("username")
	user, err := h.userService.GetByUsername(targetUsername)
	if err != nil {
		h.redirectWithError(c, "/admin/users", "User tidak ditemukan")
		return
	}

	if !h.canAccessProfile(username, user, h.isSuperAdmin(c), h.isMainAccount(c)) {
		h.redirectWithError(c, "/admin/users", "Tidak dapat mengakses profil user ini")
		return
	}

	// Check if target is main account
	targetIsMainAccount := false
	if gdbVal, ok := c.Get("globalDB"); ok {
		if gdb, ok := gdbVal.(*database.DB); ok {
			lab := c.GetString("lab")
			var count int
			gdb.QueryRow(`SELECT COUNT(*) FROM lab_permissions WHERE user_id = ? AND lab_url_path = ? AND is_main_account = 1`,
				user.ID, lab).Scan(&count)
			targetIsMainAccount = count > 0
		}
	}

	h.renderTemplate(c, http.StatusOK, "user/detail.html", gin.H{
		"title": "Detail User", "currentPage": "users",
		"username": username, "role": role, "user": user,
		"targetIsMainAccount": targetIsMainAccount,
		"error": c.Query("error"), "success": c.Query("success"),
	})
}

func (h *Handler) UserEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	targetUsername := c.Param("username")
	user, err := h.userService.GetByUsername(targetUsername)
	if err != nil {
		h.redirectWithError(c, "/admin/users", "User tidak ditemukan")
		return
	}

	if !h.canAccessProfile(username, user, h.isSuperAdmin(c), h.isMainAccount(c)) {
		h.redirectWithError(c, "/admin/users", "Tidak dapat mengakses profil user ini")
		return
	}

	// Check if target is main account
	targetIsMainAccount := false
	if gdbVal, ok := c.Get("globalDB"); ok {
		if gdb, ok := gdbVal.(*database.DB); ok {
			lab := c.GetString("lab")
			var count int
			gdb.QueryRow(`SELECT COUNT(*) FROM lab_permissions WHERE user_id = ? AND lab_url_path = ? AND is_main_account = 1`,
				user.ID, lab).Scan(&count)
			targetIsMainAccount = count > 0
		}
	}

	h.renderTemplate(c, http.StatusOK, "user/edit.html", gin.H{
		"title": "Edit User", "currentPage": "users",
		"username": username, "role": role, "user": user,
		"targetIsMainAccount": targetIsMainAccount,
		"error": c.Query("error"),
	})
}

func (h *Handler) UserEdit(c *gin.Context) {
	targetUsername := c.Param("username")
	target, err := h.userService.GetByUsername(targetUsername)
	if err != nil {
		h.redirectWithError(c, "/admin/users", "User tidak ditemukan")
		return
	}

	_, u, _, ok := h.user(c)
	if !ok { return }

	if !h.canAccessProfile(u, target, h.isSuperAdmin(c), h.isMainAccount(c)) {
		h.redirectWithError(c, "/admin/users", "Tidak dapat mengakses profil user ini")
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBind(&req); err != nil {
		h.redirectWithError(c, "/admin/users/"+targetUsername+"/edit", "Semua field harus diisi")
		return
	}

	uid, u, r, ok := h.user(c)
	if !ok { return }
	ip, ua := getRequestContext(c)

	if err := h.userService.UpdateUser(uid, target.ID, u, r, ip, ua, req.Username, req.FullName, req.Role, req.NewPassword); err != nil {
		msg := "Gagal mengupdate user"
		if errors.Is(err, services.ErrUsernameTaken) { msg = "Username sudah digunakan" }
		if errors.Is(err, services.ErrProtectedUpdate) { msg = "Tidak dapat mengubah username atau role user ini" }
		h.redirectWithError(c, "/admin/users/"+targetUsername+"/edit", msg)
		return
	}

	if u == targetUsername {
		sess := sessions.Default(c)
		sess.Set("username", req.Username)
		sess.Save()
	}

	h.redirectWithSuccess(c, "/admin/users/"+req.Username, "User berhasil diupdate", "update")
}

func (h *Handler) UserDelete(c *gin.Context) {
	targetUsername := c.Param("username")
	target, err := h.userService.GetByUsername(targetUsername)
	if err != nil {
		h.redirectWithError(c, "/admin/users", "User tidak ditemukan")
		return
	}
	sess := sessions.Default(c)
	currentUserID, _ := sess.Get("user_id").(int)
	u, _ := sess.Get("username").(string)
	r, _ := sess.Get("role").(string)
	ip, ua := getRequestContext(c)

	var targetIsMainAccount, targetIsSuperAdmin bool
	if gdbVal, exists := c.Get("globalDB"); exists {
		globalDB := gdbVal.(*database.DB)
		lab := c.GetString("lab")
		var count int
		if err := globalDB.QueryRow(
			`SELECT COUNT(*) FROM lab_permissions lp JOIN global_users gu ON gu.id = lp.user_id WHERE gu.username = ? AND lp.lab_url_path = ? AND lp.is_main_account = 1`,
			targetUsername, lab,
		).Scan(&count); err == nil && count > 0 {
			targetIsMainAccount = true
		}
		globalDB.QueryRow(`SELECT COUNT(*) FROM global_users WHERE username = ? AND is_super_admin = 1`, targetUsername).Scan(&count)
		if count > 0 {
			targetIsSuperAdmin = true
		}
	}

	if err := h.userService.DeleteUser(currentUserID, target.ID, u, r, h.isSuperAdmin(c), h.isMainAccount(c), targetIsMainAccount, targetIsSuperAdmin, ip, ua); err != nil {
		msg := "Gagal menghapus user"
		if errors.Is(err, services.ErrSelfDelete) { msg = "Tidak dapat menghapus akun sendiri" }
		if errors.Is(err, services.ErrProtectedDelete) { msg = "Tidak dapat menghapus akun admin utama" }
		if errors.Is(err, services.ErrDeleteNotAllowed) { msg = "Hanya akun utama yang dapat menghapus user lain" }
		if errors.Is(err, services.ErrUserNotFound) { msg = "User tidak ditemukan" }
		h.redirectWithError(c, "/admin/users", msg)
		return
	}
	h.redirectWithSuccess(c, "/admin/users", "User berhasil dihapus", "delete")
}

func (h *Handler) Profile(c *gin.Context) {
	userID, username, role, ok := h.user(c)
	if !ok { return }

	user, err := h.userService.GetByID(userID)
	if err != nil {
		h.redirectWithError(c, "/profile", "User tidak ditemukan")
		return
	}
	h.renderTemplate(c, http.StatusOK, "user/profile.html", gin.H{
		"title": "Profil", "currentPage": "profile",
		"username": username, "role": role, "user": user,
		"success": c.Query("success"), "error": c.Query("error"),
	})
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
	if err := c.ShouldBind(&req); err != nil {
		h.redirectWithError(c, "/profile", "Username dan Nama Lengkap harus diisi")
		return
	}
	userID, u, r, ok := h.user(c)
	if !ok { return }
	ip, ua := getRequestContext(c)

	newUsername, newFullName, err := h.userService.UpdateProfile(userID, req.Username, req.FullName, u, r, ip, ua)
	if err != nil {
		msg := "Gagal mengupdate profil"
		if errors.Is(err, services.ErrUsernameTaken) { msg = "Username sudah digunakan" }
		h.redirectWithError(c, "/profile", msg)
		return
	}

	sess := sessions.Default(c)
	oldUsername, _ := sess.Get("username").(string)
	sess.Set("username", newUsername)
	sess.Set("full_name", newFullName)
	sess.Save()

	if oldUsername != "" && oldUsername != newUsername {
		_ = h.globalAuthService.ClearDefaultPasswordFlag(userID)
	}

	h.redirectWithSuccess(c, "/profile", "Profil berhasil diupdate", "update")
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBind(&req); err != nil {
		h.redirectWithError(c, "/profile", "Semua field harus diisi")
		return
	}
	userID, u, r, ok := h.user(c)
	if !ok { return }
	ip, ua := getRequestContext(c)

	if err := h.userService.ChangePassword(userID, req.OldPassword, req.NewPassword, req.ConfirmPassword, u, r, ip, ua); err != nil {
		msg := "Gagal mengubah password"
		if errors.Is(err, services.ErrPasswordMismatch) { msg = "Password baru dan konfirmasi tidak cocok" }
		if errors.Is(err, services.ErrWrongPassword) { msg = "Password lama salah" }
		h.redirectWithError(c, "/profile", msg)
		return
	}
	h.globalAuthService.ClearDefaultPasswordFlag(userID)
	h.redirectWithSuccess(c, "/profile", "Password berhasil diubah", "update")
}

func (h *Handler) UserBatchDelete(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		h.errJSON(c, http.StatusBadRequest, "Tidak ada item yang dipilih")
		return
	}
	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	lab := c.GetString("lab")
	targetMainAccountUsernames := make(map[string]bool)
	targetSuperAdminUsernames := make(map[string]bool)
	if gdbVal, exists := c.Get("globalDB"); exists {
		globalDB := gdbVal.(*database.DB)
		for _, username := range req.IDs {
			var count int
			if err := globalDB.QueryRow(
				`SELECT COUNT(*) FROM lab_permissions lp JOIN global_users gu ON gu.id = lp.user_id WHERE gu.username = ? AND lp.lab_url_path = ? AND lp.is_main_account = 1`,
				username, lab,
			).Scan(&count); err == nil && count > 0 {
				targetMainAccountUsernames[username] = true
			}
			globalDB.QueryRow(`SELECT COUNT(*) FROM global_users WHERE username = ? AND is_super_admin = 1`, username).Scan(&count)
			if count > 0 {
				targetSuperAdminUsernames[username] = true
			}
		}
	}

	if err := h.userService.BatchDeleteUser(uid, req.IDs, u, r, h.isSuperAdmin(c), h.isMainAccount(c), targetMainAccountUsernames, targetSuperAdminUsernames, ip, ua); err != nil {
		h.errJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User berhasil dihapus"})
}
