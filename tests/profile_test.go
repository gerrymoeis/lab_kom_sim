package tests

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

// ============================================
// TestProfile — profile view + update + fail
// ============================================

func TestProfile(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA

	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	t.Run("success_view", func(t *testing.T) {
		resp, err := lab.get("/profile")
		if err != nil {
			t.Fatalf("GET /profile: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("success_update", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/profile",
			"username=labA_only&full_name=Lab+A+Admin+Updated")
		if err != nil {
			t.Fatalf("POST /profile: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_empty_fields", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/profile", "username=&full_name=")
		if err != nil {
			t.Fatalf("POST /profile empty: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_duplicate_username", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		// Try to change username to "admin" which is taken
		resp, err := lab.post("/profile",
			"username=admin&full_name=Should+Fail")
		if err != nil {
			t.Fatalf("POST /profile duplicate: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// TestChangePassword — change password + fail
// ============================================

func TestChangePassword(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA

	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	t.Run("success", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		// Keep same password to avoid breaking subsequent subtests
		resp, err := lab.post("/profile/password",
			"old_password=test123&new_password=test123&confirm_password=test123")
		if err != nil {
			t.Fatalf("POST /profile/password: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_wrong_old_password", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/profile/password",
			"old_password=wrongpass&new_password=test123&confirm_password=test123")
		if err != nil {
			t.Fatalf("POST /profile/password wrong old: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_mismatch", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/profile/password",
			"old_password=test123&new_password=newpass&confirm_password=mismatch")
		if err != nil {
			t.Fatalf("POST /profile/password mismatch: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_empty_fields", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/profile/password",
			"old_password=&new_password=&confirm_password=")
		if err != nil {
			t.Fatalf("POST /profile/password empty: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// TestPrint — print form + generate PDF + fail
// ============================================

func TestPrint(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA

	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	t.Run("success_form", func(t *testing.T) {
		resp, err := lab.get("/print")
		if err != nil {
			t.Fatalf("GET /print: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("success_generate_pdf", func(t *testing.T) {
		params := fmt.Sprintf("type=pc&pc_labels=%s&font_size=0.5&padding_h=0.3&padding_v=0.3&paper_size=A4&num_sheets=1",
			"pc-1,pc-dosen")
		resp, err := lab.get("/print/generate?" + params)
		if err != nil {
			t.Fatalf("GET /print/generate: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "application/pdf") {
			t.Errorf("expected PDF content-type, got %q", ct)
		}
		body, _ := io.ReadAll(resp.Body)
		if len(body) == 0 {
			t.Error("expected non-empty PDF body")
		}
	})

	t.Run("fail_invalid_paper", func(t *testing.T) {
		resp, err := lab.get("/print/generate?type=pc&pc_labels=pc-1&font_size=0.5&paper_size=Invalid&num_sheets=1")
		if err != nil {
			t.Fatalf("GET /print/generate invalid paper: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 500 {
			t.Errorf("expected 500, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_invalid_font_size", func(t *testing.T) {
		resp, err := lab.get("/print/generate?type=pc&pc_labels=pc-1&font_size=10.0&paper_size=A4&num_sheets=1")
		if err != nil {
			t.Fatalf("GET /print/generate invalid font: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 500 {
			t.Errorf("expected 500, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_no_labels", func(t *testing.T) {
		resp, err := lab.get("/print/generate?type=device&font_size=0.5&paper_size=A4&num_sheets=1")
		if err != nil {
			t.Fatalf("GET /print/generate no labels: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 500 {
			t.Errorf("expected 500, got %d", resp.StatusCode)
		}
	})
}
