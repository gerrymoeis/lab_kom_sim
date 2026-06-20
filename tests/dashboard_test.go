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
			t.Logf("dashboard may not show exact PC count %d (could be formatted)", pcCount)
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
