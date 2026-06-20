package tests

import (
	"io"
	"strings"
	"testing"
)

func TestDosenAccess(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA

	if !loginAndRefresh(lab, "labA_dosen", "test123") {
		t.Fatal("login failed")
	}

	t.Run("dashboard_accessible", func(t *testing.T) {
		resp, err := lab.get("/dashboard")
		if err != nil {
			t.Fatalf("GET /dashboard: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("admin_page_forbidden", func(t *testing.T) {
		resp, err := lab.get("/admin/users")
		if err != nil {
			t.Fatalf("GET /admin/users: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Errorf("expected 403 for non-admin on admin page, got %d", resp.StatusCode)
		}
	})

	t.Run("export_forbidden", func(t *testing.T) {
		resp, err := lab.get("/pc/export")
		if err != nil {
			t.Fatalf("GET /pc/export: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 500 {
			t.Errorf("expected 500 for non-admin on export, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Hanya admin") {
			t.Error("expected admin-only error message")
		}
	})
}

func TestDosenNoAdminAccess(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA

	if !loginAndRefresh(lab, "labA_dosen", "test123") {
		t.Fatal("login failed")
	}

	// Per-lab admin-only routes protected by AdminRequired middleware → 403
	for _, path := range []string{"/admin/users", "/admin/users/create", "/admin/activity-logs"} {
		path := path
		t.Run("forbidden_"+strings.TrimPrefix(path, "/"), func(t *testing.T) {
			resp, err := lab.get(path)
			if err != nil {
				t.Fatalf("GET %s: %v", path, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 403 {
				t.Errorf("GET %s: expected 403, got %d", path, resp.StatusCode)
			}
		})
	}
}
