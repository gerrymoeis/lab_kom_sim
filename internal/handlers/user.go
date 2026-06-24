package handlers

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strconv"

	"inventaris-lab-kom/internal/database"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrSelfDelete       = errors.New("tidak dapat menghapus akun sendiri")
	ErrProtectedDelete  = errors.New("tidak dapat menghapus akun admin utama")
	ErrDeleteNotAllowed = errors.New("hanya akun utama yang dapat menghapus user lain")
	ErrUserNotFound     = errors.New("user tidak ditemukan")
	ErrUsernameTaken    = errors.New("username sudah digunakan")
	ErrPasswordMismatch = errors.New("password baru dan konfirmasi tidak cocok")
	ErrWrongPassword    = errors.New("password lama salah")
)

func (h *Handler) UserList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
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
	if len(values) > 0 {
		query = template.URL("&" + values.Encode())
	}

	lab := c.GetString("lab")
	users, total, err := h.globalAuthService.ListUsersByLab(lab, search, roleFilter, sortBy, sortOrder, page, pageSize)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data user")
		return
	}

	mainAccountIDs := make(map[int]bool)
	superAdminUsernames := make(map[string]bool)
	if gdbVal, ok := c.Get("globalDB"); ok {
		if gdb, ok := gdbVal.(*database.DB); ok {
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

	isSuperAdmin := h.isSuperAdmin(c)
	isMainAccount := h.isMainAccount(c)
	isProtected := h.isProtected(c)
	isGlobalAdmin := h.isGlobalAdmin(c)
	canAccess := make(map[int]bool)
	for i := range users {
		targetIsMainAccount := mainAccountIDs[users[i].ID]
		canAccess[users[i].ID] = h.canAccessProfile(username, &users[i], isSuperAdmin, isMainAccount, targetIsMainAccount, isProtected, isGlobalAdmin)
	}

	h.renderTemplate(c, http.StatusOK, "user/list.html", gin.H{
		"title": "Manajemen User", "currentPage": "users",
		"username": username, "role": role, "users": users,
		"isSuperAdmin":       isSuperAdmin,
		"isMainAccount":      isMainAccount,
		"mainAccountIDs":     mainAccountIDs,
		"superAdminUsernames": superAdminUsernames,
		"canAccess":          canAccess,
		"page":               page, "startRow": startRow, "totalPages": totalPages, "totalItems": total,
		"query": query, "filters": gin.H{"search": search, "role": roleFilter, "sort_by": sortBy, "sort_order": sortOrder},
	})
}

func (h *Handler) UserCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}
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

	if !h.isSuperAdmin(c) && !h.isMainAccount(c) && !h.isGlobalAdmin(c) {
		h.renderTemplate(c, http.StatusForbidden, "user/create.html", gin.H{"title": "Tambah User Baru", "error": "Hanya super admin atau akun utama yang dapat menambah user", "currentPage": "users"})
		return
	}

	globalUser, err := h.globalAuthService.CreateUser(req.Username, req.Password, req.FullName, false, false)
	if err != nil {
		h.renderTemplate(c, http.StatusInternalServerError, "user/create.html", gin.H{"title": "Tambah User Baru", "error": "Gagal membuat user. Username mungkin sudah digunakan.", "currentPage": "users"})
		return
	}
	lab := c.GetString("lab")
	h.globalAuthService.SetUserPermissions(globalUser.ID, []struct {
		LabURLPath string
		Role       string
	}{{LabURLPath: lab, Role: "admin"}})

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	h.activityLogService.LogCreate(uid, u, r, "user", globalUser.ID, map[string]any{
		"username": req.Username, "full_name": req.FullName, "role": "admin",
	}, ip, ua)

	h.redirectWithSuccess(c, "/admin/users", "User berhasil ditambahkan")
}

