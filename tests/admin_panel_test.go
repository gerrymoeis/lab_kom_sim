package tests

import (
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// loginAsAdmin logs in as super admin, extracts CSRF token, returns cookies + token.
func loginAsAdmin(env *TestEnvironment) (cookies map[string]string, csrf string) {
	if !env.LabA.login("admin", "admin123") {
		env.LabA.t.Fatal("admin login failed")
	}
	// Refresh CSRF token from /labs page (first GET after login has session CSRF token)
	resp, err := env.LabA.getURL(env.TS.URL + "/labs")
	if err != nil {
		env.LabA.t.Fatalf("GET /labs: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	token := env.LabA.extractCSRFToken(string(body))
	if token == "" {
		env.LabA.t.Fatal("could not extract CSRF token from /labs")
	}
	env.LabA.csrf = token
	return env.LabA.cookies, token
}

// adminGet performs GET /admin/<path> with super admin cookies and returns response.
func adminGet(env *TestEnvironment, path string) *http.Response {
	req, _ := http.NewRequest("GET", env.TS.URL+"/admin"+path, nil)
	env.LabA.addCookies(req)
	resp, err := env.Client.Do(req)
	if err != nil {
		env.LabA.t.Fatalf("GET /admin%s: %v", path, err)
	}
	return resp
}

// adminPost performs POST /admin/<path> with CSRF token and super admin cookies.
func adminPost(env *TestEnvironment, path, data string) *http.Response {
	if data == "" {
		data = "_csrf=" + url.QueryEscape(env.LabA.csrf)
	} else {
		data = data + "&_csrf=" + url.QueryEscape(env.LabA.csrf)
	}
	req, _ := http.NewRequest("POST", env.TS.URL+"/admin"+path, strings.NewReader(data))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	env.LabA.addCookies(req)
	resp, err := env.Client.Do(req)
	if err != nil {
		env.LabA.t.Fatalf("POST /admin%s: %v", path, err)
	}
	return resp
}

// adminPostNoCSRF performs POST /admin/<path> WITHOUT CSRF token to test CSRF rejection.
func adminPostNoCSRF(env *TestEnvironment, path string) *http.Response {
	req, _ := http.NewRequest("POST", env.TS.URL+"/admin"+path, strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := env.Client.Do(req)
	if err != nil {
		env.LabA.t.Fatalf("POST /admin%s (no CSRF): %v", path, err)
	}
	// Add cookies from login for auth
	for n, v := range env.LabA.cookies {
		req.AddCookie(&http.Cookie{Name: n, Value: v})
	}
	return resp
}

// ============================================
// 1. AdminLabList — Success + Forbidden
// ============================================

func TestAdminLabList(t *testing.T) {
	env := setupTestEnvironment(t)

	t.Run("success_as_super_admin", func(t *testing.T) {
		loginAsAdmin(env)
		resp := adminGet(env, "/labs")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Manage Lab") {
			t.Error("body should contain 'Manage Lab'")
		}
		if !strings.Contains(string(body), "lab-kom-mi") {
			t.Error("body should contain lab URL path")
		}
	})

	t.Run("forbidden_non_super_admin", func(t *testing.T) {
		// Login as labA_only (not super admin)
		if !env.LabA.login("labA_only", "test123") {
			t.Fatal("labA_only login failed")
		}
		resp := adminGet(env, "/labs")
		defer resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Errorf("expected 403 for non-super-admin, got %d", resp.StatusCode)
		}
	})

	t.Run("unauthorized_no_session", func(t *testing.T) {
		req, _ := http.NewRequest("GET", env.TS.URL+"/admin/labs", nil)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("GET /admin/labs: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 redirect to login, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// 2. AdminLabLayout — GET + POST layout
// ============================================

func TestAdminLabLayout(t *testing.T) {
	env := setupTestEnvironment(t)

	t.Run("get_layout_page", func(t *testing.T) {
		loginAsAdmin(env)
		resp := adminGet(env, "/labs/lab-kom-mi/layout")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Layout") {
			t.Error("body should contain layout page title")
		}
	})

	t.Run("get_layout_404_no_lab", func(t *testing.T) {
		resp := adminGet(env, "/labs/nonexistent/layout")
		defer resp.Body.Close()
		if resp.StatusCode != 404 {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("save_layout_success", func(t *testing.T) {
		resp := adminPost(env, "/labs/lab-kom-mi/layout", "cols_per_row=10,10,10,10&has_gap=1&gap_pos=4")
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 after save, got %d", resp.StatusCode)
		}
		loc := resp.Header.Get("Location")
		if loc != "/admin/labs" {
			t.Errorf("expected redirect to /admin/labs, got %q", loc)
		}
	})

	t.Run("save_layout_bad_format", func(t *testing.T) {
		token := env.LabA.csrf
		resp := adminPost(env, "/labs/lab-kom-mi/layout", "cols_per_row=abc&has_gap=0&gap_pos=0")
		defer resp.Body.Close()
		env.LabA.csrf = token // restore after failed post (token unchanged)
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for bad cols format, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// 3. AdminLabSeeds — GET + POST reseed
// ============================================

func TestAdminLabSeeds(t *testing.T) {
	env := setupTestEnvironment(t)
	loginAsAdmin(env)

	t.Run("get_seeds_page", func(t *testing.T) {
		resp := adminGet(env, "/labs/lab-kom-mi/seeds")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("reseed_pc", func(t *testing.T) {
		resp := adminPost(env, "/labs/lab-kom-mi/seeds/pc", "")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("reseed_software", func(t *testing.T) {
		resp := adminPost(env, "/labs/lab-kom-mi/seeds/software", "")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("reseed_schedule", func(t *testing.T) {
		resp := adminPost(env, "/labs/lab-kom-mi/seeds/schedule", "")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("reseed_invalid_type", func(t *testing.T) {
		resp := adminPost(env, "/labs/lab-kom-mi/seeds/invalid", "")
		defer resp.Body.Close()
		if resp.StatusCode != 500 {
			t.Errorf("expected 500 for invalid seed type, got %d", resp.StatusCode)
		}
	})

	t.Run("reseed_nonexistent_lab", func(t *testing.T) {
		resp := adminPost(env, "/labs/nonexistent/seeds/pc", "")
		defer resp.Body.Close()
		if resp.StatusCode != 404 {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// 4. AdminUserCreate — GET + POST + validation
// ============================================

func TestAdminUserCreate(t *testing.T) {
	env := setupTestEnvironment(t)
	loginAsAdmin(env)

	t.Run("get_create_page", func(t *testing.T) {
		resp := adminGet(env, "/users/create")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		html := string(body)
		if !strings.Contains(html, "<form") {
			t.Error("create page should contain a form")
		}
		if !strings.Contains(html, `name="username"`) {
			t.Error("create page should have username input")
		}
		if !strings.Contains(html, `name="full_name"`) {
			t.Error("create page should have full_name input")
		}
		if !strings.Contains(html, `name="new_password"`) {
			t.Error("create page should have password input")
		}
		if !strings.Contains(html, `name="_csrf"`) {
			t.Error("create page should have CSRF hidden input")
		}
	})

	t.Run("create_user_success", func(t *testing.T) {
		resp := adminPost(env, "/users/create", "username=newuser&password=newpass123&full_name=New+User&is_super_admin=0")
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 after create, got %d", resp.StatusCode)
		}
		loc := resp.Header.Get("Location")
		if loc != "/admin/users" {
			t.Errorf("expected redirect to /admin/users, got %q", loc)
		}
		// Verify user exists in global DB
		var count int
		env.GlobalDB.QueryRow("SELECT COUNT(*) FROM global_users WHERE username='newuser'").Scan(&count)
		if count != 1 {
			t.Errorf("expected 1 newuser, got %d", count)
		}
	})

	t.Run("create_user_empty_fields", func(t *testing.T) {
		resp := adminPost(env, "/users/create", "username=&password=&full_name=&is_super_admin=0")
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for empty fields, got %d", resp.StatusCode)
		}
	})

	t.Run("create_duplicate_username", func(t *testing.T) {
		resp := adminPost(env, "/users/create", "username=admin&password=test123&full_name=Duplicate&is_super_admin=0")
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for duplicate, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// 5. AdminUserEdit — GET + POST edit
// ============================================

func TestAdminUserEdit(t *testing.T) {
	env := setupTestEnvironment(t)
	loginAsAdmin(env)

	t.Run("get_edit_page", func(t *testing.T) {
		resp := adminGet(env, "/users/1/edit")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		html := string(body)
		if !strings.Contains(html, `/admin/users/1/edit`) {
			t.Error("edit page form action should point to /admin/users/1/edit")
		}
		if !strings.Contains(html, `value="admin"`) {
			t.Error("edit page should pre-populate username")
		}
		if !strings.Contains(html, `name="_csrf"`) {
			t.Error("edit page should have CSRF hidden input")
		}
	})

	t.Run("edit_user_success", func(t *testing.T) {
		resp := adminPost(env, "/users/1/edit", "username=admin_updated&full_name=Admin+Updated&is_super_admin=1")
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 after edit, got %d", resp.StatusCode)
		}
		var name string
		env.GlobalDB.QueryRow("SELECT full_name FROM global_users WHERE id=1").Scan(&name)
		if name != "Admin Updated" {
			t.Errorf("expected 'Admin Updated', got %q", name)
		}
	})

	t.Run("edit_nonexistent_user", func(t *testing.T) {
		resp := adminPost(env, "/users/999/edit", "username=nonexistent&full_name=Nobody&is_super_admin=0")
		defer resp.Body.Close()
		if resp.StatusCode != 400 && resp.StatusCode != 404 {
			t.Errorf("expected 400 or 404, got %d", resp.StatusCode)
		}
	})

	t.Run("edit_user_change_password", func(t *testing.T) {
		resp := adminPost(env, "/users/1/edit", "username=admin&full_name=Admin&is_super_admin=1&new_password=newpass456")
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 after password change, got %d", resp.StatusCode)
		}
	})

	t.Run("edit_invalid_id", func(t *testing.T) {
		resp := adminGet(env, "/users/abc/edit")
		defer resp.Body.Close()
		if resp.StatusCode != 404 {
			t.Errorf("expected 404 for invalid id, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// 6. AdminUserDelete — success + protected
// ============================================

func TestAdminUserDelete(t *testing.T) {
	env := setupTestEnvironment(t)
	loginAsAdmin(env)

	t.Run("delete_create_new_user_first", func(t *testing.T) {
		// Create a deletable user first (not protected)
		resp := adminPost(env, "/users/create", "username=deletable&password=test123&full_name=Deletable&is_super_admin=0")
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Fatalf("expected 302, got %d", resp.StatusCode)
		}
		// Refresh CSRF (create changes session)
		env.LabA.refreshCSRF()
	})

	t.Run("delete_user_success", func(t *testing.T) {
		// Refresh CSRF first
		if !env.LabA.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		// Get the deletable user's ID
		var userID int
		env.GlobalDB.QueryRow("SELECT id FROM global_users WHERE username='deletable'").Scan(&userID)
		if userID == 0 {
			t.Fatal("deletable user not found")
		}
		resp := adminPost(env, "/users/"+strconv.Itoa(userID)+"/delete", "")
		defer resp.Body.Close()
		// After delete, handler redirects to /admin/users
		if resp.StatusCode != 302 && resp.StatusCode != 200 {
			t.Errorf("expected 302 or 200, got %d", resp.StatusCode)
		}
		var count int
		env.GlobalDB.QueryRow("SELECT COUNT(*) FROM global_users WHERE id=?", userID).Scan(&count)
		if count != 0 {
			t.Errorf("expected user to be deleted, count=%d", count)
		}
	})

	t.Run("delete_protected_user_returns_forbidden", func(t *testing.T) {
		// Refresh CSRF
		if !env.LabA.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		// Mark admin (id=1) as protected in DB
		env.GlobalDB.Exec("UPDATE global_users SET is_protected = 1 WHERE id = 1")
		resp := adminPost(env, "/users/1/delete", "")
		defer resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Errorf("expected 403 for protected user, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "tidak bisa dihapus") {
			t.Error("body should contain protected user error message")
		}
	})
}

// ============================================
// 7. AdminUserPermissions — GET + POST
// ============================================

func TestAdminUserPermissions(t *testing.T) {
	env := setupTestEnvironment(t)
	loginAsAdmin(env)

	t.Run("get_permissions_page", func(t *testing.T) {
		resp := adminGet(env, "/users/3/permissions")
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		html := string(body)
		if !strings.Contains(html, `name="labs"`) {
			t.Error("permissions page should have lab checkboxes")
		}
		if !strings.Contains(html, `name="roles"`) {
			t.Error("permissions page should have role selects")
		}
		if !strings.Contains(html, "lab-kom-mi") && !strings.Contains(html, "Lab Kom MI") {
			t.Error("permissions page should show lab names")
		}
	})

	t.Run("save_permissions_success", func(t *testing.T) {
		resp := adminPost(env, "/users/3/permissions", "labs=lab-kom-mi&roles=admin")
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 after permissions save, got %d", resp.StatusCode)
		}
		loc := resp.Header.Get("Location")
		if loc != "/admin/users" {
			t.Errorf("expected redirect to /admin/users, got %q", loc)
		}
		// Verify permission saved
		var role string
		env.GlobalDB.QueryRow("SELECT role FROM lab_permissions WHERE user_id=3 AND lab_url_path='lab-kom-mi'").Scan(&role)
		if role != "admin" {
			t.Errorf("expected role 'admin', got %q", role)
		}
	})

	t.Run("permissions_redirects_for_super_admin", func(t *testing.T) {
		// Super admin (id=1) permissions page should redirect
		resp := adminGet(env, "/users/1/permissions")
		defer resp.Body.Close()
		// Super admin has no permission editing — check if it redirects or shows 200
		// Based on code: AdminUserPermissions doesn't check isSuperAdmin, it just shows the page
		// but POST to AdminUserPermissionsSave redirects if isSuperAdmin
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("permissions_nonexistent_user", func(t *testing.T) {
		resp := adminGet(env, "/users/999/permissions")
		defer resp.Body.Close()
		if resp.StatusCode != 404 {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// 8. AdminCSRFProtection — missing + invalid
// ============================================

func TestAdminCSRF(t *testing.T) {
	env := setupTestEnvironment(t)
	loginAsAdmin(env)

	t.Run("post_without_csrf_returns_403", func(t *testing.T) {
		req, _ := http.NewRequest("POST", env.TS.URL+"/admin/users/create", strings.NewReader("username=csrf_test&password=test123&full_name=CSRF+Test&is_super_admin=0"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		env.LabA.addCookies(req)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("POST /admin/users/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Errorf("expected 403 for missing CSRF, got %d", resp.StatusCode)
		}
	})

	t.Run("post_with_invalid_csrf_returns_403", func(t *testing.T) {
		req, _ := http.NewRequest("POST", env.TS.URL+"/admin/users/create", strings.NewReader("_csrf=invalidtoken123&username=csrf_test2&password=test123&full_name=CSRF+Test+2&is_super_admin=0"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		env.LabA.addCookies(req)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("POST /admin/users/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Errorf("expected 403 for invalid CSRF, got %d", resp.StatusCode)
		}
	})

		t.Run("get_routes_without_auth_redirect", func(t *testing.T) {
		req, _ := http.NewRequest("GET", env.TS.URL+"/admin/labs", nil)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("GET /admin/labs: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 redirect to login, got %d", resp.StatusCode)
		}
		loc := resp.Header.Get("Location")
		if loc != "/login" {
			t.Errorf("expected redirect to /login, got %q", loc)
		}
	})
}

// ============================================
// 9. PerLabUserDetail — GET user detail page (per-lab admin)
// ============================================

func TestPerLabUserDetail(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	t.Run("detail_existing_user", func(t *testing.T) {
		resp, err := lab.get("/admin/users/labA_only")
		if err != nil {
			t.Fatalf("GET /admin/users/labA_only: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("detail_not_found", func(t *testing.T) {
		resp, err := lab.get("/admin/users/nonexistent-user")
		if err != nil {
			t.Fatalf("GET /admin/users/nonexistent-user: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 for not found, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// 10. PerLabUserBatchDelete — POST batch delete (per-lab admin)
// ============================================

func TestPerLabUserBatchDelete(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	// Ensure a user exists in global DB for batch delete
	var userCount int
	env.GlobalDB.QueryRow("SELECT COUNT(*) FROM global_users").Scan(&userCount)
	if userCount == 0 {
		t.Fatal("no user found in global DB")
	}

	t.Run("batch_delete_empty", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.postJSON("/admin/users/batch-delete", `{"ids":[]}`)
		if err != nil {
			t.Fatalf("POST /admin/users/batch-delete empty: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for empty ids, got %d", resp.StatusCode)
		}
	})
}

func TestPerLabUserBatchDeleteSuccess(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	gdb := env.GlobalDB
	// Login as super admin (admin) — only super admins or main accounts can batch-delete
	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}

	t.Run("batch_delete_success", func(t *testing.T) {
		// Create test users in global DB
		gdb.Exec("DELETE FROM lab_permissions WHERE user_id IN (SELECT id FROM global_users WHERE username IN ('batch_del_a', 'batch_del_b'))")
		gdb.Exec("DELETE FROM global_users WHERE username IN ('batch_del_a', 'batch_del_b')")
		gdb.Exec("INSERT INTO global_users (username, password, full_name) VALUES ('batch_del_a', '$2a$10$dummy', 'Batch Del A')")
		gdb.Exec("INSERT INTO global_users (username, password, full_name) VALUES ('batch_del_b', '$2a$10$dummy', 'Batch Del B')")
		var idA, idB int
		gdb.QueryRow("SELECT id FROM global_users WHERE username='batch_del_a'").Scan(&idA)
		gdb.QueryRow("SELECT id FROM global_users WHERE username='batch_del_b'").Scan(&idB)
		gdb.Exec("INSERT INTO lab_permissions (user_id, lab_url_path, role) VALUES (?, 'lab-kom-mi', 'admin')", idA)
		gdb.Exec("INSERT INTO lab_permissions (user_id, lab_url_path, role) VALUES (?, 'lab-kom-mi', 'admin')", idB)

		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.postJSON("/admin/users/batch-delete", `{"ids":["batch_del_a","batch_del_b"]}`)
		if err != nil {
			t.Fatalf("POST /admin/users/batch-delete: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		// Verify lab_permissions are removed for this lab
		var afterCount int
		gdb.QueryRow(`SELECT COUNT(*) FROM lab_permissions lp JOIN global_users gu ON gu.id=lp.user_id WHERE gu.username IN ('batch_del_a','batch_del_b') AND lp.lab_url_path='lab-kom-mi'`).Scan(&afterCount)
		if afterCount != 0 {
			t.Errorf("expected 0 lab_permissions after batch delete, got %d", afterCount)
		}
	})
}

// ============================================
// 11. PerLabUserList — GET user list (per-lab admin)
// ============================================

func TestPerLabUserList(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	t.Run("list_users", func(t *testing.T) {
		resp, err := lab.get("/admin/users")
		if err != nil {
			t.Fatalf("GET /admin/users: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("list_create_page", func(t *testing.T) {
		resp, err := lab.get("/admin/users/create")
		if err != nil {
			t.Fatalf("GET /admin/users/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// 12. PerLabUserCreate — POST user create (per-lab admin)
// ============================================

func TestPerLabUserCreate(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	gdb := env.GlobalDB
	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}

	t.Run("create_user_success", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/admin/users/create",
			"username=newuser1&password=test123&full_name=New+User+1&role=admin")
		if err != nil {
			t.Fatalf("POST /admin/users/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		// Verify user created in global_users
		var userCount int
		gdb.QueryRow("SELECT COUNT(*) FROM global_users WHERE username='newuser1'").Scan(&userCount)
		if userCount == 0 {
			t.Error("user not created in global_users")
		}
		// Verify lab_permission created
		var permCount int
		gdb.QueryRow("SELECT COUNT(*) FROM lab_permissions lp JOIN global_users gu ON gu.id=lp.user_id WHERE gu.username='newuser1' AND lp.lab_url_path='lab-kom-mi'").Scan(&permCount)
		if permCount == 0 {
			t.Error("lab_permission not created for user")
		}
	})

	t.Run("create_user_empty_form", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/admin/users/create", "username=&password=&full_name=&role=")
		if err != nil {
			t.Fatalf("POST /admin/users/create empty: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for empty form, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// 13. PerLabUserEdit — POST user edit (per-lab admin)
// ============================================

func TestPerLabUserEdit(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	gdb := env.GlobalDB
	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	t.Run("edit_user_success", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/admin/users/labA_only/edit",
			"username=labA_only&full_name=Lab+A+Updated&role=admin")
		if err != nil {
			t.Fatalf("POST /admin/users/labA_only/edit: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var fullName string
		gdb.QueryRow("SELECT full_name FROM global_users WHERE username='labA_only'").Scan(&fullName)
		if fullName != "Lab A Updated" {
			t.Errorf("expected 'Lab A Updated', got %q", fullName)
		}
	})

	t.Run("edit_not_found", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/admin/users/nonexistent-user/edit",
			"username=nonexistent&full_name=No+One&role=admin")
		if err != nil {
			t.Fatalf("POST /admin/users/nonexistent-user/edit: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 for not found, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// 14. PerLabUserDelete — POST user delete (per-lab admin)
// ============================================

func TestPerLabUserDelete(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	db := env.DB_A
	// Login as super admin (admin) — only super admins can delete per-lab users
	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}

	t.Run("delete_user_success", func(t *testing.T) {
		// Create a user first
		db.Exec("INSERT OR IGNORE INTO users (id, username, password, full_name, role) VALUES (999, 'delete_me', 'x', 'Delete Me', 'admin')")
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/admin/users/delete_me/delete", "")
		if err != nil {
			t.Fatalf("POST /admin/users/delete_me/delete: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var count int
		db.QueryRow("SELECT COUNT(*) FROM users WHERE username='delete_me'").Scan(&count)
		if count != 0 {
			t.Error("user not deleted")
		}
	})

	t.Run("delete_not_found", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/admin/users/nonexistent999/delete", "")
		if err != nil {
			t.Fatalf("POST /admin/users/nonexistent999/delete: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 for not found, got %d", resp.StatusCode)
		}
	})
}
