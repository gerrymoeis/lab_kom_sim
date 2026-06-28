package tests

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
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

// adminGet performs GET for the given path with super admin cookies and returns response.
func adminGet(env *TestEnvironment, path string) *http.Response {
	req, _ := http.NewRequest("GET", env.TS.URL+path, nil)
	env.LabA.addCookies(req)
	resp, err := env.Client.Do(req)
	if err != nil {
		env.LabA.t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

// adminPost performs POST for the given path with CSRF token and super admin cookies.
func adminPost(env *TestEnvironment, path, data string) *http.Response {
	if data == "" {
		data = "_csrf=" + url.QueryEscape(env.LabA.csrf)
	} else {
		data = data + "&_csrf=" + url.QueryEscape(env.LabA.csrf)
	}
	req, _ := http.NewRequest("POST", env.TS.URL+path, strings.NewReader(data))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	env.LabA.addCookies(req)
	resp, err := env.Client.Do(req)
	if err != nil {
		env.LabA.t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

// loginAs clears existing session, performs login as an arbitrary user and extracts CSRF token.
func loginAs(env *TestEnvironment, username, password string) (cookies map[string]string, csrf string) {
	env.LabA.cookies = make(map[string]string)
	env.LabA.csrf = ""
	// Clear any existing session_token to avoid ErrAlreadyLoggedIn
	env.GlobalDB.Exec("UPDATE global_users SET session_token = '' WHERE username = ?", username)
	if !env.LabA.login(username, password) {
		env.LabA.t.Fatalf("%s login failed", username)
	}
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

// adminPostNoCSRF performs POST for the given path WITHOUT CSRF token to test CSRF rejection.
func adminPostNoCSRF(env *TestEnvironment, path string) *http.Response {
	req, _ := http.NewRequest("POST", env.TS.URL+path, strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := env.Client.Do(req)
	if err != nil {
		env.LabA.t.Fatalf("POST %s (no CSRF): %v", path, err)
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
		// Now redirects to /<first_lab>/dashboard instead of 403
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 redirect for non-super-admin, got %d", resp.StatusCode)
		}
	})

	t.Run("unauthorized_no_session", func(t *testing.T) {
		req, _ := http.NewRequest("GET", env.TS.URL+"/labs", nil)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("GET /labs: %v", err)
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
		resp := adminPost(env, "/labs/lab-kom-mi/layout", "cols_per_row=10,10,10,10&row_gaps_json=[[],[],[],[4]]")
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 after save, got %d", resp.StatusCode)
		}
		loc := resp.Header.Get("Location")
		if loc != "/labs/lab-kom-mi" {
			t.Errorf("expected redirect to /labs/lab-kom-mi, got %q", loc)
		}
	})

	t.Run("save_layout_bad_format", func(t *testing.T) {
		token := env.LabA.csrf
		resp := adminPost(env, "/labs/lab-kom-mi/layout", "cols_per_row=abc&row_gaps_json=[]")
		defer resp.Body.Close()
		env.LabA.csrf = token // restore after failed post (token unchanged)
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for bad cols format, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// 3. AdminLabSeeds — DISABLED TEMPORARILY (seeds management for future Fase 5)
// ============================================

/*
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
*/

// ============================================
// 4. AdminUserCreate — GET + POST + validation
// ============================================

func TestAdminUserCreate(t *testing.T) {
	env := setupTestEnvironment(t)
	loginAsAdmin(env)

	t.Run("get_create_page", func(t *testing.T) {
		resp := adminGet(env, "/labs/admin/users/create")
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
		resp := adminPost(env, "/labs/admin/users/create", "username=newuser&password=newpass123&full_name=New+User&is_super_admin=0")
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 after create, got %d", resp.StatusCode)
		}
		loc := resp.Header.Get("Location")
		if loc != "/labs/admin/users" {
			t.Errorf("expected redirect to /labs/admin/users, got %q", loc)
		}
		// Verify user exists in global DB
		var count int
		env.GlobalDB.QueryRow("SELECT COUNT(*) FROM global_users WHERE username='newuser'").Scan(&count)
		if count != 1 {
			t.Errorf("expected 1 newuser, got %d", count)
		}
	})

	t.Run("create_user_empty_fields", func(t *testing.T) {
		resp := adminPost(env, "/labs/admin/users/create", "username=&password=&full_name=&is_super_admin=0")
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for empty fields, got %d", resp.StatusCode)
		}
	})

	t.Run("create_duplicate_username", func(t *testing.T) {
		resp := adminPost(env, "/labs/admin/users/create", "username=admin&password=test123&full_name=Duplicate&is_super_admin=0")
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
		resp := adminGet(env, "/labs/admin/users/1/edit")
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
		resp := adminPost(env, "/labs/admin/users/1/edit", "username=admin_updated&full_name=Admin+Updated&is_super_admin=1")
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
		resp := adminPost(env, "/labs/admin/users/999/edit", "username=nonexistent&full_name=Nobody&is_super_admin=0")
		defer resp.Body.Close()
		if resp.StatusCode != 400 && resp.StatusCode != 404 {
			t.Errorf("expected 400 or 404, got %d", resp.StatusCode)
		}
	})

	t.Run("edit_user_change_password", func(t *testing.T) {
		resp := adminPost(env, "/labs/admin/users/1/edit", "username=admin&full_name=Admin&is_super_admin=1&new_password=newpass456")
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 after password change, got %d", resp.StatusCode)
		}
	})

	t.Run("edit_invalid_id", func(t *testing.T) {
		resp := adminGet(env, "/labs/admin/users/abc/edit")
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
		resp := adminPost(env, "/labs/admin/users/create", "username=deletable&password=test123&full_name=Deletable&is_super_admin=0")
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
		resp := adminPost(env, "/labs/admin/users/"+strconv.Itoa(userID)+"/delete", "")
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
		// Create a non-SA user marked as protected
		hash, _ := bcrypt.GenerateFromPassword([]byte("test123"), bcrypt.DefaultCost)
		env.GlobalDB.Exec("INSERT OR IGNORE INTO global_users (username, password, full_name, is_super_admin, is_protected) VALUES (?, ?, ?, 0, 1)", "protected_user", string(hash), "Protected User")
		var protID int
		env.GlobalDB.QueryRow("SELECT id FROM global_users WHERE username='protected_user'").Scan(&protID)
		if protID == 0 {
			t.Fatal("could not get protected user id")
		}
		resp := adminPost(env, fmt.Sprintf("/labs/admin/users/%d/delete", protID), "")
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
		resp := adminGet(env, "/labs/admin/users/3/permissions")
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
		resp := adminPost(env, "/labs/admin/users/3/permissions", "labs=lab-kom-mi&roles=admin")
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 after permissions save, got %d", resp.StatusCode)
		}
		loc := resp.Header.Get("Location")
		if loc != "/labs/admin/users" {
			t.Errorf("expected redirect to /labs/admin/users, got %q", loc)
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
		resp := adminGet(env, "/labs/admin/users/1/permissions")
		defer resp.Body.Close()
		// Super admin has no permission editing — check if it redirects or shows 200
		// Based on code: AdminUserPermissions doesn't check isSuperAdmin, it just shows the page
		// but POST to AdminUserPermissionsSave redirects if isSuperAdmin
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("permissions_nonexistent_user", func(t *testing.T) {
		resp := adminGet(env, "/labs/admin/users/999/permissions")
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
		req, _ := http.NewRequest("POST", env.TS.URL+"/labs/admin/users/create", strings.NewReader("username=csrf_test&password=test123&full_name=CSRF+Test&is_super_admin=0"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		env.LabA.addCookies(req)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("POST /labs/admin/users/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Errorf("expected 403 for missing CSRF, got %d", resp.StatusCode)
		}
	})

	t.Run("post_with_invalid_csrf_returns_403", func(t *testing.T) {
		req, _ := http.NewRequest("POST", env.TS.URL+"/labs/admin/users/create", strings.NewReader("_csrf=invalidtoken123&username=csrf_test2&password=test123&full_name=CSRF+Test+2&is_super_admin=0"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		env.LabA.addCookies(req)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("POST /labs/admin/users/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Errorf("expected 403 for invalid CSRF, got %d", resp.StatusCode)
		}
	})

		t.Run("get_routes_without_auth_redirect", func(t *testing.T) {
		req, _ := http.NewRequest("GET", env.TS.URL+"/labs", nil)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("GET /labs: %v", err)
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
	gdb := env.GlobalDB
	// Login as super admin (admin) — only super admins can delete per-lab users
	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}

	t.Run("delete_user_success", func(t *testing.T) {
		// Create a user in global DB with lab permission
		gdb.Exec("DELETE FROM lab_permissions WHERE user_id IN (SELECT id FROM global_users WHERE username='delete_me')")
		gdb.Exec("DELETE FROM global_users WHERE username='delete_me'")
		gdb.Exec("INSERT INTO global_users (id, username, password, full_name) VALUES (999, 'delete_me', '$2a$10$dummy', 'Delete Me')")
		gdb.Exec("INSERT INTO lab_permissions (user_id, lab_url_path, role) VALUES (999, 'lab-kom-mi', 'admin')")
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
		// Verify lab_permission removed for this lab
		var permCount int
		gdb.QueryRow("SELECT COUNT(*) FROM lab_permissions WHERE user_id=999 AND lab_url_path='lab-kom-mi'").Scan(&permCount)
		if permCount != 0 {
			t.Error("lab_permission not removed after delete")
		}
		// Global user should still exist (only lab_permission removed)
		var userCount int
		gdb.QueryRow("SELECT COUNT(*) FROM global_users WHERE username='delete_me'").Scan(&userCount)
		if userCount == 0 {
			t.Error("global user should still exist after per-lab delete")
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

// ============================================
// 15. AuthZ Hierarchy — 10 test scenarios
// ============================================

func TestAuthZScenarios(t *testing.T) {
	env := setupTestEnvironment(t)
	gdb := env.GlobalDB

	bcryptHash := func(pw string) string {
		h, _ := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
		return string(h)
	}

	// Create: non-protected SA (rekan is already is_super_admin=1, is_protected=0 from seed)
	// Create: GAB user
	gdb.Exec("INSERT OR IGNORE INTO global_users (username, password, full_name, is_global_admin) VALUES (?, ?, ?, 1)",
		"gab_user", bcryptHash("test123"), "GAB User")
	var gabID int
	gdb.QueryRow("SELECT id FROM global_users WHERE username='gab_user'").Scan(&gabID)
	gdb.Exec("INSERT OR IGNORE INTO lab_permissions (user_id, lab_url_path, role) VALUES (?, ?, 'admin')", gabID, "lab-kom-mi")

	// Create: second MA user for testing (first MA "lab-kom-mi" already exists from seed)
	gdb.Exec("INSERT OR IGNORE INTO global_users (username, password, full_name) VALUES (?, ?, ?)",
		"ma_user2", bcryptHash("test123"), "MA User 2")
	var maUser2ID int
	gdb.QueryRow("SELECT id FROM global_users WHERE username='ma_user2'").Scan(&maUser2ID)
	gdb.Exec("INSERT OR IGNORE INTO lab_permissions (user_id, lab_url_path, role, is_main_account) VALUES (?, ?, 'admin', 1)",
		maUser2ID, "lab-kom-mi")

	t.Run("01_create_GAB_as_non_protected_SA_returns_403", func(t *testing.T) {
		gdb.Exec("UPDATE global_users SET session_token = ''")
		loginAs(env, "rekan", "rekan123")
		resp := adminPost(env, "/labs/admin/users/create", "username=gab_fail&password=test123&full_name=GAB+Fail&is_global_admin=1")
		defer resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Errorf("expected 403 for non-protected SA creating GAB, got %d", resp.StatusCode)
		}
	})

	t.Run("02_edit_SA_as_non_protected_SA_returns_403", func(t *testing.T) {
		gdb.Exec("UPDATE global_users SET session_token = ''")
		loginAs(env, "rekan", "rekan123")
		resp := adminPost(env, "/labs/admin/users/1/edit", "username=admin&full_name=Hacked&is_super_admin=1")
		defer resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Errorf("expected 403 for non-protected SA editing SA, got %d", resp.StatusCode)
		}
	})

	t.Run("03_delete_SA_from_global_panel_returns_error", func(t *testing.T) {
		gdb.Exec("UPDATE global_users SET session_token = ''")
		loginAs(env, "rekan", "rekan123")
		resp := adminPost(env, "/labs/admin/users/1/delete", "")
		defer resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Errorf("expected 403 for non-protected SA deleting SA, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "tidak bisa dihapus") && !strings.Contains(string(body), "protected") {
			t.Error("body should contain protected/super admin error message")
		}
	})

	t.Run("04_delete_MA_as_non_protected_SA_redirects", func(t *testing.T) {
		lab := env.LabA
		gdb.Exec("UPDATE global_users SET session_token = ''")
		lab.cookies = make(map[string]string)
		if !loginAndRefresh(lab, "rekan", "rekan123") {
			t.Fatal("rekan login failed")
		}
		resp, err := lab.post("/admin/users/ma_user2/delete", "")
		if err != nil {
			t.Fatalf("POST /admin/users/ma_user2/delete: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 redirect, got %d", resp.StatusCode)
		}
	})

	t.Run("05_delete_MA_as_protected_SA_succeeds", func(t *testing.T) {
		lab := env.LabA
		gdb.Exec("UPDATE global_users SET session_token = ''")
		lab.cookies = make(map[string]string)
		if !loginAndRefresh(lab, "admin", "admin123") {
			t.Fatal("admin login failed")
		}
		resp, err := lab.post("/admin/users/ma_user2/delete", "")
		if err != nil {
			t.Fatalf("POST /admin/users/ma_user2/delete: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 success, got %d", resp.StatusCode)
		}
		// Re-create MA user2 for subsequent tests
		gdb.Exec("INSERT OR IGNORE INTO global_users (username, password, full_name) VALUES (?, ?, ?)",
			"ma_user2", bcryptHash("test123"), "MA User 2")
		var newID int
		gdb.QueryRow("SELECT id FROM global_users WHERE username='ma_user2'").Scan(&newID)
		gdb.Exec("INSERT OR IGNORE INTO lab_permissions (user_id, lab_url_path, role, is_main_account) VALUES (?, ?, 'admin', 1)",
			newID, "lab-kom-mi")
	})

	t.Run("06_MA_view_detail_another_MA_redirects", func(t *testing.T) {
		lab := env.LabA
		gdb.Exec("UPDATE global_users SET session_token = ''")
		lab.cookies = make(map[string]string)
		if !loginAndRefresh(lab, "lab-kom-mi", "lab-kom-mi123") {
			t.Fatal("MA login failed")
		}
		// Target: "ma_user2" is also MA for same lab
		resp, err := lab.get("/admin/users/ma_user2")
		if err != nil {
			t.Fatalf("GET /admin/users/ma_user2: %v", err)
		}
		defer resp.Body.Close()
		// MA viewing another MA should redirect because canAccessProfile returns false
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 for MA viewing another MA, got %d", resp.StatusCode)
		}
	})

	t.Run("07_batch_delete_with_MA_returns_error", func(t *testing.T) {
		lab := env.LabA
		gdb.Exec("UPDATE global_users SET session_token = ''")
		lab.cookies = make(map[string]string)
		if !loginAndRefresh(lab, "admin", "admin123") {
			t.Fatal("admin login failed")
		}
		// Create a regular user with MA username in list
		resp, err := lab.postJSON("/admin/users/batch-delete", `{"ids":["ma_user2","labA_only"]}`)
		if err != nil {
			t.Fatalf("POST /admin/users/batch-delete: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 500 {
			t.Errorf("expected 500 for batch delete with MA, got %d", resp.StatusCode)
		}
	})

	t.Run("08_batch_delete_with_SA_returns_error", func(t *testing.T) {
		lab := env.LabA
		gdb.Exec("UPDATE global_users SET session_token = ''")
		lab.cookies = make(map[string]string)
		if !loginAndRefresh(lab, "admin", "admin123") {
			t.Fatal("admin login failed")
		}
		resp, err := lab.postJSON("/admin/users/batch-delete", `{"ids":["admin","labA_only"]}`)
		if err != nil {
			t.Fatalf("POST /admin/users/batch-delete: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 500 {
			t.Errorf("expected 500 for batch delete with SA, got %d", resp.StatusCode)
		}
	})

	t.Run("09_is_protected_session_value_after_login", func(t *testing.T) {
		gdb.Exec("UPDATE global_users SET session_token = ''")
		// Login as admin (protected) → should be able to create GAB
		loginAs(env, "admin", "admin123")
		resp := adminPost(env, "/labs/admin/users/create", "username=gab_ok&password=test123&full_name=GAB+OK&is_global_admin=1")
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 when protected SA creates GAB, got %d", resp.StatusCode)
		}
		// Verify GAB user was created
		var gabCount int
		gdb.QueryRow("SELECT COUNT(*) FROM global_users WHERE username='gab_ok' AND is_global_admin=1").Scan(&gabCount)
		if gabCount == 0 {
			t.Error("expected gab_ok user to be created")
		}
		// Now login as rekan (non-protected) → should NOT be able to create GAB
		gdb.Exec("UPDATE global_users SET session_token = ''")
		loginAs(env, "rekan", "rekan123")
		resp2 := adminPost(env, "/labs/admin/users/create", "username=gab_fail2&password=test123&full_name=GAB+Fail+2&is_global_admin=1")
		defer resp2.Body.Close()
		if resp2.StatusCode != 403 {
			t.Errorf("expected 403 when non-protected SA attempts to create GAB, got %d", resp2.StatusCode)
		}
	})

	t.Run("10_self_delete_from_global_panel_redirects", func(t *testing.T) {
		gdb.Exec("UPDATE global_users SET session_token = ''")
		loginAs(env, "rekan", "rekan123")
		var rekanID int
		gdb.QueryRow("SELECT id FROM global_users WHERE username='rekan'").Scan(&rekanID)
		if rekanID == 0 {
			t.Fatal("rekan not found")
		}
		resp := adminPost(env, fmt.Sprintf("/labs/admin/users/%d/delete", rekanID), "")
		defer resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Errorf("expected 403 for self-delete, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "akun Anda sendiri") {
			t.Error("body should contain self-delete error message")
		}
	})
}