func (h *Handler) UserDetail(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	targetUsername := c.Param("username")
	lab := c.GetString("lab")
	user, err := h.globalAuthService.GetUserByUsernameAndLab(targetUsername, lab)
	if err != nil {
		h.redirectWithError(c, "/admin/users", "User tidak ditemukan")
		return
	}

	targetIsMainAccount := false
	if gdbVal, ok := c.Get("globalDB"); ok {
		if gdb, ok := gdbVal.(*database.DB); ok {
			var count int
			gdb.QueryRow(`SELECT COUNT(*) FROM lab_permissions WHERE user_id = ? AND lab_url_path = ? AND is_main_account = 1`,
				user.ID, lab).Scan(&count)
			targetIsMainAccount = count > 0
		}
	}

	if !h.canAccessProfile(username, user, h.isSuperAdmin(c), h.isMainAccount(c), targetIsMainAccount, h.isProtected(c), h.isGlobalAdmin(c)) {
		h.redirectWithError(c, "/admin/users", "Tidak dapat mengakses profil user ini")
		return
	}

	h.renderTemplate(c, http.StatusOK, "user/detail.html", gin.H{
		"title": "Detail User", "currentPage": "users",
		"username": username, "role": role, "user": user,
		"targetIsMainAccount": targetIsMainAccount,
		"error":               c.Query("error"), "success": c.Query("success"),
	})
}

func (h *Handler) UserEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok {
		return
	}

	targetUsername := c.Param("username")
	lab := c.GetString("lab")
	user, err := h.globalAuthService.GetUserByUsernameAndLab(targetUsername, lab)
	if err != nil {
		h.redirectWithError(c, "/admin/users", "User tidak ditemukan")
		return
	}

	targetIsMainAccount := false
	if gdbVal, ok := c.Get("globalDB"); ok {
		if gdb, ok := gdbVal.(*database.DB); ok {
			var count int
			gdb.QueryRow(`SELECT COUNT(*) FROM lab_permissions WHERE user_id = ? AND lab_url_path = ? AND is_main_account = 1`,
				user.ID, lab).Scan(&count)
			targetIsMainAccount = count > 0
		}
	}

	if !h.canAccessProfile(username, user, h.isSuperAdmin(c), h.isMainAccount(c), targetIsMainAccount, h.isProtected(c), h.isGlobalAdmin(c)) {
		h.redirectWithError(c, "/admin/users", "Tidak dapat mengakses profil user ini")
		return
	}

	h.renderTemplate(c, http.StatusOK, "user/edit.html", gin.H{
		"title": "Edit User", "currentPage": "users",
		"username": username, "role": role, "user": user,
		"targetIsMainAccount": targetIsMainAccount,
		"error":               c.Query("error"),
	})
}

