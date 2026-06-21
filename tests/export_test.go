package tests

import (
	"io"
	"strings"
	"testing"
)

func checkExport(t *testing.T, lab *testLab, path, prefix string) {
	resp, err := lab.get(path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET %s: expected 200, got %d", path, resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Errorf("GET %s: expected excel Content-Type, got %q", path, ct)
	}
	cd := resp.Header.Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment; filename="+prefix) {
		t.Errorf("GET %s: expected Content-Disposition prefix %q, got %q", path, "attachment; filename="+prefix, cd)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Errorf("GET %s: empty response body", path)
	}
}

func TestExportPC(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}
	checkExport(t, lab, "/pc/export", "pc_export_")
}

func TestExportSoftware(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}
	checkExport(t, lab, "/software/export", "software_catalog_export_")
}

func TestExportDevice(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}
	checkExport(t, lab, "/devices/export", "devices_export_")
}

func TestExportLogbook(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}
	checkExport(t, lab, "/logbook/export", "logbook_export_")
	checkExport(t, lab, "/logbook/export-preview", "logbook_export_preview_")
}

func TestExportActivityLog(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "admin", "admin123") {
		t.Fatal("login failed")
	}
	checkExport(t, lab, "/admin/activity-logs/export", "activity_log_export_")
}


