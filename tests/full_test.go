package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/server"

	"github.com/joho/godotenv"
)

func findProjectRoot(wd string) string {
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
		dir = parent
	}
}

type testLab struct {
	url    string
	id     string
	prefix string
	db     *database.DB
	cfg    config.LabConfig

	cookies map[string]string
	csrf    string
	ts      *httptest.Server
	t       *testing.T
	client  *http.Client
}

func (l *testLab) closeResp(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func (l *testLab) saveCookies(resp *http.Response) {
	for _, c := range resp.Cookies() {
		l.cookies[c.Name] = c.Value
	}
}

func (l *testLab) addCookies(req *http.Request) {
	for n, v := range l.cookies {
		req.AddCookie(&http.Cookie{Name: n, Value: v})
	}
}

func (l *testLab) extractCSRFToken(html string) string {
	prefix := `<meta name="csrf-token" content="`
	start := strings.Index(html, prefix)
	if start == -1 {
		return ""
	}
	start += len(prefix)
	end := strings.Index(html[start:], `"`)
	if end == -1 {
		return ""
	}
	return html[start : start+end]
}

func (l *testLab) login(username, password string) bool {
	req, _ := http.NewRequest("GET", l.ts.URL+l.prefix+"/login", nil)
	resp, err := l.client.Do(req)
	if err != nil {
		return false
	}
	l.saveCookies(resp)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	token := l.extractCSRFToken(string(body))
	if token == "" {
		return false
	}

	formData := "_csrf=" + url.QueryEscape(token) + "&username=" + url.QueryEscape(username) + "&password=" + url.QueryEscape(password)
	req, _ = http.NewRequest("POST", l.ts.URL+l.prefix+"/login", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	l.addCookies(req)
	resp, err = l.client.Do(req)
	if err != nil {
		return false
	}
	defer l.closeResp(resp)
	l.saveCookies(resp)
	l.csrf = token
	return resp.StatusCode == 302 && len(l.cookies) > 0
}

func (l *testLab) get(path string) (*http.Response, error) {
	req, _ := http.NewRequest("GET", l.ts.URL+l.prefix+path, nil)
	l.addCookies(req)
	return l.client.Do(req)
}

func (l *testLab) post(path, data string) (*http.Response, error) {
	if data == "" {
		data = "_csrf=" + url.QueryEscape(l.csrf)
	} else {
		data = data + "&_csrf=" + url.QueryEscape(l.csrf)
	}
	req, _ := http.NewRequest("POST", l.ts.URL+l.prefix+path, strings.NewReader(data))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	l.addCookies(req)
	return l.client.Do(req)
}

func (l *testLab) postJSON(path, data string) (*http.Response, error) {
	req, _ := http.NewRequest("POST", l.ts.URL+l.prefix+path, strings.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", l.csrf)
	l.addCookies(req)
	return l.client.Do(req)
}

func (l *testLab) getURL(url string) (*http.Response, error) {
	req, _ := http.NewRequest("GET", url, nil)
	l.addCookies(req)
	return l.client.Do(req)
}

func (l *testLab) getLocation(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	return resp.Header.Get("Location")
}

func TestFullIntegration(t *testing.T) {
	wd, _ := os.Getwd()
	projectRoot := findProjectRoot(wd)
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("Chdir to project root %s: %v", projectRoot, err)
	}
	defer os.Chdir(wd)

	// Load .env for API keys (if available)
	godotenv.Load()

	// ---- Setup 2 separate DB files ----
	dbPathA := filepath.Join(projectRoot, "full_testing_a.db")
	dbPathB := filepath.Join(projectRoot, "full_testing_b.db")

	// Cleanup leftover DB files from previous runs
	for _, p := range []string{dbPathA, dbPathA + "-shm", dbPathA + "-wal"} {
		os.Remove(p)
	}
	for _, p := range []string{dbPathB, dbPathB + "-shm", dbPathB + "-wal"} {
		os.Remove(p)
	}
	// Cleanup leftover uploads
	os.RemoveAll(filepath.Join(projectRoot, "uploads", "temp"))
	os.RemoveAll(filepath.Join(projectRoot, "uploads", "pc"))
	os.RemoveAll(filepath.Join(projectRoot, "uploads", "logbook"))

	var dbA, dbB *database.DB
	var err error
	defer func() {
		if dbA != nil { dbA.Close() }
		if dbB != nil { dbB.Close() }
		for _, p := range []string{dbPathA, dbPathA + "-shm", dbPathA + "-wal"} {
			for i := 0; i < 3; i++ {
				if err := os.Remove(p); err == nil {
					break
				} else if os.IsNotExist(err) {
					break
				} else if i < 2 {
					time.Sleep(200 * time.Millisecond)
				} else {
					t.Logf("cleanup %s: %v", filepath.Base(p), err)
				}
			}
		}
		for _, p := range []string{dbPathB, dbPathB + "-shm", dbPathB + "-wal"} {
			for i := 0; i < 3; i++ {
				if err := os.Remove(p); err == nil {
					break
				} else if os.IsNotExist(err) {
					break
				} else if i < 2 {
					time.Sleep(200 * time.Millisecond)
				} else {
					t.Logf("cleanup %s: %v", filepath.Base(p), err)
				}
			}
		}
		os.RemoveAll(filepath.Join(projectRoot, "uploads", "temp"))
		os.RemoveAll(filepath.Join(projectRoot, "uploads", "pc"))
		os.RemoveAll(filepath.Join(projectRoot, "uploads", "logbook"))
	}()

	labAURL := "lab-kom-mi"
	labAID := "MI-1"
	labBURL := "vokasi"
	labBID := "VOKASI-1"

	cfg := &config.Config{
		DatabasePath:     dbPathA,
		SessionSecret:    "test-secret-12345",
		UploadPath:       "uploads",
		DefaultPageSize:  25,
		GeminiAPIKey:     os.Getenv("GEMINI_API_KEY"),
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
	}
	cfg.Labs = []config.LabConfig{
		{ID: labAID, URLPath: labAURL, DBPath: dbPathA, UploadDir: "uploads", Layout: config.GridLayout{ColsPerRow: []int{8, 8, 8, 8, 8}}},
		{ID: labBID, URLPath: labBURL, DBPath: dbPathB, UploadDir: "uploads", Layout: config.GridLayout{ColsPerRow: []int{10, 8, 9, 9}, HasGap: true, GapPos: 4}},
	}

	// Init Lab A DB (has seeds via seeds/mi-1/)
	dbA, err = database.InitDB(dbPathA, "")
	if err != nil {
		t.Fatalf("InitDB lab A: %v", err)
	}
	if err := database.RunMigrations(dbA, false, labAID, labAURL); err != nil {
		t.Fatalf("Migrate lab A: %v", err)
	}
	if err := database.SeedDefaultUser(dbA); err != nil {
		t.Errorf("Seed user lab A: %v", err)
	}
	dbA.Exec("UPDATE users SET session_token = NULL")

	// Init Lab B DB (no seeds — seeds/vokasi-1/ does not exist)
	dbB, err = database.InitDB(dbPathB, "")
	if err != nil {
		t.Fatalf("InitDB lab B: %v", err)
	}
	if err := database.RunMigrations(dbB, false, labBID, labBURL); err != nil {
		t.Fatalf("Migrate lab B: %v", err)
	}
	if err := database.SeedDefaultUser(dbB); err != nil {
		t.Errorf("Seed user lab B: %v", err)
	}
	dbB.Exec("UPDATE users SET session_token = NULL")

	dbs := map[string]*database.DB{labAURL: dbA, labBURL: dbB}
	router, cleanup, flushLogs := server.SetupRouter(dbs, cfg, nil)
	defer cleanup()
	ts := httptest.NewServer(router)
	defer ts.Close()

	noRedirect := func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	client := &http.Client{CheckRedirect: noRedirect}

	// Helper to make raw requests without cookies (for cross-lab tests)
	rawGet := func(url string) (*http.Response, error) {
		req, _ := http.NewRequest("GET", url, nil)
		return client.Do(req)
	}

	// ---- Create testLab instances ----
	labA := &testLab{
		url: labAURL, id: labAID, prefix: "/" + labAURL,
		db: dbA, cfg: cfg.Labs[0], cookies: make(map[string]string),
		ts: ts, t: t, client: client,
	}
	labB := &testLab{
		url: labBURL, id: labBID, prefix: "/" + labBURL,
		db: dbB, cfg: cfg.Labs[1], cookies: make(map[string]string),
		ts: ts, t: t, client: client,
	}

	assert := func(cond bool, msg string, args ...any) {
		if !cond {
			t.Errorf("FAIL: "+msg, args...)
		}
	}

	var resp *http.Response

	// ============================================
	// PHASE 0: Routing Validation
	// ============================================
	t.Log("\n=== 0. ROUTING VALIDATION ===")

	{
		resp, err = rawGet(ts.URL + "/")
		assert(err == nil, "GET / request")
		assert(resp.StatusCode == 200, "GET / returns 200 (landing page): %d", resp.StatusCode)
		labA.closeResp(resp)
	}
	{
		resp, err = rawGet(ts.URL + "/nonexistent/dashboard")
		assert(err == nil, "GET /nonexistent/dashboard request")
		assert(resp.StatusCode == 404, "invalid lab 404: %d", resp.StatusCode)
		labA.closeResp(resp)
	}
	{
		resp, err = rawGet(ts.URL + labA.prefix + "/")
		assert(err == nil, "GET %s/ request", labA.prefix)
		assert(resp.StatusCode == 302, "%s/ redirects to login: %d", labA.prefix, resp.StatusCode)
		assert(labA.getLocation(resp) == labA.prefix+"/login", "%s/ → login: %s", labA.prefix, labA.getLocation(resp))
		labA.closeResp(resp)
	}
	{
		resp, err = rawGet(ts.URL + labB.prefix + "/")
		assert(err == nil, "GET %s/ request", labB.prefix)
		assert(resp.StatusCode == 302, "%s/ redirects to login: %d", labB.prefix, resp.StatusCode)
		assert(labB.getLocation(resp) == labB.prefix+"/login", "%s/ → login: %s", labB.prefix, labB.getLocation(resp))
		labB.closeResp(resp)
	}
	{
		resp, err = rawGet(ts.URL + labA.prefix + "/device-loans")
		assert(err == nil, "GET %s/device-loans without auth", labA.prefix)
		assert(resp.StatusCode == 302, "device-loans → login: %d", resp.StatusCode)
		assert(labA.getLocation(resp) == labA.prefix+"/login", "device-loans → login Location: %s", labA.getLocation(resp))
		labA.closeResp(resp)
	}

	// ============================================
	// PHASE 1: Cross-Lab Session Isolation
	// ============================================
	t.Log("\n=== 1. CROSS-LAB SESSION ISOLATION ===")

	// Login to Lab A
	assert(labA.login("admin", "admin123"), "Lab A login sets session cookie")

	// While logged into Lab A, accessing Lab B should redirect to Lab B login
	resp, err = rawGet(ts.URL + labB.prefix + "/dashboard")
	assert(err == nil, "GET %s/dashboard while logged in A", labB.prefix)
	assert(resp.StatusCode == 302, "Lab A session cannot access Lab B dashboard: %d", resp.StatusCode)
	assert(labB.getLocation(resp) == labB.prefix+"/login", "redirect to Lab B login: %s", labB.getLocation(resp))
	labB.closeResp(resp)

	// Also verify Lab A can access its own dashboard
	resp, err = labA.get("/dashboard")
	assert(err == nil && resp.StatusCode == 200, "Lab A dashboard returns 200")
	labA.closeResp(resp)

	// Verify data isolation: Lab A has 43 PCs (seeded), Lab B has 0
	var pcCountA, pcCountB int
	dbA.QueryRow("SELECT COUNT(*) FROM pcs").Scan(&pcCountA)
	dbB.QueryRow("SELECT COUNT(*) FROM pcs").Scan(&pcCountB)
	assert(pcCountA == 43, "Lab A has %d PCs (expected 43)", pcCountA)
	assert(pcCountB == 0, "Lab B has %d PCs (expected 0 - no seeds)", pcCountB)

	// ============================================
	// PHASE 2: Lab A Full CRUD
	// ============================================
	t.Log("\n=== 2. LAB A CRUD ===")

	var csrfToken string

	// Refresh CSRF from dashboard
	resp, err = labA.get("/dashboard")
	assert(err == nil && resp.StatusCode == 200, "Lab A dashboard")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if t := labA.extractCSRFToken(string(body)); t != "" {
		csrfToken = t
		labA.csrf = t
	}
	assert(csrfToken != "", "CSRF token exists on Lab A dashboard")

	// 2a. PC Read (seeded PCs exist)
	t.Log("  --- 2a. PC READ (seeded) ---")
	resp, _ = labA.get("/pc")
	assert(resp.StatusCode == 200, "/pc list: %d", resp.StatusCode)
	labA.closeResp(resp)

	resp, _ = labA.get("/pc/pc-1")
	assert(resp.StatusCode == 200, "/pc/pc-1: %d", resp.StatusCode)
	labA.closeResp(resp)
	resp, _ = labA.get("/pc/pc-1/edit")
	assert(resp.StatusCode == 200, "/pc/pc-1 edit: %d", resp.StatusCode)
	labA.closeResp(resp)

	// 2b. PC Photo Upload
	t.Log("  --- 2b. PC PHOTO UPLOAD ---")
	photoData, _ := os.ReadFile(filepath.Join("tests", "resources", "logbook.jpeg"))
	var photoBuf bytes.Buffer
	mw := multipart.NewWriter(&photoBuf)
	fw, _ := mw.CreateFormFile("image", "logbook.jpeg")
	fw.Write(photoData)
	mw.WriteField("type", "serial")
	mw.Close()
	req, _ := http.NewRequest("POST", ts.URL+labA.prefix+"/api/upload-image", &photoBuf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-CSRF-Token", csrfToken)
	labA.addCookies(req)
	resp, err = client.Do(req)
	assert(err == nil, "upload image request")
	var uploadRes struct {
		Success bool   `json:"success"`
		FileRef string `json:"file_ref"`
	}
	json.NewDecoder(resp.Body).Decode(&uploadRes)
	labA.closeResp(resp)
	assert(uploadRes.Success && uploadRes.FileRef != "", "upload image: file_ref=%s", uploadRes.FileRef)

	resp, _ = labA.post("/pc/pc-1/edit",
		"status=normal&placement=dipakai&serial_number=SN001&operating_system=Win11&pc_type=PC&brand_model=Dell&accessories=KB&processor=i7&ram=16GB&storage=512GB&notes=&serial_file_ref="+uploadRes.FileRef)
	assert(resp.StatusCode == 302, "PC edit with photo: %d", resp.StatusCode)
	labA.closeResp(resp)
	var photoSerial string
	dbA.QueryRow("SELECT COALESCE(photo_serial,'') FROM pcs WHERE label='pc-1'").Scan(&photoSerial)
	assert(photoSerial != "", "photo_serial saved: %s", photoSerial)

	// 2c. PC Create + Delete
	t.Log("  --- 2c. PC CREATE + DELETE ---")
	pcCreateData := url.Values{
		"row": {"5"}, "column": {"8"},
		"status": {"normal"}, "placement": {"dipakai"},
		"is_mahasiswa": {"true"},
		"serial_number": {"SN-TEST-NEW"},
		"operating_system": {"Win11"}, "pc_type": {"PC"},
		"brand_model": {"Dell"}, "accessories": {"KB"},
		"processor": {"i7"}, "ram": {"16GB"}, "storage": {"512GB"},
	}.Encode()
	resp, _ = labA.post("/pc/create", pcCreateData)
	labA.closeResp(resp)
	var newPCID int
	var newPCLabel string
	dbA.QueryRow("SELECT id, label FROM pcs ORDER BY id DESC LIMIT 1").Scan(&newPCID, &newPCLabel)
	assert(newPCID > 0, "PC created: id=%d label=%s", newPCID, newPCLabel)

	resp, _ = labA.post("/pc/"+newPCLabel+"/delete", "")
	assert(resp.StatusCode == 302, "PC delete: %d", resp.StatusCode)
	labA.closeResp(resp)
	var pcDeleted int
	dbA.QueryRow("SELECT COUNT(*) FROM pcs WHERE id=?", newPCID).Scan(&pcDeleted)
	assert(pcDeleted == 0, "PC deleted")

	// Seed categories & device types for device CRUD
	t.Log("  --- 2d. SEED CATEGORY + DEVICE TYPE ---")
	dbA.Exec("INSERT OR IGNORE INTO categories (id, name, default_prefix) VALUES (1, 'Pentab', 'PENTAB')")
	dbA.Exec("INSERT OR IGNORE INTO device_types (id, category_id, name, brand, model, asset_code_prefix, usage_type, default_location) VALUES (1, 1, 'Pentab', 'Wacom', 'One', 'PENTAB', 'loanable', 'Lab')")

	// 2d. Device CRUD
	t.Log("  --- 2e. DEVICE CRUD ---")
	resp, _ = labA.get("/devices")
	assert(resp.StatusCode == 200, "/devices: %d", resp.StatusCode)
	labA.closeResp(resp)

	resp, _ = labA.post("/devices/create", "device_type_id=1&serial_number=SN-TEST001&condition=normal&location=Lab&purchase_date=&notes=Device+test")
	assert(resp.StatusCode == 302, "create device: %d", resp.StatusCode)
	labA.closeResp(resp)
	var devID int
	dbA.QueryRow("SELECT id FROM devices WHERE serial_number='SN-TEST001'").Scan(&devID)
	assert(devID > 0, "Device ID=%d", devID)

	var devAssetCode, devCatPrefix, devTypePrefix string
	dbA.QueryRow(`SELECT d.asset_code, COALESCE(c.default_prefix,''), COALESCE(dt.asset_code_prefix,'')
		FROM devices d
		JOIN device_types dt ON dt.id = d.device_type_id
		JOIN categories c ON c.id = dt.category_id
		WHERE d.id=?`, devID).Scan(&devAssetCode, &devCatPrefix, &devTypePrefix)
	devSlug := strings.ToLower(devAssetCode)
	devCatSlug := strings.ToLower(devCatPrefix)
	devTypeSlug := strings.ToLower(devTypePrefix)
	assert(devSlug != "", "Device slug=%s", devSlug)

	nestedURL := "/devices/" + devCatSlug + "/" + devTypeSlug + "/" + devSlug
	resp, _ = labA.get(nestedURL)
	assert(resp.StatusCode == 200, "%s: %d", nestedURL, resp.StatusCode)
	labA.closeResp(resp)

	resp, _ = labA.get("/devices/" + devSlug + "/edit")
	assert(resp.StatusCode == 200, "/devices/%s/edit: %d", devSlug, resp.StatusCode)
	labA.closeResp(resp)

	resp, _ = labA.post("/devices/"+devSlug+"/edit",
		"device_type_id=1&asset_code=PENTAB-001&serial_number=SN-TEST002&condition=rusak&location=Lab2&purchase_date=&notes=Updated")
	assert(resp.StatusCode == 302, "edit device: %d", resp.StatusCode)
	labA.closeResp(resp)
	var devSerial string
	dbA.QueryRow("SELECT serial_number FROM devices WHERE id=?", devID).Scan(&devSerial)
	assert(devSerial == "SN-TEST002", "Device serial updated: %s", devSerial)

	t.Log("  --- BATCH CREATE ---")
	var dtID int
	dbA.QueryRow("SELECT id FROM device_types ORDER BY id LIMIT 1").Scan(&dtID)
	assert(dtID > 0, "device_type exists for batch")
	batchBody := fmt.Sprintf(`{"device_type_id":%d,"devices":[{"serial_number":"SN-BATCH1","condition":"normal","location":"Lab"},{"serial_number":"SN-BATCH2","condition":"rusak","location":"Lab"}]}`, dtID)
	resp, _ = labA.postJSON("/devices/batch-create", batchBody)
	assert(resp.StatusCode == 200, "batch create: %d", resp.StatusCode)
	var batchRes struct {
		Success bool     `json:"success"`
		Codes   []string `json:"codes"`
	}
	json.NewDecoder(resp.Body).Decode(&batchRes)
	labA.closeResp(resp)
	assert(batchRes.Success, "batch create success")
	assert(len(batchRes.Codes) == 2, "batch codes: %d", len(batchRes.Codes))
	var batchCount int
	dbA.QueryRow("SELECT COUNT(*) FROM devices WHERE serial_number LIKE 'SN-BATCH%'").Scan(&batchCount)
	assert(batchCount == 2, "batch devices: %d", batchCount)

	// 2e. Software CRUD
	t.Log("  --- 2f. SOFTWARE CRUD ---")
	resp, _ = labA.get("/software")
	assert(resp.StatusCode == 200, "/software: %d", resp.StatusCode)
	labA.closeResp(resp)

	resp, _ = labA.post("/software/create", "name=TestSW&category=other&description=Test")
	assert(resp.StatusCode == 302, "create software: %d", resp.StatusCode)
	labA.closeResp(resp)
	var swID int
	dbA.QueryRow("SELECT id FROM software_catalog WHERE name='Testsw'").Scan(&swID)
	assert(swID > 0, "Software ID=%d", swID)

	var swSlug string
	dbA.QueryRow("SELECT slug FROM software_catalog WHERE id=?", swID).Scan(&swSlug)
	assert(swSlug != "", "Software slug=%s", swSlug)

	resp, _ = labA.get("/software/" + swSlug + "/edit")
	assert(resp.StatusCode == 200, "/software/%s/edit: %d", swSlug, resp.StatusCode)
	labA.closeResp(resp)

	resp, _ = labA.post("/software/"+swSlug+"/edit", "name=TestSW2&category=required&description=Test2+updated")
	assert(resp.StatusCode == 302, "edit software: %d", resp.StatusCode)
	labA.closeResp(resp)
	var swName string
	dbA.QueryRow("SELECT name FROM software_catalog WHERE id=?", swID).Scan(&swName)
	assert(swName == "Testsw2", "Software name updated: %s", swName)

	dbA.QueryRow("SELECT slug FROM software_catalog WHERE id=?", swID).Scan(&swSlug)
	resp, _ = labA.post("/software/"+swSlug+"/delete", "")
	assert(resp.StatusCode == 302, "delete software: %d", resp.StatusCode)
	labA.closeResp(resp)
	var swDelCount int
	dbA.QueryRow("SELECT COUNT(*) FROM software_catalog WHERE id=?", swID).Scan(&swDelCount)
	assert(swDelCount == 0, "Software deleted")

	// 2f. Schedule CRUD
	t.Log("  --- 2g. SCHEDULE CRUD ---")
	resp, _ = labA.get("/schedules")
	assert(resp.StatusCode == 200, "/schedules: %d", resp.StatusCode)
	labA.closeResp(resp)

	resp, _ = labA.post("/schedules/create", "course_name=Algo&lecturer=Dr.T&day=Senin&class=IF-1&time_start=08:00&time_end=09:40")
	assert(resp.StatusCode == 302, "create schedule: %d", resp.StatusCode)
	labA.closeResp(resp)
	var scID int
	dbA.QueryRow("SELECT id FROM course_schedules WHERE course_name='Algo'").Scan(&scID)
	assert(scID > 0, "Schedule ID=%d", scID)

	resp, _ = labA.get("/schedules/" + fmt.Sprint(scID) + "/edit")
	assert(resp.StatusCode == 200, "/schedules/%d/edit: %d", scID, resp.StatusCode)
	labA.closeResp(resp)

	resp, _ = labA.post("/schedules/"+fmt.Sprint(scID)+"/edit",
		"course_name=Algo2&lecturer=Dr.T&day=Senin&class=IF-1&time_start=08:00&time_end=09:40")
	assert(resp.StatusCode == 302, "edit schedule: %d", resp.StatusCode)
	labA.closeResp(resp)
	var scName string
	dbA.QueryRow("SELECT course_name FROM course_schedules WHERE id=?", scID).Scan(&scName)
	assert(scName == "Algo2", "Schedule name updated: %s", scName)

	resp, _ = labA.post("/schedules/"+fmt.Sprint(scID)+"/delete", "")
	assert(resp.StatusCode == 302, "delete schedule: %d", resp.StatusCode)
	labA.closeResp(resp)
	var scDelCount int
	dbA.QueryRow("SELECT COUNT(*) FROM course_schedules WHERE id=?", scID).Scan(&scDelCount)
	assert(scDelCount == 0, "Schedule deleted")

	// 2g. Logbook CRUD
	t.Log("  --- 2h. LOGBOOK CRUD ---")
	resp, _ = labA.get("/logbook")
	assert(resp.StatusCode == 200, "/logbook: %d", resp.StatusCode)
	labA.closeResp(resp)

	resp, _ = labA.post("/logbook/create", "date=2026-05-16&student_name=Mhs+Test&nim=24091234567&time_in=08:00&time_out=09:40&purpose=Prak")
	assert(resp.StatusCode == 302, "create logbook: %d", resp.StatusCode)
	labA.closeResp(resp)
	var lbCount int
	dbA.QueryRow("SELECT COUNT(*) FROM logbook_entries").Scan(&lbCount)
	assert(lbCount > 0, "Logbook entries: %d", lbCount)

	t.Log("  --- LOGBOOK UPLOAD ---")
	photoBuf.Reset()
	mw = multipart.NewWriter(&photoBuf)
	fw, _ = mw.CreateFormFile("logbook_image", "logbook.jpeg")
	fw.Write(photoData)
	mw.Close()
	req, _ = http.NewRequest("POST", ts.URL+labA.prefix+"/logbook/upload", &photoBuf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-CSRF-Token", csrfToken)
	labA.addCookies(req)
	resp, err = client.Do(req)
	assert(err == nil, "logbook upload")
	bodyOCR, _ := io.ReadAll(resp.Body)
	labA.closeResp(resp)

	if cfg.GeminiAPIKey != "" || cfg.OpenRouterAPIKey != "" {
		assert(resp.StatusCode == 200, "logbook upload (with API key): %d", resp.StatusCode)
		assert(strings.Contains(string(bodyOCR), "Preview Hasil OCR"), "OCR preview page rendered")
	} else {
		assert(resp.StatusCode == 500, "logbook upload (no API key): %d", resp.StatusCode)
		assert(strings.Contains(string(bodyOCR), "API key tidak dikonfigurasi"), "proper error message when no API keys")
	}

	// 2h. User management
	t.Log("  --- 2i. USER MANAGEMENT ---")
	resp, _ = labA.get("/admin/users")
	assert(resp.StatusCode == 200, "/admin/users: %d", resp.StatusCode)
	labA.closeResp(resp)
	resp, _ = labA.get("/profile")
	assert(resp.StatusCode == 200, "/profile: %d", resp.StatusCode)
	labA.closeResp(resp)
	resp, _ = labA.post("/profile", "username=admin&full_name=Admin+U")
	assert(resp.StatusCode == 302, "profile update: %d", resp.StatusCode)
	labA.closeResp(resp)

	// 2i. Activity Log
	t.Log("  --- 2j. ACTIVITY LOG ---")
	var logCountA int
	dbA.QueryRow("SELECT COUNT(*) FROM activity_logs").Scan(&logCountA)
	t.Logf("  Activity logs: %d", logCountA)
	resp, _ = labA.get("/admin/activity-logs")
	assert(resp.StatusCode == 200, "/admin/activity-logs: %d", resp.StatusCode)
	labA.closeResp(resp)

	// 2j. Export Download
	t.Log("  --- 2k. EXPORT DOWNLOAD ---")
	checkExport := func(l *testLab, path, prefix string) {
		resp, _ := l.get(path)
		assert(resp.StatusCode == 200, "%s: %d", path, resp.StatusCode)
		ct := resp.Header.Get("Content-Type")
		assert(ct == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "%s CT: %s", path, ct)
		cd := resp.Header.Get("Content-Disposition")
		assert(strings.HasPrefix(cd, "attachment; filename="+prefix), "%s CD: %s", path, cd)
		body, _ := io.ReadAll(resp.Body)
		l.closeResp(resp)
		assert(len(body) > 0, "%s empty", path)
	}
	checkExport(labA, "/pc/export", "pc_export")
	checkExport(labA, "/software/export", "software_catalog_export")
	checkExport(labA, "/logbook/export", "logbook_export")
	checkExport(labA, "/logbook/export-preview", "logbook_export_preview")
	checkExport(labA, "/admin/activity-logs/export", "activity_log_export")

	// 2k. Device Loan CRUD
	t.Log("  --- 2l. DEVICE LOAN CRUD ---")
	var loanDevID int
	dbA.QueryRow("SELECT id FROM devices ORDER BY id LIMIT 1").Scan(&loanDevID)
	assert(loanDevID > 0, "device exists for loan")
	resp, _ = labA.post("/device-loans/create", fmt.Sprintf("device_id=%d&borrower_name=Mahasiswa+Test&borrower_type=mahasiswa&loan_date=2026-05-16&return_date=2026-05-20&purpose=Praktikum", loanDevID))
	assert(resp.StatusCode == 302, "create loan: %d", resp.StatusCode)
	labA.closeResp(resp)
	var loanCount int
	dbA.QueryRow("SELECT COUNT(*) FROM device_loans").Scan(&loanCount)
	assert(loanCount > 0, "loans: %d", loanCount)
	resp, _ = labA.get("/devices?tab=loans")
	assert(resp.StatusCode == 200, "/devices loans: %d", resp.StatusCode)
	labA.closeResp(resp)

	var loanID int
	dbA.QueryRow("SELECT id FROM device_loans ORDER BY id DESC LIMIT 1").Scan(&loanID)
	assert(loanID > 0, "Loan ID=%d", loanID)
	resp, _ = labA.get("/device-loans/" + fmt.Sprint(loanID) + "/edit")
	assert(resp.StatusCode == 200, "/device-loans/%d/edit: %d", loanID, resp.StatusCode)
	labA.closeResp(resp)

	resp, _ = labA.post("/device-loans/"+fmt.Sprint(loanID)+"/edit",
		"borrower_name=Mahasiswa+Test&borrower_type=mahasiswa&loan_date=2026-05-16&purpose=Praktikum&status=returned&actual_return_date=2026-05-17&notes=")
	assert(resp.StatusCode == 302, "edit loan: %d", resp.StatusCode)
	labA.closeResp(resp)
	var loanReturned string
	dbA.QueryRow("SELECT COALESCE(actual_return_date,'') FROM device_loans WHERE id=?", loanID).Scan(&loanReturned)
	assert(loanReturned != "", "Loan returned: %s", loanReturned)

	resp, _ = labA.post("/device-loans/"+fmt.Sprint(loanID)+"/delete", "")
	assert(resp.StatusCode == 302, "delete loan: %d", resp.StatusCode)
	labA.closeResp(resp)
	dbA.QueryRow("SELECT COUNT(*) FROM device_loans WHERE id=?", loanID).Scan(&loanCount)
	assert(loanCount == 0, "Loan deleted")

	// 2l. Device Usage CRUD
	t.Log("  --- 2m. DEVICE USAGE CRUD ---")
	resp, _ = labA.post("/device-usages/create", fmt.Sprintf("device_id=%d&user_name=Dosen+Test&user_type=dosen&usage_date=2026-05-16&is_available=yes&purpose=Demo", loanDevID))
	assert(resp.StatusCode == 302, "create usage: %d", resp.StatusCode)
	labA.closeResp(resp)
	var usageCount int
	dbA.QueryRow("SELECT COUNT(*) FROM device_usages").Scan(&usageCount)
	assert(usageCount > 0, "usages: %d", usageCount)
	resp, _ = labA.get("/devices?tab=usages")
	assert(resp.StatusCode == 200, "/devices usages: %d", resp.StatusCode)
	labA.closeResp(resp)

	var usageID int
	dbA.QueryRow("SELECT id FROM device_usages ORDER BY id DESC LIMIT 1").Scan(&usageID)
	assert(usageID > 0, "Usage ID=%d", usageID)
	resp, _ = labA.get("/device-usages/" + fmt.Sprint(usageID) + "/edit")
	assert(resp.StatusCode == 200, "/device-usages/%d/edit: %d", usageID, resp.StatusCode)
	labA.closeResp(resp)

	resp, _ = labA.post("/device-usages/"+fmt.Sprint(usageID)+"/edit",
		"user_name=Dosen+Updated&user_type=dosen&usage_date=2026-05-16&is_available=yes&purpose=Demo")
	assert(resp.StatusCode == 302, "edit usage: %d", resp.StatusCode)
	labA.closeResp(resp)
	var usageUser string
	dbA.QueryRow("SELECT user_name FROM device_usages WHERE id=?", usageID).Scan(&usageUser)
	assert(usageUser == "Dosen Updated", "Usage user updated: %s", usageUser)

	resp, _ = labA.post("/device-usages/"+fmt.Sprint(usageID)+"/delete", "")
	assert(resp.StatusCode == 302, "delete usage: %d", resp.StatusCode)
	labA.closeResp(resp)
	dbA.QueryRow("SELECT COUNT(*) FROM device_usages WHERE id=?", usageID).Scan(&usageCount)
	assert(usageCount == 0, "Usage deleted")

	// 2m. Device Installation CRUD
	t.Log("  --- 2n. DEVICE INSTALLATION CRUD ---")
	var installDevID int
	dbA.QueryRow("SELECT id FROM devices ORDER BY id LIMIT 1").Scan(&installDevID)
	assert(installDevID > 0, "device exists for installation")
	resp, _ = labA.post("/installations/create", fmt.Sprintf("device_id=%d&location_installed=Lab+Utama&installation_start_date=2026-05-01&notes=Installed+test", installDevID))
	assert(resp.StatusCode == 302, "create installation: %d", resp.StatusCode)
	labA.closeResp(resp)
	var installCount int
	dbA.QueryRow("SELECT COUNT(*) FROM device_installations").Scan(&installCount)
	assert(installCount > 0, "installations: %d", installCount)
	var installID int
	dbA.QueryRow("SELECT id FROM device_installations ORDER BY id DESC LIMIT 1").Scan(&installID)
	assert(installID > 0, "Installation ID=%d", installID)

	resp, _ = labA.get("/installations/" + fmt.Sprint(installID))
	assert(resp.StatusCode == 200, "/installations/%d: %d", installID, resp.StatusCode)
	labA.closeResp(resp)
	resp, _ = labA.get("/devices?tab=installations")
	assert(resp.StatusCode == 200, "/devices?tab=installations: %d", resp.StatusCode)
	labA.closeResp(resp)
	resp, _ = labA.get("/installations/" + fmt.Sprint(installID) + "/edit")
	assert(resp.StatusCode == 200, "/installations/%d/edit: %d", installID, resp.StatusCode)
	labA.closeResp(resp)

	resp, _ = labA.post("/installations/"+fmt.Sprint(installID)+"/edit",
		"location_installed=Lab+Cadangan&installation_start_date=2026-05-01&installation_finish_date=2026-05-10&notes=Updated")
	assert(resp.StatusCode == 302, "edit installation: %d", resp.StatusCode)
	labA.closeResp(resp)
	var installLoc string
	dbA.QueryRow("SELECT location_installed FROM device_installations WHERE id=?", installID).Scan(&installLoc)
	assert(installLoc == "Lab Cadangan", "Installation location updated: %s", installLoc)

	resp, _ = labA.post("/installations/"+fmt.Sprint(installID)+"/delete", "")
	assert(resp.StatusCode == 302, "delete installation: %d", resp.StatusCode)
	labA.closeResp(resp)
	dbA.QueryRow("SELECT COUNT(*) FROM device_installations WHERE id=?", installID).Scan(&installCount)
	assert(installCount == 0, "Installation deleted")

	// 2n. Logbook Save
	t.Log("  --- 2o. LOGBOOK SAVE ---")
	resp, _ = labA.post("/logbook/save", "source_file=test&date[]=2026-05-17&student_name[]=Mahasiswa+Save&nim[]=24091111111&time_in[]=10:00&time_out[]=11:40&purpose[]=Praktikum")
	assert(resp.StatusCode == 200, "logbook save: %d", resp.StatusCode)
	var lsRes struct {
		Success bool
		Saved   int
	}
	json.NewDecoder(resp.Body).Decode(&lsRes)
	labA.closeResp(resp)
	assert(lsRes.Success && lsRes.Saved == 1, "save: success=%v saved=%d", lsRes.Success, lsRes.Saved)

	// 2o. Change Password
	t.Log("  --- 2p. CHANGE PASSWORD ---")
	resp, _ = labA.post("/profile/password", "old_password=admin123&new_password=admin123&confirm_password=admin123")
	assert(resp.StatusCode == 302, "change password success: %d", resp.StatusCode)
	labA.closeResp(resp)
	resp, _ = labA.post("/profile/password", "old_password=wrongpass&new_password=admin123&confirm_password=admin123")
	assert(resp.StatusCode == 302, "change password wrong pass: %d", resp.StatusCode)
	labA.closeResp(resp)
	resp, _ = labA.post("/profile/password", "old_password=admin123&new_password=newpass123&confirm_password=mismatch")
	assert(resp.StatusCode == 302, "change password mismatch: %d", resp.StatusCode)
	labA.closeResp(resp)

	// Cleanup device
	t.Log("  --- 2q. DEVICE CLEANUP ---")
	dbA.QueryRow("SELECT asset_code FROM devices WHERE id=?", devID).Scan(&devAssetCode)
	devSlug = strings.ToLower(devAssetCode)
	resp, _ = labA.post("/devices/"+devSlug+"/delete", "")
	assert(resp.StatusCode == 302, "delete device: %d", resp.StatusCode)
	labA.closeResp(resp)
	var devDelCount int
	dbA.QueryRow("SELECT COUNT(*) FROM devices WHERE id=?", devID).Scan(&devDelCount)
	assert(devDelCount == 0, "Device deleted")

	// ============================================
	// PHASE 3: Cross-Lab Data Isolation (reverse)
	// ============================================
	t.Log("\n=== 3. CROSS-LAB DATA ISOLATION (reverse) ===")

	// Login to Lab B
	assert(labB.login("admin", "admin123"), "Lab B login sets session cookie")

	// While logged into Lab B, accessing Lab A should redirect to Lab A login
	resp, err = rawGet(ts.URL + labA.prefix + "/dashboard")
	assert(err == nil, "GET %s/dashboard while logged in B", labA.prefix)
	assert(resp.StatusCode == 302, "Lab B session cannot access Lab A dashboard: %d", resp.StatusCode)
	assert(labA.getLocation(resp) == labA.prefix+"/login", "redirect to Lab A login: %s", labA.getLocation(resp))
	labA.closeResp(resp)

	// Verify Lab B can access its own dashboard
	resp, err = labB.get("/dashboard")
	assert(err == nil && resp.StatusCode == 200, "Lab B dashboard returns 200")
	labB.closeResp(resp)

	// Verify Lab B has no PC data (no seeds)
	var pcCountB2 int
	dbB.QueryRow("SELECT COUNT(*) FROM pcs").Scan(&pcCountB2)
	assert(pcCountB2 == 0, "Lab B still has %d PCs after login (expected 0)", pcCountB2)

	// Verify Lab A's data is not visible in Lab B's DB
	var swCountA, swCountB int
	dbA.QueryRow("SELECT COUNT(*) FROM software_catalog").Scan(&swCountA)
	dbB.QueryRow("SELECT COUNT(*) FROM software_catalog").Scan(&swCountB)
	assert(swCountA > 0, "Lab A has software catalog (%d entries)", swCountA)
	assert(swCountB == 0, "Lab B has no software catalog (%d entries - no seeds)", swCountB)

	var schedCountA, schedCountB int
	dbA.QueryRow("SELECT COUNT(*) FROM course_schedules").Scan(&schedCountA)
	dbB.QueryRow("SELECT COUNT(*) FROM course_schedules").Scan(&schedCountB)
	assert(schedCountA > 0, "Lab A has schedules (%d entries)", schedCountA)
	assert(schedCountB == 0, "Lab B has no schedules (%d entries - no seeds)", schedCountB)

	// ============================================
	// PHASE 4: Lab B CRUD (empty lab)
	// ============================================
	t.Log("\n=== 4. LAB B CRUD (empty lab) ===")

	// Refresh CSRF for Lab B
	resp, err = labB.get("/dashboard")
	assert(err == nil && resp.StatusCode == 200, "Lab B dashboard")
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if t := labB.extractCSRFToken(string(body)); t != "" {
		labB.csrf = t
	}
	assert(labB.csrf != "", "CSRF token exists on Lab B dashboard")

	// 4a. PC list (empty, no seeds)
	t.Log("  --- 4a. PC LIST (empty) ---")
	resp, _ = labB.get("/pc")
	assert(resp.StatusCode == 200, "Lab B /pc: %d", resp.StatusCode)
	labB.closeResp(resp)

	// 4b. PC Create on Lab B
	t.Log("  --- 4b. PC CREATE on Lab B ---")
	pcBData := url.Values{
		"row": {"1"}, "column": {"1"},
		"status": {"normal"}, "placement": {"dipakai"},
		"is_mahasiswa": {"true"},
		"serial_number": {"SN-LAB-B-001"},
		"operating_system": {"Win11"}, "pc_type": {"PC"},
		"brand_model": {"Dell"}, "accessories": {"KB"},
		"processor": {"i5"}, "ram": {"8GB"}, "storage": {"256GB"},
	}.Encode()
	resp, _ = labB.post("/pc/create", pcBData)
	assert(resp.StatusCode == 302, "Lab B PC create: %d", resp.StatusCode)
	labB.closeResp(resp)
	var pcBID int
	var pcBLabel string
	dbB.QueryRow("SELECT id, label FROM pcs ORDER BY id DESC LIMIT 1").Scan(&pcBID, &pcBLabel)
	assert(pcBID > 0, "Lab B PC created: id=%d label=%s", pcBID, pcBLabel)

	// Verify PC created on Lab B does NOT appear on Lab A (use unique serial_number)
	var pcA_afterB int
	dbA.QueryRow("SELECT COUNT(*) FROM pcs WHERE serial_number='SN-LAB-B-001'").Scan(&pcA_afterB)
	assert(pcA_afterB == 0, "Lab B's PC not visible in Lab A")

	// 4c. Seed category+device_type for Lab B, then Device CRUD
	t.Log("  --- 4c. DEVICE CRUD on Lab B ---")
	dbB.Exec("INSERT OR IGNORE INTO categories (id, name, default_prefix) VALUES (1, 'Monitor', 'MONITOR')")
	dbB.Exec("INSERT OR IGNORE INTO device_types (id, category_id, name, brand, model, asset_code_prefix, usage_type, default_location) VALUES (1, 1, 'LCD Monitor', 'Dell', '22in', 'MONITOR', 'loanable', 'Lab B')")

	resp, _ = labB.get("/devices")
	assert(resp.StatusCode == 200, "Lab B /devices: %d", resp.StatusCode)
	labB.closeResp(resp)

	resp, _ = labB.post("/devices/create", "device_type_id=1&serial_number=SN-LAB-B-001&condition=normal&location=Lab+B&purchase_date=&notes=")
	assert(resp.StatusCode == 302, "Lab B create device: %d", resp.StatusCode)
	labB.closeResp(resp)
	var devBID int
	dbB.QueryRow("SELECT id FROM devices WHERE serial_number='SN-LAB-B-001'").Scan(&devBID)
	assert(devBID > 0, "Lab B Device ID=%d", devBID)

	// Verify device created on Lab B does NOT appear on Lab A
	var devA_check int
	dbA.QueryRow("SELECT COUNT(*) FROM devices WHERE id=?", devBID).Scan(&devA_check)
	assert(devA_check == 0, "Lab B's device not visible in Lab A")

	// 4d. Software CRUD on Lab B
	t.Log("  --- 4d. SOFTWARE CRUD on Lab B ---")
	resp, _ = labB.get("/software")
	assert(resp.StatusCode == 200, "Lab B /software: %d", resp.StatusCode)
	labB.closeResp(resp)

	resp, _ = labB.post("/software/create", "name=Python&category=required&description=Language")
	assert(resp.StatusCode == 302, "Lab B create software: %d", resp.StatusCode)
	labB.closeResp(resp)
	var swBID int
	dbB.QueryRow("SELECT id FROM software_catalog WHERE name='Python'").Scan(&swBID)
	assert(swBID > 0, "Lab B Software ID=%d", swBID)

	// 4e. Schedule CRUD on Lab B
	t.Log("  --- 4e. SCHEDULE CRUD on Lab B ---")
	resp, _ = labB.get("/schedules")
	assert(resp.StatusCode == 200, "Lab B /schedules: %d", resp.StatusCode)
	labB.closeResp(resp)

	resp, _ = labB.post("/schedules/create", "course_name=Jaringan&lecturer=Pak+Budi&day=Senin&class=TI-1&time_start=07:00&time_end=08:40")
	assert(resp.StatusCode == 302, "Lab B create schedule: %d", resp.StatusCode)
	labB.closeResp(resp)
	var scBID int
	dbB.QueryRow("SELECT id FROM course_schedules WHERE course_name='Jaringan'").Scan(&scBID)
	assert(scBID > 0, "Lab B Schedule ID=%d", scBID)

	// Cleanup Lab B: delete PC + device
	t.Log("  --- 4f. CLEANUP Lab B ---")
	resp, _ = labB.post("/pc/"+pcBLabel+"/delete", "")
	assert(resp.StatusCode == 302, "Lab B PC delete: %d", resp.StatusCode)
	labB.closeResp(resp)
	dbB.QueryRow("SELECT COUNT(*) FROM pcs WHERE id=?", pcBID).Scan(&pcBID)
	assert(pcBID == 0, "Lab B PC deleted")

	resp, _ = labB.post("/devices/monitor-001/delete", "")
	assert(resp.StatusCode == 302, "Lab B device delete: %d", resp.StatusCode)
	labB.closeResp(resp)
	dbB.QueryRow("SELECT COUNT(*) FROM devices WHERE id=?", devBID).Scan(&devBID)
	assert(devBID == 0, "Lab B device deleted")

	// ============================================
	// PHASE 5: Summary
	// ============================================
	t.Log("\n=== 5. SUMMARY ===")

	t.Log("--- Lab A tables ---")
	rows, _ := dbA.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var tbl string
			rows.Scan(&tbl)
			var c int
			dbA.QueryRow("SELECT COUNT(*) FROM " + tbl).Scan(&c)
			if c > 0 {
				t.Logf("  %s: %d rows", tbl, c)
			}
		}
	}
	dbA.Flush()

	t.Log("--- Lab B tables ---")
	rowsB, _ := dbB.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if rowsB != nil {
		defer rowsB.Close()
		for rowsB.Next() {
			var tbl string
			rowsB.Scan(&tbl)
			var c int
			dbB.QueryRow("SELECT COUNT(*) FROM " + tbl).Scan(&c)
			if c > 0 {
				t.Logf("  %s: %d rows", tbl, c)
			}
		}
	}
	dbB.Flush()

	flushLogs()
	dbA.QueryRow("SELECT COUNT(*) FROM activity_logs").Scan(&logCountA)
	t.Logf("  Lab A activity logs: %d", logCountA)
	var logCountB int
	dbB.QueryRow("SELECT COUNT(*) FROM activity_logs").Scan(&logCountB)
	t.Logf("  Lab B activity logs: %d", logCountB)
	assert(logCountA > 0, "Lab A activity logs exist: %d", logCountA)
	assert(logCountB > 0, "Lab B activity logs exist: %d", logCountB)
	t.Logf("  All tests passed!")
}
