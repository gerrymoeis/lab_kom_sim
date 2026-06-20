package tests

import (
	"testing"
)

func TestPaginationPC(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}

	t.Run("default_page", func(t *testing.T) {
		resp, err := lab.get("/pc")
		if err != nil {
			t.Fatalf("GET /pc: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("explicit_page_1", func(t *testing.T) {
		resp, err := lab.get("/pc?page=1")
		if err != nil {
			t.Fatalf("GET /pc?page=1: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("page_2", func(t *testing.T) {
		resp, err := lab.get("/pc?page=2")
		if err != nil {
			t.Fatalf("GET /pc?page=2: %v", err)
		}
		defer resp.Body.Close()
		// Page 2 may be empty if data < page size, but should still succeed
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("search_param", func(t *testing.T) {
		resp, err := lab.get("/pc?search=pc-1")
		if err != nil {
			t.Fatalf("GET /pc?search=pc-1: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("filter_status", func(t *testing.T) {
		resp, err := lab.get("/pc?status=normal")
		if err != nil {
			t.Fatalf("GET /pc?status=normal: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("filter_placement", func(t *testing.T) {
		resp, err := lab.get("/pc?placement=dipakai")
		if err != nil {
			t.Fatalf("GET /pc?placement=dipakai: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("sort_param", func(t *testing.T) {
		resp, err := lab.get("/pc?sort_by=label&sort_order=ASC")
		if err != nil {
			t.Fatalf("GET /pc?sort_by=label&sort_order=ASC: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestPaginationSoftware(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}

	t.Run("default_page", func(t *testing.T) {
		resp, err := lab.get("/software")
		if err != nil {
			t.Fatalf("GET /software: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("search_param", func(t *testing.T) {
		resp, err := lab.get("/software?search=excel")
		if err != nil {
			t.Fatalf("GET /software?search=excel: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("filter_category", func(t *testing.T) {
		resp, err := lab.get("/software?category=required")
		if err != nil {
			t.Fatalf("GET /software?category=required: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestPaginationSchedule(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}

	t.Run("default_page", func(t *testing.T) {
		resp, err := lab.get("/schedules")
		if err != nil {
			t.Fatalf("GET /schedules: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("filter_day", func(t *testing.T) {
		resp, err := lab.get("/schedules?day=Senin")
		if err != nil {
			t.Fatalf("GET /schedules?day=Senin: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("search_param", func(t *testing.T) {
		resp, err := lab.get("/schedules?search=algo")
		if err != nil {
			t.Fatalf("GET /schedules?search=algo: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestPaginationDevice(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}

	t.Run("default_page", func(t *testing.T) {
		resp, err := lab.get("/devices")
		if err != nil {
			t.Fatalf("GET /devices: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("tab_loans", func(t *testing.T) {
		resp, err := lab.get("/devices?tab=loans")
		if err != nil {
			t.Fatalf("GET /devices?tab=loans: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("filter_condition", func(t *testing.T) {
		resp, err := lab.get("/devices?condition=normal")
		if err != nil {
			t.Fatalf("GET /devices?condition=normal: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("tab_usages_filter_available", func(t *testing.T) {
		resp, err := lab.get("/devices?tab=usages&is_available=yes")
		if err != nil {
			t.Fatalf("GET /devices?tab=usages&is_available=yes: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("tab_installations", func(t *testing.T) {
		resp, err := lab.get("/devices?tab=installations")
		if err != nil {
			t.Fatalf("GET /devices?tab=installations: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestPaginationLogbook(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}

	t.Run("default_page", func(t *testing.T) {
		resp, err := lab.get("/logbook")
		if err != nil {
			t.Fatalf("GET /logbook: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("date_filter", func(t *testing.T) {
		resp, err := lab.get("/logbook?date_from=2026-01-01&date_to=2026-12-31")
		if err != nil {
			t.Fatalf("GET /logbook?date_from=2026-01-01&date_to=2026-12-31: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestPaginationActivityLog(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}

	t.Run("default_page", func(t *testing.T) {
		resp, err := lab.get("/admin/activity-logs")
		if err != nil {
			t.Fatalf("GET /admin/activity-logs: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("filter_action", func(t *testing.T) {
		resp, err := lab.get("/admin/activity-logs?action=create")
		if err != nil {
			t.Fatalf("GET /admin/activity-logs?action=create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("date_range_filter", func(t *testing.T) {
		resp, err := lab.get("/admin/activity-logs?date_from=2026-01-01&date_to=2026-12-31")
		if err != nil {
			t.Fatalf("GET /admin/activity-logs?date_from=2026-01-01&date_to=2026-12-31: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestPaginationUser(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}

	t.Run("default_page", func(t *testing.T) {
		resp, err := lab.get("/admin/users")
		if err != nil {
			t.Fatalf("GET /admin/users: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("search_param", func(t *testing.T) {
		resp, err := lab.get("/admin/users?search=admin")
		if err != nil {
			t.Fatalf("GET /admin/users?search=admin: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("filter_role", func(t *testing.T) {
		resp, err := lab.get("/admin/users?role=admin")
		if err != nil {
			t.Fatalf("GET /admin/users?role=admin: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestPaginationInvalidParams(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}

	t.Run("page_non_numeric", func(t *testing.T) {
		resp, err := lab.get("/pc?page=abc")
		if err != nil {
			t.Fatalf("GET /pc?page=abc: %v", err)
		}
		defer resp.Body.Close()
		// Should gracefully default to page 1
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("page_negative", func(t *testing.T) {
		resp, err := lab.get("/pc?page=-1")
		if err != nil {
			t.Fatalf("GET /pc?page=-1: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("page_zero", func(t *testing.T) {
		resp, err := lab.get("/pc?page=0")
		if err != nil {
			t.Fatalf("GET /pc?page=0: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("page_exceeds_max", func(t *testing.T) {
		resp, err := lab.get("/pc?page=9999")
		if err != nil {
			t.Fatalf("GET /pc?page=9999: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("empty_search", func(t *testing.T) {
		resp, err := lab.get("/pc?search=")
		if err != nil {
			t.Fatalf("GET /pc?search=: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("combined_pattern", func(t *testing.T) {
		// Multiple query params together
		resp, err := lab.get("/software?page=1&search=office&category=required")
		if err != nil {
			t.Fatalf("GET /software?page=1&search=office&category=required: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestPaginationLabBEmpty(t *testing.T) {
	// Verify empty Lab B list pages also handle pagination gracefully
	env := setupTestEnvironment(t)
	lab := env.LabB
	if !loginAndRefresh(lab, "labB_only", "test123") {
		t.Fatal("login failed")
	}

	for _, ep := range []struct{ name, path string }{
		{"pc", "/pc?page=1"},
		{"software", "/software?page=1"},
		{"schedules", "/schedules?page=1"},
		{"devices", "/devices?page=1"},
	} {
		ep := ep
		t.Run("list_"+ep.name, func(t *testing.T) {
			resp, err := lab.get(ep.path)
			if err != nil {
				t.Fatalf("GET %s: %v", ep.path, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Errorf("expected 200, got %d", resp.StatusCode)
			}
		})
	}
}
