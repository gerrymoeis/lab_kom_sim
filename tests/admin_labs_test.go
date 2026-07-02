package tests

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================
// Fase D: Lab Lifecycle Testing
// ============================================

// D.1: AdminLabCreatePage — GET /labs/create
func TestAdminLabCreatePage(t *testing.T) {
	env := setupTestEnvironment(t)

	t.Run("D.1_success_as_super_admin", func(t *testing.T) {
		if !loginAndRefresh(env.LabA, "admin", "admin123") {
			t.Fatal("admin login failed")
		}
		req, _ := http.NewRequest("GET", env.TS.URL+"/labs/create", nil)
		env.LabA.addCookies(req)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("GET /labs/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		html := string(body)
		for _, field := range []string{"id", "title", "url", "rows", "cols"} {
			if !strings.Contains(html, `name="`+field+`"`) {
				t.Errorf("form should have %s field", field)
			}
		}
	})

	t.Run("D.1_fail_unauthenticated", func(t *testing.T) {
		req, _ := http.NewRequest("GET", env.TS.URL+"/labs/create", nil)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("GET /labs/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
	})

	t.Run("D.1_fail_regular_admin", func(t *testing.T) {
		if !loginAndRefresh(env.LabA, "labA_only", "test123") {
			t.Fatal("labA_only login failed")
		}
		req, _ := http.NewRequest("GET", env.TS.URL+"/labs/create", nil)
		env.LabA.addCookies(req)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("GET /labs/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
	})
}

// D.2: AdminLabCreate — POST /labs/create
func TestAdminLabCreate(t *testing.T) {
	uploadDir := t.TempDir()
	env := setupTestEnvironment(t, TestConfigOverrides{UploadPath: uploadDir})

	envFile := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envFile, []byte("EXISTING_VAR=1\n"), 0644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	env.Config.EnvPath = envFile

	if !loginAndRefresh(env.LabA, "admin", "admin123") {
		t.Fatal("admin login failed")
	}

	t.Run("D.2_create_lab_success", func(t *testing.T) {
		if !env.LabA.refreshCSRF() {
			t.Fatal("refresh CSRF failed")
		}
		formData := url.Values{
			"_csrf": {env.LabA.csrf},
			"id":    {"NEWLAB-1"},
			"title": {"Lab Baru"},
			"url":   {"labbaru"},
			"rows":  {"2"},
			"cols":  {"8,8"},
		}.Encode()

		req, _ := http.NewRequest("POST", env.TS.URL+"/labs/create", strings.NewReader(formData))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		env.LabA.addCookies(req)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("POST /labs/create: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 302 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 302, got %d: %s", resp.StatusCode, string(body))
		}
		loc := resp.Header.Get("Location")
		if loc != "/labs/labbaru" {
			t.Errorf("expected redirect to /labs/labbaru, got %q", loc)
		}

		// Verify new lab dashboard is accessible
		req, _ = http.NewRequest("GET", env.TS.URL+"/labbaru/dashboard", nil)
		env.LabA.addCookies(req)
		resp, err = env.Client.Do(req)
		if err != nil {
			t.Fatalf("GET /labbaru/dashboard: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200 for new lab dashboard, got %d", resp.StatusCode)
		}

		// Verify DB file exists
		existingDir := filepath.Dir(env.Config.Labs[0].DBPath)
		dbPath := filepath.Join(existingDir, "lab_labbaru.db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			t.Errorf("DB file not found: %s", dbPath)
		}

		// Verify .env was updated with new LABS entries
		envData, _ := os.ReadFile(envFile)
		content := string(envData)
		if !strings.Contains(content, "LABS_1_ID=NEWLAB-1") {
			t.Error(".env should contain LABS_1_ID=NEWLAB-1")
		}
		if !strings.Contains(content, "LABS_1_TITLE=Lab Baru") {
			t.Error(".env should contain LABS_1_TITLE=Lab Baru")
		}
	})

	t.Run("D.2_create_duplicate_url", func(t *testing.T) {
		if !env.LabA.refreshCSRF() {
			t.Fatal("refresh CSRF failed")
		}
		formData := url.Values{
			"_csrf": {env.LabA.csrf},
			"id":    {"DUP-1"},
			"title": {"Duplicate URL"},
			"url":   {"lab-kom-mi"},
			"rows":  {"2"},
			"cols":  {"8,8"},
		}.Encode()

		req, _ := http.NewRequest("POST", env.TS.URL+"/labs/create", strings.NewReader(formData))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		env.LabA.addCookies(req)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("POST /labs/create duplicate: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for duplicate URL, got %d", resp.StatusCode)
		}
	})

	t.Run("D.2_create_duplicate_id", func(t *testing.T) {
		if !env.LabA.refreshCSRF() {
			t.Fatal("refresh CSRF failed")
		}
		formData := url.Values{
			"_csrf": {env.LabA.csrf},
			"id":    {"MI-1"},
			"title": {"Duplicate ID"},
			"url":   {"dupid"},
			"rows":  {"2"},
			"cols":  {"8,8"},
		}.Encode()

		req, _ := http.NewRequest("POST", env.TS.URL+"/labs/create", strings.NewReader(formData))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		env.LabA.addCookies(req)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("POST /labs/create duplicate id: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for duplicate ID, got %d", resp.StatusCode)
		}
	})

	t.Run("D.2_create_empty_fields", func(t *testing.T) {
		if !env.LabA.refreshCSRF() {
			t.Fatal("refresh CSRF failed")
		}
		formData := url.Values{
			"_csrf": {env.LabA.csrf},
			"id":    {""},
			"title": {""},
			"url":   {""},
			"rows":  {""},
			"cols":  {""},
		}.Encode()

		req, _ := http.NewRequest("POST", env.TS.URL+"/labs/create", strings.NewReader(formData))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		env.LabA.addCookies(req)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("POST /labs/create empty: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for empty fields, got %d", resp.StatusCode)
		}
	})

	t.Run("D.2_create_mismatched_rows_cols", func(t *testing.T) {
		if !env.LabA.refreshCSRF() {
			t.Fatal("refresh CSRF failed")
		}
		formData := url.Values{
			"_csrf": {env.LabA.csrf},
			"id":    {"BAD-1"},
			"title": {"Bad Layout"},
			"url":   {"badlayout"},
			"rows":  {"3"},
			"cols":  {"8,8"},
		}.Encode()

		req, _ := http.NewRequest("POST", env.TS.URL+"/labs/create", strings.NewReader(formData))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		env.LabA.addCookies(req)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("POST /labs/create bad layout: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for mismatched rows/cols, got %d", resp.StatusCode)
		}
	})
}

