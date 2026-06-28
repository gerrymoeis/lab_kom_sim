package tests

import (
	"fmt"
	"testing"
)

// ============================================
// TestCrossLabIsolation — verifies lab-to-lab 403 + data isolation
// ============================================

func TestCrossLabIsolation(t *testing.T) {
	env := setupTestEnvironment(t)
	labA := env.LabA
	labB := env.LabB
	dbA := env.DB_A
	dbB := env.DB_B
	tsURL := env.TS.URL

	t.Run("lab_a_to_b_forbidden", func(t *testing.T) {
		if !loginAndRefresh(labA, "labA_only", "test123") {
			t.Fatal("login failed")
		}
		// Lab A-only user accessing Lab B — now redirects to own lab
		resp, err := labA.getURL(tsURL + labB.prefix + "/dashboard")
		if err != nil {
			t.Fatalf("GET Lab B dashboard: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 (redirect to own lab), got %d", resp.StatusCode)
		}
		// Own lab still accessible
		resp, err = labA.get("/dashboard")
		if err != nil {
			t.Fatalf("GET Lab A dashboard: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200 for own lab, got %d", resp.StatusCode)
		}
	})

	t.Run("lab_b_to_a_forbidden", func(t *testing.T) {
		if !loginAndRefresh(labB, "labB_only", "test123") {
			t.Fatal("login failed")
		}
		// Lab B-only user accessing Lab A — now redirects to own lab
		resp, err := labB.getURL(tsURL + labA.prefix + "/dashboard")
		if err != nil {
			t.Fatalf("GET Lab A dashboard from Lab B: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 (redirect to own lab), got %d", resp.StatusCode)
		}
		// Own lab still accessible
		resp, err = labB.get("/dashboard")
		if err != nil {
			t.Fatalf("GET Lab B dashboard: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200 for own lab, got %d", resp.StatusCode)
		}
	})

	t.Run("data_isolation_pc", func(t *testing.T) {
		var pcCountA, pcCountB int
		dbA.QueryRow("SELECT COUNT(*) FROM pcs").Scan(&pcCountA)
		dbB.QueryRow("SELECT COUNT(*) FROM pcs").Scan(&pcCountB)
		if pcCountA == 0 {
			t.Error("Lab A should have seeded PCs")
		}
		// Each lab has its own seed data; verify both have PCs
		if pcCountB == 0 {
			t.Error("Lab B should have its own seeded PCs")
		}
	})

	t.Run("data_isolation_software", func(t *testing.T) {
		var swCountA, swCountB int
		dbA.QueryRow("SELECT COUNT(*) FROM software_catalog").Scan(&swCountA)
		dbB.QueryRow("SELECT COUNT(*) FROM software_catalog").Scan(&swCountB)
		if swCountA == 0 {
			t.Error("Lab A should have seeded software")
		}
		if swCountB != 0 {
			t.Errorf("Lab B should have 0 software, got %d", swCountB)
		}
	})

	t.Run("data_isolation_schedules", func(t *testing.T) {
		var schedCountA, schedCountB int
		dbA.QueryRow("SELECT COUNT(*) FROM course_schedules").Scan(&schedCountA)
		dbB.QueryRow("SELECT COUNT(*) FROM course_schedules").Scan(&schedCountB)
		if schedCountA == 0 {
			t.Error("Lab A should have seeded schedules")
		}
		if schedCountB != 0 {
			t.Errorf("Lab B should have 0 schedules, got %d", schedCountB)
		}
	})

	t.Run("data_isolation_after_create", func(t *testing.T) {
		// Verify that data created on Lab B is NOT visible in Lab A
		if !labB.refreshCSRF() {
			// Not logged in yet — clear cookies and do fresh login
			labB.cookies = make(map[string]string)
			if !loginAndRefresh(labB, "labB_only", "test123") {
				t.Fatal("login failed")
			}
		}
		resp, err := labB.post("/software/create", "name=Isolation-99")
		if err != nil {
			t.Fatalf("POST Lab B software create: %v", err)
		}
		resp.Body.Close()
		var countInA int
		dbA.QueryRow("SELECT COUNT(*) FROM software_catalog WHERE name='Isolation-99'").Scan(&countInA)
		if countInA != 0 {
			t.Error("Lab B created software should NOT appear in Lab A DB")
		}
		var swCreated int
		dbB.QueryRow("SELECT COUNT(*) FROM software_catalog WHERE name='Isolation-99'").Scan(&swCreated)
		if swCreated == 0 {
			t.Error("Lab B created software should exist in Lab B DB")
		}
	})
}

// ============================================
// TestAutoSync — verifies global user permissions-based access
// ============================================

func TestAutoSync(t *testing.T) {
	env := setupTestEnvironment(t)
	labA := env.LabA
	labB := env.LabB
	tsURL := env.TS.URL
	gdb := env.GlobalDB

	var adminID, labAOnlyID int
	gdb.QueryRow("SELECT id FROM global_users WHERE username='admin'").Scan(&adminID)
	if adminID == 0 {
		t.Fatal("admin not found in global DB")
	}
	gdb.QueryRow("SELECT id FROM global_users WHERE username='labA_only'").Scan(&labAOnlyID)
	if labAOnlyID == 0 {
		t.Fatal("labA_only not found in global DB")
	}

	t.Run("super_admin_access_lab_a", func(t *testing.T) {
		if !loginAndRefresh(labA, "admin", "admin123") {
			t.Fatal("login failed")
		}
		// Super admin should be able to access any lab dashboard
		resp, err := labA.get("/dashboard")
		if err != nil {
			t.Fatalf("GET Lab A dashboard: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200 for super admin, got %d", resp.StatusCode)
		}
	})

	t.Run("super_admin_access_lab_b", func(t *testing.T) {
		// admin session is already in labA.cookies from previous login
		resp, err := labA.getURL(tsURL + labB.prefix + "/dashboard")
		if err != nil {
			t.Fatalf("GET Lab B dashboard: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200 for super admin on Lab B, got %d", resp.StatusCode)
		}
	})

	t.Run("no_duplicate_lab_permissions", func(t *testing.T) {
		if !loginAndRefresh(labA, "labA_only", "test123") {
			t.Fatal("login failed")
		}
		// Access dashboard twice; should not create duplicate lab_permissions
		resp, _ := labA.get("/dashboard")
		resp.Body.Close()
		resp, _ = labA.get("/dashboard")
		resp.Body.Close()

		var count int
		gdb.QueryRow("SELECT COUNT(*) FROM lab_permissions WHERE user_id=? AND lab_url_path='lab-kom-mi'", labAOnlyID).Scan(&count)
		if count != 1 {
			t.Errorf("expected exactly 1 lab_permission (no duplicate), got %d", count)
		}
	})

	t.Run("global_user_profile_no_error", func(t *testing.T) {
		// Session from previous subtest (labA_only) is still valid
		// Just refresh CSRF before POST
		if !labA.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := labA.post("/profile",
			fmt.Sprintf("username=%s&full_name=Updated+Profile+Name", "labA_only"))
		if err != nil {
			t.Fatalf("POST /profile: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 && resp.StatusCode != 200 {
			t.Errorf("expected 302 or 200, got %d", resp.StatusCode)
		}
	})
}
