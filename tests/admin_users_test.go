package tests

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// ============================================
// Fase E: Global Admin User Management (G19)
// ============================================

func TestAdminUserList(t *testing.T) {
	env := setupTestEnvironment(t)

	// Login once as super admin for all admin subtests
	if !env.LabA.login("admin", "admin123") {
		t.Fatal("admin login failed")
	}

	t.Run("E.1_success_as_super_admin", func(t *testing.T) {
		req, _ := http.NewRequest("GET", env.TS.URL+"/labs/admin/users", nil)
		env.LabA.addCookies(req)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("GET /labs/admin/users: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		html := string(body)
		for _, user := range []string{"admin", "rekan", "labA_only", "labB_only", "no_perm_user", "labA_dosen"} {
			if !strings.Contains(html, user) {
				t.Errorf("expected user %q in response", user)
			}
		}
		if !strings.Contains(html, "Manage Users") {
			t.Error("expected page title 'Manage Users'")
		}
	})

	t.Run("E.1_fail_unauthenticated", func(t *testing.T) {
		req, _ := http.NewRequest("GET", env.TS.URL+"/labs/admin/users", nil)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("GET /labs/admin/users: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
	})

	t.Run("E.2_pagination_query_params", func(t *testing.T) {
		req, _ := http.NewRequest("GET", env.TS.URL+"/labs/admin/users?page=1&per_page=10", nil)
		env.LabA.addCookies(req)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("GET /labs/admin/users?page=1&per_page=10: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("E.2_search_query_param", func(t *testing.T) {
		req, _ := http.NewRequest("GET", env.TS.URL+"/labs/admin/users?search=admin", nil)
		env.LabA.addCookies(req)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("GET /labs/admin/users?search=admin: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		html := string(body)
		if !strings.Contains(html, "admin") {
			t.Error("expected 'admin' user in search results")
		}
	})

	t.Run("E.1_fail_regular_admin", func(t *testing.T) {
		// Clear cookies and login as regular admin
		env.LabA.cookies = make(map[string]string)
		if !env.LabA.login("labA_only", "test123") {
			t.Fatal("labA_only login failed")
		}
		req, _ := http.NewRequest("GET", env.TS.URL+"/labs/admin/users", nil)
		env.LabA.addCookies(req)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("GET /labs/admin/users: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
	})
}