// D.3: AdminLabDelete — POST /labs/:urlPath/delete
func TestAdminLabDelete(t *testing.T) {
	uploadDir := t.TempDir()
	env := setupTestEnvironment(t, TestConfigOverrides{UploadPath: uploadDir})

	envFile := filepath.Join(t.TempDir(), ".env")
	env.Config.EnvPath = envFile

	if !loginAndRefresh(env.LabA, "admin", "admin123") {
		t.Fatal("admin login failed")
	}

	t.Run("D.3_delete_vokasi_success", func(t *testing.T) {
		// Capture DB path BEFORE delete (handler mutates cfg.Labs in-place)
		vokasiDBPath := env.Config.Labs[1].DBPath
		if !env.LabA.refreshCSRF() {
			t.Fatal("refresh CSRF failed")
		}
		formData := url.Values{"_csrf": {env.LabA.csrf}}.Encode()
		req, _ := http.NewRequest("POST", env.TS.URL+"/labs/vokasi/delete", strings.NewReader(formData))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		env.LabA.addCookies(req)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("POST /labs/vokasi/delete: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 302 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 302, got %d: %s", resp.StatusCode, string(body))
		}

		// Verify DB file renamed to .deleted
		if _, err := os.Stat(vokasiDBPath + ".deleted"); os.IsNotExist(err) {
			t.Error("expected vokasi DB to be renamed to .deleted")
		}

		// Verify vokasi dashboard returns redirect (no longer accessible)
		req, _ = http.NewRequest("GET", env.TS.URL+"/vokasi/dashboard", nil)
		env.LabA.addCookies(req)
		resp, err = env.Client.Do(req)
		if err != nil {
			t.Fatalf("GET /vokasi/dashboard: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 for deleted lab, got %d", resp.StatusCode)
		}

		// Verify lab-kom-mi (LabA) still works
		req, _ = http.NewRequest("GET", env.TS.URL+"/lab-kom-mi/dashboard", nil)
		env.LabA.addCookies(req)
		resp, err = env.Client.Do(req)
		if err != nil {
			t.Fatalf("GET /lab-kom-mi/dashboard: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200 for remaining lab, got %d", resp.StatusCode)
		}
	})

	t.Run("D.3_delete_nonexistent_lab", func(t *testing.T) {
		if !env.LabA.refreshCSRF() {
			t.Fatal("refresh CSRF failed")
		}
		formData := url.Values{"_csrf": {env.LabA.csrf}}.Encode()
		req, _ := http.NewRequest("POST", env.TS.URL+"/labs/nonexistent/delete", strings.NewReader(formData))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		env.LabA.addCookies(req)
		resp, err := env.Client.Do(req)
		if err != nil {
			t.Fatalf("POST /labs/nonexistent/delete: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		html := string(body)
		if !strings.Contains(html, "Lab Tidak Ditemukan") {
			t.Errorf("expected lab not found message, got: %s", string(body))
		}
	})

	t.Run("D.3_delete_last_lab", func(t *testing.T) {
		// This test runs in a fresh environment — only 1 lab needed
		env2 := setupTestEnvironment(t, TestConfigOverrides{UploadPath: t.TempDir()})
		if loginAndRefresh(env2.LabA, "admin", "admin123") {
			env2.LabA.refreshCSRF()
			// Delete LabB first
			formData := url.Values{"_csrf": {env2.LabA.csrf}}.Encode()
			req, _ := http.NewRequest("POST", env2.TS.URL+"/labs/vokasi/delete", strings.NewReader(formData))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			env2.LabA.addCookies(req)
			env2.Client.Do(req)
			env2.LabA.refreshCSRF()

			// Now try to delete the last lab
			formData = url.Values{"_csrf": {env2.LabA.csrf}}.Encode()
			req, _ = http.NewRequest("POST", env2.TS.URL+"/labs/lab-kom-mi/delete", strings.NewReader(formData))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			env2.LabA.addCookies(req)
			resp, err := env2.Client.Do(req)
			if err != nil {
				t.Fatalf("POST /labs/lab-kom-mi/delete last: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 302 {
				t.Errorf("expected 302 when trying to delete last lab, got %d", resp.StatusCode)
			}
		}
	})
}
