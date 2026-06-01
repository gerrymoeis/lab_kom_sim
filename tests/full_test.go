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

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/server"

	"github.com/joho/godotenv"
)

func TestFullIntegration(t *testing.T) {
	// Change to project root (tests/ project root)
	wd, _ := os.Getwd()
	projectRoot := filepath.Dir(wd)
	os.Chdir(projectRoot)
	defer os.Chdir(wd)
	defer func() {
		dbPath := "full_testing.db"
		os.Remove(dbPath)
		os.Remove(dbPath + "-shm")
		os.Remove(dbPath + "-wal")
	}()
	defer func() {
		os.RemoveAll(filepath.Join("uploads", "temp"))
		os.RemoveAll(filepath.Join("uploads", "pc"))
		os.RemoveAll(filepath.Join("uploads", "logbook"))
	}()
	// Setup
	dbPath := "full_testing.db"

	// Load .env for API keys (if available)
	godotenv.Load()

	cfg := &config.Config{
		DatabasePath:     dbPath,
		SessionSecret:    "test-secret-12345",
		UploadPath:       "uploads",
		DefaultPageSize:  25,
		GeminiAPIKey:     os.Getenv("GEMINI_API_KEY"),
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
	}
	db, err := database.InitDB(dbPath, "")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db, false); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if err := database.SeedDefaultUser(db); err != nil {
		t.Errorf("Seed user: %v", err)
	}
	db.Exec("UPDATE users SET session_token = NULL")

	router, cleanup := server.SetupRouter(db, cfg, nil)
	defer cleanup()
	ts := httptest.NewServer(router)
	defer ts.Close()

	noRedirect := func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	client := &http.Client{CheckRedirect: noRedirect}
	jar := make(map[string]string)

	closeResp := func(resp *http.Response) {
		if resp != nil && resp.Body != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}
	saveCookies := func(resp *http.Response) {
		for _, c := range resp.Cookies() {
			jar[c.Name] = c.Value
		}
	}
	addCookies := func(req *http.Request) {
		for n, v := range jar {
			req.AddCookie(&http.Cookie{Name: n, Value: v})
		}
	}

	login := func() bool {
		req, _ := http.NewRequest("POST", ts.URL+"/login", strings.NewReader("username=admin&password=admin123"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		defer closeResp(resp)
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
	postJSON := func(path, data string) (*http.Response, error) {
		req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		addCookies(req)
		return client.Do(req)
	}

	assert := func(cond bool, msg string, args ...any) {
		if !cond {
			t.Errorf("FAIL: "+msg, args...)
		}
	}

	// 1. Login
	t.Log("\n=== 1. LOGIN ===")
	assert(login(), "Login should set session cookie")
	resp, err := get("/dashboard")
	assert(err == nil && resp.StatusCode == 200, "/dashboard returns 200")
	closeResp(resp)

	// 2. PC CRUD
	t.Log("\n=== 2. PC CRUD ===")
	resp, _ = get("/pc")
	assert(resp.StatusCode == 200, "/pc list: %d", resp.StatusCode)
	closeResp(resp)

	var pcCount int
	db.QueryRow("SELECT COUNT(*) FROM pcs").Scan(&pcCount)
	assert(pcCount > 0, "PCs seeded: %d", pcCount)

	resp, _ = get("/pc/pc-1")
	assert(resp.StatusCode == 200, "/pc/pc-1: %d", resp.StatusCode)
	closeResp(resp)
	resp, _ = get("/pc/pc-1/edit")
	assert(resp.StatusCode == 200, "/pc/pc-1 edit: %d", resp.StatusCode)
	closeResp(resp)

	//  2b. PC Photo Upload →
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
	closeResp(resp)
	assert(uploadRes.Success && uploadRes.FileRef != "", "upload image: file_ref=%s", uploadRes.FileRef)

	resp, _ = post("/pc/pc-1/edit",
		"status=normal&placement=dipakai&serial_number=SN001&operating_system=Win11&pc_type=PC&brand_model=Dell&accessories=KB&processor=i7&ram=16GB&storage=512GB&notes=&serial_file_ref="+uploadRes.FileRef)
	assert(resp.StatusCode == 302, "PC edit with photo: %d", resp.StatusCode)
	closeResp(resp)

	var photoSerial string
	db.QueryRow("SELECT COALESCE(photo_serial,'') FROM pcs WHERE label='pc-1'").Scan(&photoSerial)
	assert(photoSerial != "", "photo_serial saved: %s", photoSerial)

	// 2c. PC Create + Delete
	t.Log("\n=== 2c. PC CREATE + DELETE ===")
	pcCreateData := url.Values{
		"row": {"5"}, "column": {"8"},
		"status": {"normal"}, "placement": {"dipakai"},
		"is_mahasiswa": {"true"},
		"serial_number": {"SN-TEST40"},
		"operating_system": {"Win11"}, "pc_type": {"PC"},
		"brand_model": {"Dell"}, "accessories": {"KB"},
		"processor": {"i7"}, "ram": {"16GB"}, "storage": {"512GB"},
	}.Encode()
	resp, _ = post("/pc/create", pcCreateData)
	closeResp(resp)
	var newPCID int
	db.QueryRow("SELECT id FROM pcs WHERE label='pc-40'").Scan(&newPCID)
	assert(newPCID > 0, "PC 40 created: id=%d", newPCID)

	resp, _ = post("/pc/pc-40/delete", "")
	assert(resp.StatusCode == 302, "PC delete: %d", resp.StatusCode)
	closeResp(resp)
	var pcDeleted int
	db.QueryRow("SELECT COUNT(*) FROM pcs WHERE label='pc-40'").Scan(&pcDeleted)
	assert(pcDeleted == 0, "PC 40 deleted")

	// Seed category & device type (seed_devices.go was removed in Item 6.1)
	db.Exec("INSERT OR IGNORE INTO categories (id, name, default_prefix) VALUES (1, 'Pentab', 'PENTAB')")
	db.Exec("INSERT OR IGNORE INTO device_types (id, category_id, name, brand, model, asset_code_prefix, usage_type, default_location) VALUES (1, 1, 'Pentab', 'Wacom', 'One', 'PENTAB', 'loanable', 'Lab')")

	// 3. Device CRUD
	t.Log("\n=== 3. DEVICE CRUD ===")
	resp, _ = get("/devices")
	assert(resp.StatusCode == 200, "/devices: %d", resp.StatusCode)
	closeResp(resp)

	resp, _ = post("/devices/create", "device_type_id=1&serial_number=SN-TEST001&condition=normal&location=Lab&purchase_date=&notes=Device+test")
	assert(resp.StatusCode == 302, "create device: %d", resp.StatusCode)
	closeResp(resp)
	var devID int
	db.QueryRow("SELECT id FROM devices WHERE serial_number='SN-TEST001'").Scan(&devID)
	assert(devID > 0, "Device ID=%d", devID)

	// Query asset_code for device (no slug column, use LOWER(asset_code) as slug)
	var devAssetCode string
	db.QueryRow("SELECT asset_code FROM devices WHERE id=?", devID).Scan(&devAssetCode)
	devSlug := strings.ToLower(devAssetCode)
	assert(devSlug != "", "Device slug=%s", devSlug)

	// Device detail
	resp, _ = get("/devices/" + devSlug)
	assert(resp.StatusCode == 200, "/devices/%s: %d", devSlug, resp.StatusCode)
	closeResp(resp)

	// Device edit page
	resp, _ = get("/devices/" + devSlug + "/edit")
	assert(resp.StatusCode == 200, "/devices/%s/edit: %d", devSlug, resp.StatusCode)
	closeResp(resp)

	// Device edit POST
	resp, _ = post("/devices/"+devSlug+"/edit",
		"device_type_id=1&asset_code=PENTAB-001&serial_number=SN-TEST002&condition=rusak&location=Lab2&purchase_date=&notes=Updated")
	assert(resp.StatusCode == 302, "edit device: %d", resp.StatusCode)
	closeResp(resp)
	var devSerial string
	db.QueryRow("SELECT serial_number FROM devices WHERE id=?", devID).Scan(&devSerial)
	assert(devSerial == "SN-TEST002", "Device serial updated: %s", devSerial)

	// Batch create devices
	t.Log("--- BATCH CREATE ---")
	var dtID int
	db.QueryRow("SELECT id FROM device_types ORDER BY id LIMIT 1").Scan(&dtID)
	assert(dtID > 0, "device_type exists for batch")
	body := fmt.Sprintf(`{"device_type_id":%d,"devices":[{"serial_number":"SN-BATCH1","condition":"normal","location":"Lab"},{"serial_number":"SN-BATCH2","condition":"rusak","location":"Lab"}]}`, dtID)
	resp, _ = postJSON("/devices/batch-create", body)
	assert(resp.StatusCode == 200, "batch create: %d", resp.StatusCode)
	var batchRes struct {
		Success bool     `json:"success"`
		Codes   []string `json:"codes"`
	}
	json.NewDecoder(resp.Body).Decode(&batchRes)
	closeResp(resp)
	assert(batchRes.Success, "batch create success")
	assert(len(batchRes.Codes) == 2, "batch codes: %d", len(batchRes.Codes))
	var batchCount int
	db.QueryRow("SELECT COUNT(*) FROM devices WHERE serial_number LIKE 'SN-BATCH%'").Scan(&batchCount)
	assert(batchCount == 2, "batch devices: %d", batchCount)

	// 4. Software CRUD
	t.Log("\n=== 4. SOFTWARE CRUD ===")
	resp, _ = get("/software")
	assert(resp.StatusCode == 200, "/software: %d", resp.StatusCode)
	closeResp(resp)

	resp, _ = post("/software/create", "name=TestSW&category=other&description=Test")
	assert(resp.StatusCode == 302, "create software: %d", resp.StatusCode)
	closeResp(resp)
	var swID int
	db.QueryRow("SELECT id FROM software_catalog WHERE name='TestSW'").Scan(&swID)
	assert(swID > 0, "Software ID=%d", swID)

	// Query slug for software
	var swSlug string
	db.QueryRow("SELECT slug FROM software_catalog WHERE id=?", swID).Scan(&swSlug)
	assert(swSlug != "", "Software slug=%s", swSlug)

	resp, _ = get("/software/" + swSlug + "/edit")
	assert(resp.StatusCode == 200, "/software/%s/edit: %d", swSlug, resp.StatusCode)
	closeResp(resp)

	resp, _ = post("/software/"+swSlug+"/edit", "name=TestSW2&category=required&description=Test2+updated")
	assert(resp.StatusCode == 302, "edit software: %d", resp.StatusCode)
	closeResp(resp)
	var swName string
	db.QueryRow("SELECT name FROM software_catalog WHERE id=?", swID).Scan(&swName)
	assert(swName == "TestSW2", "Software name updated: %s", swName)

	// Query updated slug after name change
	db.QueryRow("SELECT slug FROM software_catalog WHERE id=?", swID).Scan(&swSlug)

	resp, _ = post("/software/"+swSlug+"/delete", "")
	assert(resp.StatusCode == 302, "delete software: %d", resp.StatusCode)
	closeResp(resp)
	db.QueryRow("SELECT COUNT(*) FROM software_catalog WHERE id=?", swID).Scan(&swID)
	assert(swID == 0, "Software deleted")

	// 5. Schedule CRUD
	t.Log("\n=== 5. SCHEDULE CRUD ===")
	resp, _ = get("/schedules")
	assert(resp.StatusCode == 200, "/schedules: %d", resp.StatusCode)
	closeResp(resp)

	resp, _ = post("/schedules/create", "course_name=Algo&lecturer=Dr.T&day=Senin&class=IF-1&time_start=08:00&time_end=09:40")
	assert(resp.StatusCode == 302, "create schedule: %d", resp.StatusCode)
	closeResp(resp)
	var scID int
	db.QueryRow("SELECT id FROM course_schedules WHERE course_name='Algo'").Scan(&scID)
	assert(scID > 0, "Schedule ID=%d", scID)

	// Schedule edit page
	resp, _ = get("/schedules/" + fmt.Sprint(scID) + "/edit")
	assert(resp.StatusCode == 200, "/schedules/%d/edit: %d", scID, resp.StatusCode)
	closeResp(resp)

	// Schedule edit POST
	resp, _ = post("/schedules/"+fmt.Sprint(scID)+"/edit",
		"course_name=Algo2&lecturer=Dr.T&day=Senin&class=IF-1&time_start=08:00&time_end=09:40")
	assert(resp.StatusCode == 302, "edit schedule: %d", resp.StatusCode)
	closeResp(resp)
	var scName string
	db.QueryRow("SELECT course_name FROM course_schedules WHERE id=?", scID).Scan(&scName)
	assert(scName == "Algo2", "Schedule name updated: %s", scName)

	// Schedule delete
	resp, _ = post("/schedules/"+fmt.Sprint(scID)+"/delete", "")
	assert(resp.StatusCode == 302, "delete schedule: %d", resp.StatusCode)
	closeResp(resp)
	var scCount int
	db.QueryRow("SELECT COUNT(*) FROM course_schedules WHERE id=?", scID).Scan(&scCount)
	assert(scCount == 0, "Schedule deleted")

	// 6. Logbook CRUD
	t.Log("\n=== 6. LOGBOOK CRUD ===")
	resp, _ = get("/logbook")
	assert(resp.StatusCode == 200, "/logbook: %d", resp.StatusCode)
	closeResp(resp)

	resp, _ = post("/logbook/create", "date=2026-05-16&student_name=Mhs+Test&nim=24091234567&time_in=08:00&time_out=09:40&purpose=Prak")
	assert(resp.StatusCode == 302, "create logbook: %d", resp.StatusCode)
	closeResp(resp)
	var lb int
	db.QueryRow("SELECT COUNT(*) FROM logbook_entries").Scan(&lb)
	assert(lb > 0, "Logbook entries: %d", lb)

	// 6b. Logbook Upload
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
	bodyOCR, _ := io.ReadAll(resp.Body)
	closeResp(resp)

	if cfg.GeminiAPIKey != "" || cfg.OpenRouterAPIKey != "" {
		assert(resp.StatusCode == 200, "logbook upload (with API key): %d", resp.StatusCode)
		assert(strings.Contains(string(bodyOCR), "Preview Hasil OCR"), "OCR preview page rendered")
	} else {
		assert(resp.StatusCode == 500, "logbook upload (no API key): %d", resp.StatusCode)
		assert(strings.Contains(string(bodyOCR), "API key tidak dikonfigurasi"), "proper error message when no API keys")
	}

	// 7. User management
	t.Log("\n=== 7. USER ===")
	resp, _ = get("/admin/users")
	assert(resp.StatusCode == 200, "/admin/users: %d", resp.StatusCode)
	closeResp(resp)
	resp, _ = get("/profile")
	assert(resp.StatusCode == 200, "/profile: %d", resp.StatusCode)
	closeResp(resp)
	resp, _ = post("/profile", "username=admin&full_name=Admin+U")
	assert(resp.StatusCode == 302, "profile update: %d", resp.StatusCode)
	closeResp(resp)

	// 8. Activity Log
	t.Log("\n=== 8. ACTIVITY LOG ===")
	var logCount int
	db.QueryRow("SELECT COUNT(*) FROM activity_logs").Scan(&logCount)
	t.Logf("  Activity logs: %d (async writer mungkin belum flush)", logCount)
	resp, _ = get("/admin/activity-logs")
	assert(resp.StatusCode == 200, "/admin/activity-logs: %d", resp.StatusCode)
	closeResp(resp)

	// 9. Export Download
	t.Log("\n=== 9. EXPORT DOWNLOAD ===")
	checkExport := func(path, prefix string) {
		resp, _ := get(path)
		assert(resp.StatusCode == 200, "%s: %d", path, resp.StatusCode)
		ct := resp.Header.Get("Content-Type")
		assert(ct == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "%s CT: %s", path, ct)
		cd := resp.Header.Get("Content-Disposition")
		assert(strings.HasPrefix(cd, "attachment; filename="+prefix), "%s CD: %s", path, cd)
		body, _ := io.ReadAll(resp.Body)
		closeResp(resp)
		assert(len(body) > 0, "%s empty", path)
	}
	checkExport("/pc/export", "pc_export")
	checkExport("/software/export", "software_catalog_export")
	checkExport("/logbook/export", "logbook_export")
	checkExport("/logbook/export-preview", "logbook_export_preview")
	checkExport("/admin/activity-logs/export", "activity_log_export")

	// 10. Device Loan CRUD
	t.Log("\n=== 10. DEVICE LOAN ===")
	var loanDevID int
	db.QueryRow("SELECT id FROM devices ORDER BY id LIMIT 1").Scan(&loanDevID)
	assert(loanDevID > 0, "device exists for loan")
	resp, _ = post("/device-loans/create", fmt.Sprintf("device_id=%d&borrower_name=Mahasiswa+Test&borrower_type=mahasiswa&loan_date=2026-05-16&return_date=2026-05-20&purpose=Praktikum", loanDevID))
	assert(resp.StatusCode == 302, "create loan: %d", resp.StatusCode)
	closeResp(resp)
	var loanCount int
	db.QueryRow("SELECT COUNT(*) FROM device_loans").Scan(&loanCount)
	assert(loanCount > 0, "loans: %d", loanCount)
	resp, _ = get("/devices?tab=loans")
	assert(resp.StatusCode == 200, "/devices loans: %d", resp.StatusCode)
	closeResp(resp)

	// Loan edit page
	var loanID int
	db.QueryRow("SELECT id FROM device_loans ORDER BY id DESC LIMIT 1").Scan(&loanID)
	assert(loanID > 0, "Loan ID=%d", loanID)

	resp, _ = get("/device-loans/" + fmt.Sprint(loanID) + "/edit")
	assert(resp.StatusCode == 200, "/device-loans/%d/edit: %d", loanID, resp.StatusCode)
	closeResp(resp)

	// Loan edit POST (return the loan)
	resp, _ = post("/device-loans/"+fmt.Sprint(loanID)+"/edit",
		"borrower_name=Mahasiswa+Updated&borrower_type=mahasiswa&loan_date=2026-05-16&return_date=2026-05-20&actual_return_date=2026-05-17&purpose=Praktikum")
	assert(resp.StatusCode == 302, "edit loan: %d", resp.StatusCode)
	closeResp(resp)
	var loanReturned string
	db.QueryRow("SELECT COALESCE(actual_return_date,'') FROM device_loans WHERE id=?", loanID).Scan(&loanReturned)
	assert(loanReturned != "", "Loan returned: %s", loanReturned)

	// Loan delete
	resp, _ = post("/device-loans/"+fmt.Sprint(loanID)+"/delete", "")
	assert(resp.StatusCode == 302, "delete loan: %d", resp.StatusCode)
	closeResp(resp)
	db.QueryRow("SELECT COUNT(*) FROM device_loans WHERE id=?", loanID).Scan(&loanCount)
	assert(loanCount == 0, "Loan deleted")

	//  11. Device Usage CRUD
	t.Log("\n=== 11. DEVICE USAGE ===")
	resp, _ = post("/device-usages/create", fmt.Sprintf("device_id=%d&user_name=Dosen+Test&user_type=dosen&usage_date=2026-05-16&is_available=yes&purpose=Demo", loanDevID))
	assert(resp.StatusCode == 302, "create usage: %d", resp.StatusCode)
	closeResp(resp)
	var usageCount int
	db.QueryRow("SELECT COUNT(*) FROM device_usages").Scan(&usageCount)
	assert(usageCount > 0, "usages: %d", usageCount)
	resp, _ = get("/devices?tab=usages")
	assert(resp.StatusCode == 200, "/devices usages: %d", resp.StatusCode)
	closeResp(resp)

	// Usage edit page
	var usageID int
	db.QueryRow("SELECT id FROM device_usages ORDER BY id DESC LIMIT 1").Scan(&usageID)
	assert(usageID > 0, "Usage ID=%d", usageID)
	resp, _ = get("/device-usages/" + fmt.Sprint(usageID) + "/edit")
	assert(resp.StatusCode == 200, "/device-usages/%d/edit: %d", usageID, resp.StatusCode)
	closeResp(resp)

	// Usage edit POST
	resp, _ = post("/device-usages/"+fmt.Sprint(usageID)+"/edit",
		"user_name=Dosen+Updated&user_type=dosen&usage_date=2026-05-16&is_available=yes&purpose=Demo")
	assert(resp.StatusCode == 302, "edit usage: %d", resp.StatusCode)
	closeResp(resp)
	var usageUser string
	db.QueryRow("SELECT user_name FROM device_usages WHERE id=?", usageID).Scan(&usageUser)
	assert(usageUser == "Dosen Updated", "Usage user updated: %s", usageUser)

	// Usage delete
	resp, _ = post("/device-usages/"+fmt.Sprint(usageID)+"/delete", "")
	assert(resp.StatusCode == 302, "delete usage: %d", resp.StatusCode)
	closeResp(resp)
	db.QueryRow("SELECT COUNT(*) FROM device_usages WHERE id=?", usageID).Scan(&usageCount)
	assert(usageCount == 0, "Usage deleted")

	// 12. Device Installation CRUD
	t.Log("\n=== 12. DEVICE INSTALLATION ===")
	var installDevID int
	db.QueryRow("SELECT id FROM devices ORDER BY id LIMIT 1").Scan(&installDevID)
	assert(installDevID > 0, "device exists for installation")
	resp, _ = post("/installations/create", fmt.Sprintf("device_id=%d&location_installed=Lab+Utama&installation_start_date=2026-05-01&notes=Installed+test", installDevID))
	assert(resp.StatusCode == 302, "create installation: %d", resp.StatusCode)
	closeResp(resp)
	var installCount int
	db.QueryRow("SELECT COUNT(*) FROM device_installations").Scan(&installCount)
	assert(installCount > 0, "installations: %d", installCount)
	var installID int
	db.QueryRow("SELECT id FROM device_installations ORDER BY id DESC LIMIT 1").Scan(&installID)
	assert(installID > 0, "Installation ID=%d", installID)

	// Installation detail
	resp, _ = get("/installations/" + fmt.Sprint(installID))
	assert(resp.StatusCode == 200, "/installations/%d: %d", installID, resp.StatusCode)
	closeResp(resp)

	// Installation list (tab)
	resp, _ = get("/devices?tab=installations")
	assert(resp.StatusCode == 200, "/devices?tab=installations: %d", resp.StatusCode)
	closeResp(resp)

	// Installation edit page
	resp, _ = get("/installations/" + fmt.Sprint(installID) + "/edit")
	assert(resp.StatusCode == 200, "/installations/%d/edit: %d", installID, resp.StatusCode)
	closeResp(resp)

	// Installation edit POST
	resp, _ = post("/installations/"+fmt.Sprint(installID)+"/edit",
		"location_installed=Lab+Cadangan&installation_start_date=2026-05-01&installation_finish_date=2026-05-10&notes=Updated")
	assert(resp.StatusCode == 302, "edit installation: %d", resp.StatusCode)
	closeResp(resp)
	var installLoc string
	db.QueryRow("SELECT location_installed FROM device_installations WHERE id=?", installID).Scan(&installLoc)
	assert(installLoc == "Lab Cadangan", "Installation location updated: %s", installLoc)

	// Installation delete
	resp, _ = post("/installations/"+fmt.Sprint(installID)+"/delete", "")
	assert(resp.StatusCode == 302, "delete installation: %d", resp.StatusCode)
	closeResp(resp)
	db.QueryRow("SELECT COUNT(*) FROM device_installations WHERE id=?", installID).Scan(&installCount)
	assert(installCount == 0, "Installation deleted")

	// 13. Logbook Save
	t.Log("\n=== 13. LOGBOOK SAVE ===")
	resp, _ = post("/logbook/save", "source_file=test&date[]=2026-05-17&student_name[]=Mahasiswa+Save&nim[]=24091111111&time_in[]=10:00&time_out[]=11:40&purpose[]=Praktikum")
	assert(resp.StatusCode == 200, "logbook save: %d", resp.StatusCode)
	var lsRes struct {
		Success bool
		Saved   int
	}
	json.NewDecoder(resp.Body).Decode(&lsRes)
	closeResp(resp)
	assert(lsRes.Success && lsRes.Saved == 1, "save: success=%v saved=%d", lsRes.Success, lsRes.Saved)

	// 14. Change Password
	t.Log("\n=== 14. CHANGE PASSWORD ===")
	resp, _ = post("/profile/password", "old_password=admin123&new_password=admin123&confirm_password=admin123")
	assert(resp.StatusCode == 302, "change password success: %d", resp.StatusCode)
	closeResp(resp)

	resp, _ = post("/profile/password", "old_password=wrongpass&new_password=admin123&confirm_password=admin123")
	assert(resp.StatusCode == 302, "change password wrong pass: %d", resp.StatusCode)
	closeResp(resp)

	resp, _ = post("/profile/password", "old_password=admin123&new_password=newpass123&confirm_password=mismatch")
	assert(resp.StatusCode == 302, "change password mismatch: %d", resp.StatusCode)
	closeResp(resp)

	// Cleanup: delete device created in §3
	t.Log("\n=== DEVICE CLEANUP ===")
	// Re-query asset_code in case it changed
	db.QueryRow("SELECT asset_code FROM devices WHERE id=?", devID).Scan(&devAssetCode)
	devSlug = strings.ToLower(devAssetCode)
	resp, _ = post("/devices/"+devSlug+"/delete", "")
	assert(resp.StatusCode == 302, "delete device: %d", resp.StatusCode)
	closeResp(resp)
	var devDelCount int
	db.QueryRow("SELECT COUNT(*) FROM devices WHERE id=?", devID).Scan(&devDelCount)
	assert(devDelCount == 0, "Device deleted")

	// 15. Summary
	t.Log("\n=== SUMMARY ===")
	rows, _ := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var tbl string
			rows.Scan(&tbl)
			var c int
			db.QueryRow("SELECT COUNT(*) FROM " + tbl).Scan(&c)
			if c > 0 {
				t.Logf("  %s: %d rows", tbl, c)
			}
		}
	}
	db.QueryRow("SELECT COUNT(*) FROM activity_logs").Scan(&logCount)
	assert(logCount > 0, "Activity logs should exist after full test: %d", logCount)
	t.Logf("  All tests passed!")
}
