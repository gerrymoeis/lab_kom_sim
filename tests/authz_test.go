package tests

import (
	"io"
	"net/http"
	"testing"
)

func TestAuthZ_SuperAdminAccessAllLabs(t *testing.T) {
	env := setupTestEnvironment(t)

	loginAs(env, "admin", "admin123")

	for _, lab := range []*testLab{env.LabA, env.LabB} {
		resp, err := env.LabA.getURL(lab.ts.URL + lab.prefix + "/dashboard")
		if err != nil {
			t.Fatalf("%s: request gagal: %v", lab.url, err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: super admin expected 200, got %d", lab.url, resp.StatusCode)
		}
	}
}

func TestAuthZ_LabAdminDeniedFromOtherLab(t *testing.T) {
	env := setupTestEnvironment(t)

	// Use env.LabA.login directly (not loginAs from admin_panel_test.go)
	env.GlobalDB.Exec("UPDATE global_users SET session_token = '' WHERE username = 'labA_only'")
	if !env.LabA.login("labA_only", "test123") {
		t.Fatal("labA_only login gagal")
	}

	resp, err := env.LabA.getURL(env.LabA.ts.URL + env.LabB.prefix + "/dashboard")
	if err != nil {
		t.Fatalf("request gagal: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Errorf("expected non-200 for cross-lab access, got %d", resp.StatusCode)
	}
}

func TestAuthZ_NoPermissionUserDenied(t *testing.T) {
	env := setupTestEnvironment(t)

	env.GlobalDB.Exec("UPDATE global_users SET session_token = '' WHERE username = 'no_perm_user'")
	if !env.LabA.login("no_perm_user", "test123") {
		t.Fatal("no_perm_user login gagal")
	}

	resp, err := env.LabA.getURL(env.LabA.ts.URL + env.LabA.prefix + "/dashboard")
	if err != nil {
		t.Fatalf("request gagal: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Errorf("expected non-200 for user without lab access, got %d", resp.StatusCode)
	}
}

func TestAuthZ_UnauthenticatedRedirectToLogin(t *testing.T) {
	env := setupTestEnvironment(t)

	resp, err := env.LabA.client.Get(env.LabA.ts.URL + env.LabA.prefix + "/dashboard")
	if err != nil {
		t.Fatalf("request gagal: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusOK {
		t.Errorf("expected redirect 302 or login 200, got %d", resp.StatusCode)
	}
}
