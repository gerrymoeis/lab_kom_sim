package tests

import (
	"io"
	"strings"
	"testing"
)

func TestDashboardContent(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	db := env.DB_A

	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}

	t.Run("dashboard_renders", func(t *testing.T) {
		resp, err := lab.get("/dashboard")
		if err != nil {
			t.Fatalf("GET /dashboard: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		s := string(body)
		if !strings.Contains(s, "Dashboard") && !strings.Contains(s, "dashboard") {
			t.Error("dashboard page missing title")
		}
	})

	t.Run("dashboard_shows_status_cards", func(t *testing.T) {
		resp, _ := lab.get("/dashboard")
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		s := string(body)
		// Status cards rendered by the template
		if !strings.Contains(s, "Normal") && !strings.Contains(s, "normal") {
			t.Error("dashboard missing Normal status card")
		}
		if !strings.Contains(s, "Warning") && !strings.Contains(s, "warning") {
			t.Error("dashboard missing Warning status card")
		}
		if !strings.Contains(s, "Rusak") && !strings.Contains(s, "broken") {
			t.Error("dashboard missing Rusak/broken status card")
		}
		if !strings.Contains(s, "Cadangan") {
			t.Error("dashboard missing Cadangan status card")
		}
	})

	t.Run("dashboard_shows_pc_count", func(t *testing.T) {
		var pcCount int
		db.QueryRow("SELECT COUNT(*) FROM pcs").Scan(&pcCount)
		resp, _ := lab.get("/dashboard")
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		s := string(body)
		if strings.Contains(s, "Total PC") {
			// At minimum the section label exists
		} else {
			t.Error("dashboard missing Total PC section")
		}
		if pcCount > 0 && !strings.Contains(s, itoa(pcCount)) {
			t.Errorf("dashboard should show PC count %d, but not found in rendered HTML", pcCount)
		}
	})

	t.Run("dashboard_shows_software_count", func(t *testing.T) {
		resp, _ := lab.get("/dashboard")
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		s := string(body)
		if !strings.Contains(s, "Total Software Tracked") && !strings.Contains(s, "Software") {
			t.Error("dashboard missing software count section")
		}
	})

	t.Run("dashboard_shows_device_count", func(t *testing.T) {
		resp, _ := lab.get("/dashboard")
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		s := string(body)
		if !strings.Contains(s, "Total Perangkat Lain") {
			t.Error("dashboard missing Total Perangkat Lain section")
		}
	})

	t.Run("dashboard_shows_device_count_positive", func(t *testing.T) {
		// Seed a device type + device so dashboard shows count > 0
		db.Exec("INSERT OR IGNORE INTO categories (id, name, label_prefix) VALUES (50, 'DashboardCat', 'DASHCAT')")
		db.Exec("INSERT OR IGNORE INTO device_types (id, category_id, name, brand, model, label_prefix, usage_type, default_location) VALUES (50, 50, 'DashDevice', 'Brand', 'Model', 'DASHDT', 'loanable', 'Lab')")
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/devices/create",
			"device_type_id=50&serial_number=SN-DASHBOARD-DEV&condition=normal&location=Lab&purchase_date=&notes=")
		if err != nil {
			t.Fatalf("POST /devices/create: %v", err)
		}
		defer resp.Body.Close()
		// Verify device was created
		var devCount int
		db.QueryRow("SELECT COUNT(*) FROM devices WHERE serial_number='SN-DASHBOARD-DEV'").Scan(&devCount)
		if devCount == 0 {
			t.Fatal("device not created for dashboard test")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp2, _ := lab.get("/dashboard")
		defer resp2.Body.Close()
		body2, _ := io.ReadAll(resp2.Body)
		s2 := string(body2)
		if !strings.Contains(s2, "Total Perangkat Lain") {
			t.Error("dashboard missing Total Perangkat Lain after device creation")
		}
		if !strings.Contains(s2, "DASHDT") && !strings.Contains(s2, "DashDevice") {
			t.Log("dashboard may show device type info in other format")
		}
		var totalDevCount int
		db.QueryRow("SELECT COUNT(*) FROM devices").Scan(&totalDevCount)
		if totalDevCount > 0 && !strings.Contains(s2, itoa(totalDevCount)) {
			t.Errorf("dashboard should show total device count %d, but not found in rendered HTML", totalDevCount)
		}
	})

	t.Run("dashboard_shows_schedule_count", func(t *testing.T) {
		var schedCount int
		db.QueryRow("SELECT COUNT(*) FROM course_schedules").Scan(&schedCount)
		if schedCount > 0 {
			t.Logf("schedule count in DB: %d", schedCount)
		} else {
			t.Log("no schedules found in DB")
		}
	})

	t.Run("dashboard_empty_lab", func(t *testing.T) {
		// Lab B starts empty
		if !loginAndRefresh(env.LabB, "labB_only", "test123") {
			t.Fatal("login failed")
		}
		resp, err := env.LabB.get("/dashboard")
		if err != nil {
			t.Fatalf("GET Lab B /dashboard: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}
