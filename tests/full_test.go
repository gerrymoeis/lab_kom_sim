package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/server"
)

func TestFullIntegration(t *testing.T) {
	// Change to project root (tests/ → project root)
	wd, _ := os.Getwd()
	projectRoot := filepath.Dir(wd)
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
	defer db.Close()

	if err := database.RunMigrations(db, false); err != nil { t.Fatalf("Migrate: %v", err) }
	if err := database.SeedDefaultUser(db); err != nil { t.Errorf("Seed user: %v", err) }
	db.Exec("UPDATE users SET session_token = NULL")

	router := server.SetupRouter(db, cfg)
	ts := httptest.NewServer(router)
	defer ts.Close()

	noRedirect := func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
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
		if err != nil { return false }
		defer resp.Body.Close()
		saveCookies(resp)
		return resp.StatusCode == 302 && len(jar) > 0
	}

	get := func(path string) (*http.Response, error) {
		req, _ := http.NewRequest("GET", ts.URL+path, nil)
		addCookies(req)
		return client.Do(req)
	}
	post := func(path, data string) (*http.Response, error) {
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
	resp, err := get("/dashboard")
	assert(err == nil && resp.StatusCode == 200, "/dashboard returns 200")
	resp.Body.Close()

	// ── 2. PC CRUD ──────────────────────────────────────────────
	t.Log("\n=== 2. PC CRUD ===")
	resp, _ = get("/pc")
	assert(resp.StatusCode == 200, "/pc list: %d", resp.StatusCode)
	resp.Body.Close()

	var pcCount int
	db.QueryRow("SELECT COUNT(*) FROM pcs").Scan(&pcCount)
	assert(pcCount > 0, "PCs seeded: %d", pcCount)

	resp, _ = get("/pc/1")
	assert(resp.StatusCode == 200, "/pc/1: %d", resp.StatusCode)
	resp.Body.Close()
	resp, _ = get("/pc/1/edit")
	assert(resp.StatusCode == 200, "/pc/1 edit: %d", resp.StatusCode)
	resp.Body.Close()

	// ── 2b. PC Photo Upload ─────────────────────────────────────
	t.Log("\n=== 2b. PC PHOTO UPLOAD ===")
	photoData, _ := os.ReadFile(filepath.Join("tests", "resources", "logbook.jpeg"))

	var photoBuf bytes.Buffer
	mw := multipart.NewWriter(&photoBuf)
	fw, _ := mw.CreateFormFile("image", "logbook.jpeg")
	fw.Write(photoData)
	mw.WriteField("type", "serial")
	mw.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/upload-image", &photoBuf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	addCookies(req)
	resp, err = client.Do(req)
	assert(err == nil, "upload image request")
	var uploadRes struct {
		Success bool   `json:"success"`
		FileRef string `json:"file_ref"`
	}
	json.NewDecoder(resp.Body).Decode(&uploadRes)
	resp.Body.Close()
	assert(uploadRes.Success && uploadRes.FileRef != "", "upload image: file_ref=%s", uploadRes.FileRef)

	resp, _ = post("/pc/1/edit",
		"status=normal&serial_number=SN001&operating_system=Win11&device_type=PC&brand_model=Dell&accessories=KB&processor=i7&ram=16GB&storage=512GB&notes=&action_notes=&serial_file_ref="+uploadRes.FileRef)
	assert(resp.StatusCode == 302, "PC edit with photo: %d", resp.StatusCode)
	resp.Body.Close()

	var photoSerial string
	db.QueryRow("SELECT COALESCE(photo_serial,'') FROM pcs WHERE pc_number=1").Scan(&photoSerial)
	assert(photoSerial != "", "photo_serial saved: %s", photoSerial)

	// ── 3. Device CRUD ───────────────────────────────────────────
	t.Log("\n=== 3. DEVICE CRUD ===")
	resp, _ = get("/devices")
	assert(resp.StatusCode == 200, "/devices: %d", resp.StatusCode)
	resp.Body.Close()
	resp, _ = get("/devices?tab=types")
	assert(resp.StatusCode == 200, "/devices types: %d", resp.StatusCode)
	resp.Body.Close()

	resp, _ = post("/device-types/create", "name=TestKB&category=peripheral&brand=B&model=T100&item_type=individual")
	assert(resp.StatusCode == 302, "create device type: %d", resp.StatusCode)
	resp.Body.Close()
	var dtID int
	db.QueryRow("SELECT id FROM device_types WHERE name='TestKB'").Scan(&dtID)
	assert(dtID > 0, "Device type ID=%d", dtID)

	resp, _ = post("/devices/create", "device_type_id=1&name=Monitor&brand=LG&model=27&item_type=individual&item_mode=loanable&quantity_total=5&condition=baik&location=Lab")
	assert(resp.StatusCode == 302, "create device: %d", resp.StatusCode)
	resp.Body.Close()

	// ── 4. Software CRUD ─────────────────────────────────────────
	t.Log("\n=== 4. SOFTWARE CRUD ===")
	resp, _ = get("/software")
	assert(resp.StatusCode == 200, "/software: %d", resp.StatusCode)
	resp.Body.Close()

	resp, _ = post("/software/create", "name=TestSW&category=other&description=Test")
	assert(resp.StatusCode == 302, "create software: %d", resp.StatusCode)
	resp.Body.Close()
	var swID int
	db.QueryRow("SELECT id FROM software_catalog WHERE name='TestSW'").Scan(&swID)
	assert(swID > 0, "Software ID=%d", swID)

	resp, _ = get("/software/"+fmt.Sprint(swID)+"/edit")
	assert(resp.StatusCode == 200, "/software/%d/edit: %d", swID, resp.StatusCode)
	resp.Body.Close()

	resp, _ = post("/software/"+fmt.Sprint(swID)+"/delete", "")
	assert(resp.StatusCode == 302, "delete software: %d", resp.StatusCode)
	resp.Body.Close()
	db.QueryRow("SELECT COUNT(*) FROM software_catalog WHERE id=?", swID).Scan(&swID)
	assert(swID == 0, "Software deleted")

	// ── 5. Schedule CRUD ─────────────────────────────────────────
	t.Log("\n=== 5. SCHEDULE CRUD ===")
	resp, _ = get("/schedules")
	assert(resp.StatusCode == 200, "/schedules: %d", resp.StatusCode)
	resp.Body.Close()

	resp, _ = post("/schedules/create", "course_name=Algo&lecturer=Dr.T&day=Senin&class=IF-1&time_start=08:00&time_end=09:40")
	assert(resp.StatusCode == 302, "create schedule: %d", resp.StatusCode)
	resp.Body.Close()
	var scID int
	db.QueryRow("SELECT id FROM course_schedules WHERE course_name='Algo'").Scan(&scID)
	assert(scID > 0, "Schedule ID=%d", scID)

	// ── 6. Logbook CRUD ─────────────────────────────────────────
	t.Log("\n=== 6. LOGBOOK CRUD ===")
	resp, _ = get("/logbook")
	assert(resp.StatusCode == 200, "/logbook: %d", resp.StatusCode)
	resp.Body.Close()

	resp, _ = post("/logbook/create", "date=2026-05-16&student_name=Mhs+Test&nim=24091234567&time_in=08:00&time_out=09:40&purpose=Prak")
	assert(resp.StatusCode == 302, "create logbook: %d", resp.StatusCode)
	resp.Body.Close()
	var lb int
	db.QueryRow("SELECT COUNT(*) FROM logbook_entries").Scan(&lb)
	assert(lb > 0, "Logbook entries: %d", lb)

	// ── 6b. Logbook Upload ──────────────────────────────────────
	t.Log("\n=== 6b. LOGBOOK UPLOAD ===")
	photoBuf.Reset()
	mw = multipart.NewWriter(&photoBuf)
	fw, _ = mw.CreateFormFile("logbook_image", "logbook.jpeg")
	fw.Write(photoData)
	mw.Close()

	req, _ = http.NewRequest("POST", ts.URL+"/logbook/upload", &photoBuf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	addCookies(req)
	resp, err = client.Do(req)
	assert(err == nil, "logbook upload")
	resp.Body.Close()
	logbookFiles, _ := os.ReadDir("uploads/logbook")
	assert(len(logbookFiles) > 0, "logbook file saved to uploads/logbook/")

	// ── 7. User management ───────────────────────────────────────
	t.Log("\n=== 7. USER ===")
	resp, _ = get("/admin/users")
	assert(resp.StatusCode == 200, "/admin/users: %d", resp.StatusCode)
	resp.Body.Close()
	resp, _ = get("/profile")
	assert(resp.StatusCode == 200, "/profile: %d", resp.StatusCode)
	resp.Body.Close()
	resp, _ = post("/profile", "username=admin&full_name=Admin+U")
	assert(resp.StatusCode == 302, "profile update: %d", resp.StatusCode)
	resp.Body.Close()

	// ── 8. Activity Logs ────────────────────────────────────────
	t.Log("\n=== 8. ACTIVITY LOGS ===")
	var logCount int
	db.QueryRow("SELECT COUNT(*) FROM activity_logs").Scan(&logCount)
	assert(logCount > 0, "Activity logs: %d", logCount)
	resp, _ = get("/admin/activity-logs")
	assert(resp.StatusCode == 200, "/admin/activity-logs: %d", resp.StatusCode)
	resp.Body.Close()

	// ── 9. Summary ──────────────────────────────────────────────
	t.Log("\n=== SUMMARY ===")
	rows, _ := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var tbl string; rows.Scan(&tbl)
			var c int
			db.QueryRow("SELECT COUNT(*) FROM " + tbl).Scan(&c)
			if c > 0 { t.Logf("  %s: %d rows", tbl, c) }
		}
	}
	t.Logf("  All tests passed!")
}