func (h *Handler) UserEdit(c *gin.Context) {
	targetUsername := c.Param("username")
	lab := c.GetString("lab")
	target, err := h.globalAuthService.GetUserByUsernameAndLab(targetUsername, lab)
	if err != nil {
		h.redirectWithError(c, "/admin/users", "User tidak ditemukan")
		return
	}

	_, u, _, ok := h.user(c)
	if !ok {
		return
	}

	targetIsMainAccount := false
	if gdbVal, ok := c.Get("globalDB"); ok {
		if gdb, ok := gdbVal.(*database.DB); ok {
			var count int
			gdb.QueryRow(`SELECT COUNT(*) FROM lab_permissions WHERE user_id = ? AND lab_url_path = ? AND is_main_account = 1`,
				target.ID, lab).Scan(&count)
			targetIsMainAccount = count > 0
		}
	}

	if !h.canAccessProfile(u, target, h.isSuperAdmin(c), h.isMainAccount(c), targetIsMainAccount, h.isProtected(c), h.isGlobalAdmin(c)) {
		h.redirectWithError(c, "/admin/users", "Tidak dapat mengakses profil user ini")
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBind(&req); err != nil {
		h.redirectWithError(c, "/admin/users/"+targetUsername+"/edit", "Semua field harus diisi")
		return
	}

	if err := h.globalAuthService.UpdateUser(target.ID, req.Username, req.FullName, target.IsSuperAdmin, target.IsGlobalAdmin); err != nil {
		h.redirectWithError(c, "/admin/users/"+targetUsername+"/edit", "Gagal mengupdate user")
		return
	}

	if req.NewPassword != "" {
		if err := h.globalAuthService.UpdateUserPassword(target.ID, req.NewPassword); err != nil {
			h.redirectWithError(c, "/admin/users/"+targetUsername+"/edit", "Gagal mengupdate password")
			return
		}
	}

	uid, u, r, _ := h.user(c)
	ip, ua := getRequestContext(c)
	oldVals := map[string]any{}
	newVals := map[string]any{}
	if target.Username != req.Username {
		oldVals["username"] = target.Username
		newVals["username"] = req.Username
	}
	if target.FullName != req.FullName {
		oldVals["full_name"] = target.FullName
		newVals["full_name"] = req.FullName
	}
	if req.NewPassword != "" {
		oldVals["password_changed"] = false
		newVals["password_changed"] = true
	}
	if len(oldVals) > 0 || len(newVals) > 0 {
		h.activityLogService.LogUpdate(uid, u, r, "user", target.ID, oldVals, newVals, ip, ua)
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
	lab := c.GetString("lab")
	target, err := h.globalAuthService.GetUserByUsernameAndLab(targetUsername, lab)
	if err != nil {
		h.redirectWithError(c, "/admin/users", "User tidak ditemukan")
		return
	}
	sess := sessions.Default(c)
	currentUserID, _ := sess.Get("user_id").(int)
	u, _ := sess.Get("username").(string)
	r, _ := sess.Get("role").(string)
	ip, ua := getRequestContext(c)

	if currentUserID == target.ID {
		h.redirectWithError(c, "/admin/users", ErrSelfDelete.Error())
		return
	}
	if u == targetUsername {
		h.redirectWithError(c, "/admin/users", ErrSelfDelete.Error())
		return
	}
	if target.IsProtected {
		h.redirectWithError(c, "/admin/users", ErrProtectedDelete.Error())
		return
	}

	var targetIsMainAccount, targetIsSuperAdmin bool
	if gdbVal, exists := c.Get("globalDB"); exists {
		globalDB := gdbVal.(*database.DB)
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

	if targetIsSuperAdmin {
		h.redirectWithError(c, "/admin/users", "Tidak dapat menghapus super admin")
		return
	}
	if targetIsMainAccount && !h.isProtected(c) {
		h.redirectWithError(c, "/admin/users", "Tidak dapat menghapus akun utama lab")
		return
	}
	if !h.isSuperAdmin(c) && !h.isMainAccount(c) && !h.isGlobalAdmin(c) {
		h.redirectWithError(c, "/admin/users", ErrDeleteNotAllowed.Error())
		return
	}

	if err := h.globalAuthService.RemoveLabPermission(target.ID, lab); err != nil {
		h.activityLogService.LogDelete(currentUserID, u, r, "user", target.ID,
			map[string]any{"deleted_username": target.Username}, ip, ua, err.Error())
		h.redirectWithError(c, "/admin/users", "Gagal menghapus user")
		return
	}

	h.activityLogService.LogDelete(currentUserID, u, r, "user", target.ID,
		map[string]any{"deleted_username": target.Username}, ip, ua)
	h.redirectWithSuccess(c, "/admin/users", "User berhasil dihapus", "delete")
}

func (h *Handler) Profile(c *gin.Context) {
	userID, username, role, ok := h.user(c)
	if !ok {
		return
	}

	user, err := h.globalAuthService.GetUser(userID)
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
	if !ok {
		return
	}
	ip, ua := getRequestContext(c)

	exists, _ := h.globalAuthService.GetUserByUsername(req.Username)
	if exists != nil && exists.ID != userID {
		h.redirectWithError(c, "/profile", ErrUsernameTaken.Error())
		return
	}

	currentUser, err := h.globalAuthService.GetUser(userID)
	if err != nil {
		h.redirectWithError(c, "/profile", "Sesi tidak valid, silakan login ulang")
		return
	}

	if err := h.globalAuthService.UpdateUser(userID, req.Username, req.FullName, currentUser.IsSuperAdmin, currentUser.IsGlobalAdmin); err != nil {
		h.redirectWithError(c, "/profile", "Gagal sinkronisasi profil, coba lagi")
		return
	}
	if u != req.Username {
		_ = h.globalAuthService.ClearDefaultPasswordFlag(userID)
	}

	oldVals := map[string]any{}
	newVals := map[string]any{}
	if currentUser.Username != req.Username {
		oldVals["username"] = currentUser.Username
		newVals["username"] = req.Username
	}
	if currentUser.FullName != req.FullName {
		oldVals["full_name"] = currentUser.FullName
		newVals["full_name"] = req.FullName
	}
	if len(oldVals) > 0 || len(newVals) > 0 {
		h.activityLogService.LogUpdate(userID, u, r, "user", userID, oldVals, newVals, ip, ua)
	}

	sess := sessions.Default(c)
	oldUsername, _ := sess.Get("username").(string)
	sess.Set("username", req.Username)
	sess.Set("full_name", req.FullName)
	sess.Save()

	if oldUsername != req.Username {
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
	if !ok {
		return
	}
	ip, ua := getRequestContext(c)

	if req.NewPassword != req.ConfirmPassword {
		h.activityLogService.LogAction(userID, u, r, "update", "user", userID,
			map[string]any{"password_changed": true}, map[string]any{"password_changed": false},
			ip, ua, ErrPasswordMismatch.Error())
		h.redirectWithError(c, "/profile", ErrPasswordMismatch.Error())
		return
	}

	user, err := h.globalAuthService.GetUser(userID)
	if err != nil {
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)) != nil {
		h.activityLogService.LogAction(userID, u, r, "update", "user", userID,
			map[string]any{"password_changed": true}, map[string]any{"password_changed": false},
			ip, ua, ErrWrongPassword.Error())
		h.redirectWithError(c, "/profile", ErrWrongPassword.Error())
		return
	}

	if err := h.globalAuthService.UpdateUserPassword(userID, req.NewPassword); err != nil {
		h.activityLogService.LogAction(userID, u, r, "update", "user", userID,
			map[string]any{"password_changed": true}, map[string]any{"password_changed": false},
			ip, ua, err.Error())
		h.redirectWithError(c, "/profile", "Gagal mengubah password")
		return
	}

	h.activityLogService.LogAction(userID, u, r, "update", "user", userID,
		map[string]any{"password_changed": true}, map[string]any{"password_changed": true},
		ip, ua)
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

	items := make([]map[string]string, 0, len(req.IDs))
	for _, username := range req.IDs {
		if u == username {
			h.activityLogService.LogDelete(uid, u, r, "user", 0,
				map[string]any{"action": "batch_delete", "count": len(req.IDs), "items": items},
				ip, ua, ErrSelfDelete.Error())
			h.errJSON(c, http.StatusInternalServerError, ErrSelfDelete.Error())
			return
		}
		target, err := h.globalAuthService.GetUserByUsernameAndLab(username, lab)
		if err != nil {
			h.activityLogService.LogDelete(uid, u, r, "user", 0,
				map[string]any{"action": "batch_delete", "count": len(req.IDs), "items": items},
				ip, ua, "user "+username+" not found")
			h.errJSON(c, http.StatusInternalServerError, "User "+username+" tidak ditemukan")
			return
		}
		if target.IsProtected {
			h.activityLogService.LogDelete(uid, u, r, "user", 0,
				map[string]any{"action": "batch_delete", "count": len(req.IDs), "items": items},
				ip, ua, ErrProtectedDelete.Error())
			h.errJSON(c, http.StatusInternalServerError, ErrProtectedDelete.Error())
			return
		}
		if targetSuperAdminUsernames[username] {
			h.activityLogService.LogDelete(uid, u, r, "user", 0,
				map[string]any{"action": "batch_delete", "count": len(req.IDs), "items": items},
				ip, ua, "tidak dapat menghapus super admin")
			h.errJSON(c, http.StatusInternalServerError, "Tidak dapat menghapus super admin")
			return
		}
		if targetMainAccountUsernames[username] {
			h.activityLogService.LogDelete(uid, u, r, "user", 0,
				map[string]any{"action": "batch_delete", "count": len(req.IDs), "items": items},
				ip, ua, "tidak dapat menghapus akun utama lab")
			h.errJSON(c, http.StatusInternalServerError, "Tidak dapat menghapus akun utama lab")
			return
		}
		if !h.isSuperAdmin(c) && !h.isMainAccount(c) && !h.isGlobalAdmin(c) {
			h.activityLogService.LogDelete(uid, u, r, "user", 0,
				map[string]any{"action": "batch_delete", "count": len(req.IDs), "items": items},
				ip, ua, ErrDeleteNotAllowed.Error())
			h.errJSON(c, http.StatusInternalServerError, ErrDeleteNotAllowed.Error())
			return
		}
		info := map[string]string{"username": target.Username, "full_name": target.FullName}
		if err := h.globalAuthService.RemoveLabPermission(target.ID, lab); err != nil {
			h.activityLogService.LogDelete(uid, u, r, "user", 0,
				map[string]any{"action": "batch_delete", "count": len(req.IDs), "items": items},
				ip, ua, err.Error())
			h.errJSON(c, http.StatusInternalServerError, "Gagal menghapus user")
			return
		}
		items = append(items, info)
	}
	h.activityLogService.LogDelete(uid, u, r, "user", 0,
		map[string]any{"action": "batch_delete", "count": len(req.IDs), "items": items},
		ip, ua)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User berhasil dihapus"})
}
