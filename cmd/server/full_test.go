package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"

	"github.com/gin-gonic/gin"
)

func TestFullIntegration(t *testing.T) {
	wd, _ := os.Getwd()
	projectRoot := filepath.Dir(filepath.Dir(wd))
	os.Chdir(projectRoot)
	defer os.Chdir(wd)
	defer os.Remove("full_testing.db")
	defer os.RemoveAll("uploads")
	// ── Setup ────────────────────────────────────────────────────
	dbPath := "full_testing.db"
	cfg := &config.Config{
		DatabasePath:  dbPath,
		SessionSecret: "test-secret-12345",
		UploadPath:    "uploads",
	}

	db, err := database.InitDB(dbPath, "")
	if err != nil { t.Fatalf("InitDB: %v", err) }

	if err := database.RunMigrations(db, false); err != nil { t.Fatalf("Migrate: %v", err) }
	if err := database.SeedDefaultUser(db); err != nil { t.Errorf("Seed user: %v", err) }
	db.Exec("UPDATE users SET session_token = NULL")

	gin.SetMode(gin.ReleaseMode)
	router := setupRouter(db, cfg)
	ts := httptest.NewServer(router)
	defer ts.Close()

	noRedirect := func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	client := &http.Client{CheckRedirect: noRedirect}
	jar := make(map[string]string)
	saveCookies := func(resp *http.Response) {
		for _, c := range resp.Cookies() { jar[c.Name] = c.Value }
	}
	addCookies := func(req *http.Request) {
		for n, v := range jar { req.AddCookie(&http.Cookie{Name: n, Value: v}) }
	}

	login := func() bool {
		req, _ := http.NewRequest("POST", ts.URL+"/login", strings.NewReader("username=admin&password=admin123"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := client.Do(req)
		if err != nil { t.Logf("Login error: %v", err); return false }
		defer resp.Body.Close()
		saveCookies(resp)
		if resp.StatusCode != 302 {
			body := make([]byte, 200)
			resp.Body.Read(body)
			t.Logf("Login returned %d: %s", resp.StatusCode, string(body))
		}
		return resp.StatusCode == 302 && len(jar) > 0
	}

	httpGet := func(path string) (*http.Response, error) {
		req, _ := http.NewRequest("GET", ts.URL+path, nil)
		addCookies(req)
		return client.Do(req)
	}
	httpPost := func(path, data string) (*http.Response, error) {
		req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(data))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		addCookies(req)
		return client.Do(req)
	}

	assert := func(cond bool, msg string, args ...interface{}) {
		if !cond { t.Errorf("FAIL: "+msg, args...) }
	}

	// ── 1. Login ─────────────────────────────────────────────────
	t.Log("\n=== 1. LOGIN ===")
	assert(login(), "Login should set session cookie")

	// Verify dashboard accessible
	resp, err := httpGet("/dashboard")
	assert(err == nil && resp.StatusCode == 200, "/dashboard returns 200")
	resp.Body.Close()

	// ── 2. PC CRUD ──────────────────────────────────────────────
	t.Log("\n=== 2. PC CRUD ===")
	resp, _ = httpGet("/pc")
	assert(resp.StatusCode == 200, "/pc list: %d", resp.StatusCode)
	resp.Body.Close()

	var pcCount int
	db.QueryRow("SELECT COUNT(*) FROM pcs").Scan(&pcCount)
	assert(pcCount > 0, "PCs seeded: %d", pcCount)

	resp, _ = httpGet("/pc/1")
	assert(resp.StatusCode == 200, "/pc/1 detail: %d", resp.StatusCode)
	resp.Body.Close()

	resp, _ = httpGet("/pc/1/edit")
	assert(resp.StatusCode == 200, "/pc/1 edit: %d", resp.StatusCode)
	resp.Body.Close()

	// ── 3. Device CRUD ───────────────────────────────────────────
	t.Log("\n=== 3. DEVICE CRUD ===")
	resp, _ = httpGet("/devices")
	assert(resp.StatusCode == 200, "/devices: %d", resp.StatusCode)
	resp.Body.Close()

	resp, _ = httpGet("/devices?tab=types")
	assert(resp.StatusCode == 200, "/devices types: %d", resp.StatusCode)
	resp.Body.Close()

	resp, _ = httpPost("/device-types/create", "name=TestKeyboard&category=peripheral&brand=TestB&model=T100&item_type=individual")
	assert(resp.StatusCode == 302, "create device type redirect: %d", resp.StatusCode)
	resp.Body.Close()

	var dtID int
	db.QueryRow("SELECT id FROM device_types WHERE name='TestKeyboard'").Scan(&dtID)
	assert(dtID > 0, "Device type created, ID=%d", dtID)

	resp, _ = httpPost("/devices/create", "device_type_id=1&name=MonitorLG&brand=LG&model=27UL500&item_type=individual&item_mode=loanable&quantity_total=5&condition=baik&location=Lab1")
	assert(resp.StatusCode == 302, "create device redirect: %d", resp.StatusCode)
	resp.Body.Close()

	resp, _ = httpGet("/devices")
	assert(resp.StatusCode == 200, "/devices after create: %d", resp.StatusCode)
	resp.Body.Close()

	// ── 4. Software CRUD ─────────────────────────────────────────
	t.Log("\n=== 4. SOFTWARE CRUD ===")
	resp, _ = httpGet("/software")
	assert(resp.StatusCode == 200, "/software: %d", resp.StatusCode)
	resp.Body.Close()

	resp, _ = httpPost("/software/create", "name=TestApp123&category=other&description=Test")
	assert(resp.StatusCode == 302, "create software redirect: %d", resp.StatusCode)
	resp.Body.Close()

	var swID int
	db.QueryRow("SELECT id FROM software_catalog WHERE name='TestApp123'").Scan(&swID)
	assert(swID > 0, "Software created, ID=%d", swID)

	resp, _ = httpGet("/software/" + fmt.Sprint(swID) + "/edit")
	assert(resp.StatusCode == 200, "/software/%d/edit: %d", swID, resp.StatusCode)
	resp.Body.Close()

	resp, _ = httpPost("/software/"+fmt.Sprint(swID)+"/delete", "")
	assert(resp.StatusCode == 302, "delete software redirect: %d", resp.StatusCode)
	resp.Body.Close()

	var swCount int
	db.QueryRow("SELECT COUNT(*) FROM software_catalog WHERE id=?", swID).Scan(&swCount)
	assert(swCount == 0, "Software deleted")

	// ── 5. Schedule CRUD ─────────────────────────────────────────
	t.Log("\n=== 5. SCHEDULE CRUD ===")
	resp, _ = httpGet("/schedules")
	assert(resp.StatusCode == 200, "/schedules: %d", resp.StatusCode)
	resp.Body.Close()

	resp, _ = httpPost("/schedules/create", "course_name=Algoritma&lecturer=Dr.Test&day=Senin&class=IF-1&time_start=08:00&time_end=09:40")
	assert(resp.StatusCode == 302, "create schedule redirect: %d", resp.StatusCode)
	resp.Body.Close()

	var scID int
	db.QueryRow("SELECT id FROM course_schedules WHERE course_name='Algoritma'").Scan(&scID)
	assert(scID > 0, "Schedule created, ID=%d", scID)

	// ── 6. Logbook CRUD ─────────────────────────────────────────
	t.Log("\n=== 6. LOGBOOK CRUD ===")
	resp, _ = httpGet("/logbook")
	assert(resp.StatusCode == 200, "/logbook: %d", resp.StatusCode)
	resp.Body.Close()

	resp, _ = httpPost("/logbook/create", "date=2026-05-16&student_name=Mahasiswa+Test&nim=24091234567&time_in=08:00&time_out=09:40&purpose=Praktikum")
	assert(resp.StatusCode == 302, "create logbook redirect: %d", resp.StatusCode)
	resp.Body.Close()

	var lbCount int
	db.QueryRow("SELECT COUNT(*) FROM logbook_entries").Scan(&lbCount)
	assert(lbCount > 0, "Logbook entries: %d", lbCount)

	// ── 7. User management ───────────────────────────────────────
	t.Log("\n=== 7. USER MANAGEMENT ===")
	resp, _ = httpGet("/admin/users")
	assert(resp.StatusCode == 200, "/admin/users: %d", resp.StatusCode)
	resp.Body.Close()

	resp, _ = httpGet("/profile")
	assert(resp.StatusCode == 200, "/profile: %d", resp.StatusCode)
	resp.Body.Close()

	resp, _ = httpPost("/profile", "username=admin&full_name=Admin+Utama")
	assert(resp.StatusCode == 302, "profile update redirect: %d", resp.StatusCode)
	resp.Body.Close()

	// ── 8. Verify Activity Logs ──────────────────────────────────
	t.Log("\n=== 8. ACTIVITY LOGS ===")
	var logCount int
	db.QueryRow("SELECT COUNT(*) FROM activity_logs").Scan(&logCount)
	assert(logCount > 0, "Activity logs exist: %d", logCount)

	resp, _ = httpGet("/admin/activity-logs")
	assert(resp.StatusCode == 200, "/admin/activity-logs: %d", resp.StatusCode)
	resp.Body.Close()

	// ── 9. Summary ──────────────────────────────────────────────
	t.Log("\n=== SUMMARY ===")
	var tables []string
	rows, _ := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if rows != nil {
		defer rows.Close()
		for rows.Next() { var n string; rows.Scan(&n); tables = append(tables, n) }
	}
	for _, tbl := range tables {
		var c int
		db.QueryRow("SELECT COUNT(*) FROM " + tbl).Scan(&c)
		if c > 0 { t.Logf("  %s: %d rows", tbl, c) }
	}
	t.Logf("  All tests passed!")
}
