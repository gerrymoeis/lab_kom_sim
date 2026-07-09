package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"inventaris-lab-kom/internal/database"
	"golang.org/x/sync/errgroup"
)

// TestGAB_FullCRUD — Fase 4A: Global Admin (GAB) full CRUD across labs.
//
//  1. create_gab_user     create GAB user via admin panel
//  2. gab_crud_lab_a      login sebagai GAB → full CRUD di Lab A
//  3. gab_crud_lab_b_iso  login sebagai GAB → CRUD di Lab B + isolasi Lab A
func TestGAB_FullCRUD(t *testing.T) {
	env := wrapSharedEnv(t)
	gabUsername := fmt.Sprintf("gab_%d", time.Now().UnixMilli())

	t.Run("create_gab_user", func(t *testing.T) {
		loginAsAdmin(env)

		resp := adminPost(env, "/labs/admin/users/create",
			fmt.Sprintf("username=%s&password=test123&full_name=GAB+User&is_global_admin=1", gabUsername))
		resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 when creating GAB user, got %d", resp.StatusCode)
		}
		var count int
		env.GlobalDB.QueryRow(
			"SELECT COUNT(*) FROM global_users WHERE username=? AND is_global_admin=1", gabUsername,
		).Scan(&count)
		if count != 1 {
			t.Fatalf("GAB user %s not created as global_admin", gabUsername)
		}
	})

	// ———————————————————— Lab A ————————————————————

	t.Run("gab_crud_lab_a", func(t *testing.T) {
		if !loginAndRefresh(env.LabA, gabUsername, "test123") {
			t.Fatal("GAB login for Lab A failed")
		}
		seedGABCatAndType(env.DB_A, 100, "GAB-A-Cat", "GABA", "GAB-A-Type", "GABA")

		// --- PC: create → edit → delete ---
		pcSerial := fmt.Sprintf("GAB-A-PC-%d", time.Now().UnixMilli())
		pcData := url.Values{
			"row": {"1"}, "column": {"1"},
			"status": {"normal"}, "placement": {"dipakai"},
			"is_mahasiswa": {"true"},
			"serial_number": {pcSerial},
			"operating_system": {"Win11"}, "pc_type": {"PC"},
			"brand_model": {"Dell"}, "accessories": {"KB"},
			"processor": {"i7"}, "ram": {"16GB"}, "storage": {"512GB"},
		}.Encode()

		resp, err := env.LabA.post("/pc/create", pcData)
		if err != nil {
			t.Fatalf("POST /pc/create: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 on create PC, got %d", resp.StatusCode)
		}
		var pcLabel string
		env.DB_A.QueryRow("SELECT label FROM pcs WHERE serial_number=?", pcSerial).Scan(&pcLabel)
		if pcLabel == "" {
			t.Fatal("PC not found in Lab A DB after create")
		}

		// edit PC
		resp, err = env.LabA.post("/pc/"+pcLabel+"/edit",
			"status=warning&placement=dipakai&serial_number="+pcSerial+"&operating_system=Win11&pc_type=PC&brand_model=Dell&accessories=KB&processor=i7&ram=16GB&storage=512GB&notes=Updated+by+GAB")
		if err != nil {
			t.Fatalf("POST /pc/edit: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 on edit PC, got %d", resp.StatusCode)
		}
		var pcStatus string
		env.DB_A.QueryRow("SELECT status FROM pcs WHERE label=?", pcLabel).Scan(&pcStatus)
		if pcStatus != "warning" {
			t.Errorf("expected PC status 'warning', got %q", pcStatus)
		}

		// delete PC via batch-delete
		if !env.LabA.refreshCSRF() {
			t.Fatal("failed to refresh CSRF for batch-delete")
		}
		resp, err = env.LabA.postJSON("/pc/batch-delete", fmt.Sprintf(`{"ids":["%s"]}`, pcLabel))
		if err != nil {
			t.Fatalf("POST /pc/batch-delete: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200 on batch delete, got %d", resp.StatusCode)
		}
		var pcRemoved int
		env.DB_A.QueryRow("SELECT COUNT(*) FROM pcs WHERE label=?", pcLabel).Scan(&pcRemoved)
		if pcRemoved != 0 {
			t.Error("PC should be deleted from DB")
		}

		if !env.LabA.refreshCSRF() {
			t.Fatal("failed to refresh CSRF after batch-delete")
		}

		// --- Software ---
		swName := fmt.Sprintf("GAB-A-SW-%d", time.Now().UnixMilli())
		resp, err = env.LabA.post("/software/create",
			"name="+swName+"&category=other&description=GAB+test")
		if err != nil {
			t.Fatalf("POST /software/create: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 on create software, got %d", resp.StatusCode)
		}
		var swID int
		env.DB_A.QueryRow("SELECT id FROM software_catalog WHERE name=?", swName).Scan(&swID)
		if swID == 0 {
			t.Fatal("Software not created in Lab A DB")
		}

		// --- Schedule ---
		resp, err = env.LabA.post("/schedules/create",
			"course_name=GAB-A-Algo&lecturer=Dr.G&day=Senin&class=IF-GAB&time_start=08:00&time_end=09:40")
		if err != nil {
			t.Fatalf("POST /schedules/create: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 on create schedule, got %d", resp.StatusCode)
		}
		var scCount int
		env.DB_A.QueryRow("SELECT COUNT(*) FROM course_schedules WHERE course_name='GAB-A-Algo'").Scan(&scCount)
		if scCount == 0 {
			t.Fatal("Schedule not created in Lab A DB")
		}

		// --- Device ---
		devSerial := fmt.Sprintf("GAB-A-DEV-%d", time.Now().UnixMilli())
		resp, err = env.LabA.post("/devices/create",
			fmt.Sprintf("device_type_id=100&serial_number=%s&condition=normal&location=Lab&purchase_date=&notes=GAB+device", devSerial))
		if err != nil {
			t.Fatalf("POST /devices/create: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 on create device, got %d", resp.StatusCode)
		}
		var devID int
		env.DB_A.QueryRow("SELECT id FROM devices WHERE serial_number=?", devSerial).Scan(&devID)
		if devID == 0 {
			t.Fatal("Device not created in Lab A DB")
		}

		// --- Device Loan ---
		if !env.LabA.refreshCSRF() {
			t.Fatal("failed to refresh CSRF for device loan")
		}
		resp, err = env.LabA.post("/device-loans/create",
			fmt.Sprintf("device_id=%d&borrower_name=GAB+A+Student&borrower_type=mahasiswa&loan_date=2026-07-01&return_date=2026-07-05&purpose=Test", devID))
		if err != nil {
			t.Fatalf("POST /device-loans/create: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 on create loan, got %d", resp.StatusCode)
		}
		env.DB_A.Flush()
		var loanCount int
		env.DB_A.QueryRow("SELECT COUNT(*) FROM device_loans").Scan(&loanCount)
		if loanCount == 0 {
			t.Fatal("Device loan not created in Lab A DB")
		}

		// --- Device Usage ---
		if !env.LabA.refreshCSRF() {
			t.Fatal("failed to refresh CSRF for device usage")
		}
		resp, err = env.LabA.post("/device-usages/create",
			fmt.Sprintf("device_id=%d&user_name=GAB+A+Dosen&user_type=dosen&usage_date=2026-07-01&is_available=yes&purpose=Demo", devID))
		if err != nil {
			t.Fatalf("POST /device-usages/create: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 on create usage, got %d", resp.StatusCode)
		}
		var usageCount int
		env.DB_A.QueryRow("SELECT COUNT(*) FROM device_usages").Scan(&usageCount)
		if usageCount == 0 {
			t.Fatal("Device usage not created in Lab A DB")
		}

		// --- Installation ---
		if !env.LabA.refreshCSRF() {
			t.Fatal("failed to refresh CSRF for installation")
		}
		resp, err = env.LabA.post("/installations/create",
			fmt.Sprintf("device_id=%d&location_installed=Lab+A&installation_start_date=2026-07-01&notes=GAB+installation", devID))
		if err != nil {
			t.Fatalf("POST /installations/create: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302 on create installation, got %d", resp.StatusCode)
		}
		var installCount int
		env.DB_A.QueryRow("SELECT COUNT(*) FROM device_installations").Scan(&installCount)
		if installCount == 0 {
			t.Fatal("Installation not created in Lab A DB")
		}
	})

	// ———————————————————— Lab B + isolasi ————————————————————

	t.Run("gab_crud_lab_b_isolation", func(t *testing.T) {
		// Clear GAB session to avoid ErrAlreadyLoggedIn
		env.GlobalDB.Exec("UPDATE global_users SET session_token = '' WHERE username = ?", gabUsername)

		if !loginAndRefresh(env.LabB, gabUsername, "test123") {
			t.Fatal("GAB login for Lab B failed")
		}
		seedGABCatAndType(env.DB_B, 200, "GAB-B-Cat", "GABB", "GAB-B-Type", "GABB")

		// Record pre state in Lab B
		var prePCCount, preDevCount int
		env.DB_B.QueryRow("SELECT COUNT(*) FROM pcs").Scan(&prePCCount)
		env.DB_B.QueryRow("SELECT COUNT(*) FROM devices").Scan(&preDevCount)

		// --- PC in Lab B ---
		pcSerialB := fmt.Sprintf("GAB-B-PC-%d", time.Now().UnixMilli())
		pcDataB := url.Values{
			"row": {"2"}, "column": {"2"},
			"status": {"normal"}, "placement": {"dipakai"},
			"is_mahasiswa": {"true"},
			"serial_number": {pcSerialB},
			"operating_system": {"Win11"}, "pc_type": {"PC"},
			"brand_model": {"Dell"}, "accessories": {"KB"},
			"processor": {"i5"}, "ram": {"8GB"}, "storage": {"256GB"},
		}.Encode()

		resp, err := env.LabB.post("/pc/create", pcDataB)
		if err != nil {
			t.Fatalf("POST Lab B pc/create: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var pcIDB int
		env.DB_B.QueryRow("SELECT id FROM pcs WHERE serial_number=?", pcSerialB).Scan(&pcIDB)
		if pcIDB == 0 {
			t.Fatal("PC not created in Lab B DB")
		}

		// Verify Lab A does NOT have this PC
		var pcInA int
		env.DB_A.QueryRow("SELECT COUNT(*) FROM pcs WHERE serial_number=?", pcSerialB).Scan(&pcInA)
		if pcInA != 0 {
			t.Error("Lab A leaked Lab B PC data — isolation breach")
		}

		// --- Device in Lab B ---
		devSerialB := fmt.Sprintf("GAB-B-DEV-%d", time.Now().UnixMilli())
		resp, err = env.LabB.post("/devices/create",
			fmt.Sprintf("device_type_id=200&serial_number=%s&condition=normal&location=Lab&purchase_date=&notes=GAB+B+device", devSerialB))
		if err != nil {
			t.Fatalf("POST Lab B devices/create: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var devIDB int
		env.DB_B.QueryRow("SELECT id FROM devices WHERE serial_number=?", devSerialB).Scan(&devIDB)
		if devIDB == 0 {
			t.Fatal("Device not created in Lab B DB")
		}

		// --- Device Loan in Lab B ---
		if !env.LabB.refreshCSRF() {
			t.Fatal("failed to refresh CSRF for loan")
		}
		env.LabB.post("/device-loans/create",
			fmt.Sprintf("device_id=%d&borrower_name=GAB+B+Student&borrower_type=mahasiswa&loan_date=2026-07-01&return_date=2026-07-05&purpose=Test", devIDB))

		// Verify Lab B counts increased
		var postPCCount int
		env.DB_B.QueryRow("SELECT COUNT(*) FROM pcs").Scan(&postPCCount)
		if postPCCount != prePCCount+1 {
			t.Errorf("Lab B PC count expected %d, got %d", prePCCount+1, postPCCount)
		}
		var postDevCount int
		env.DB_B.QueryRow("SELECT COUNT(*) FROM devices").Scan(&postDevCount)
		if postDevCount != preDevCount+1 {
			t.Errorf("Lab B device count expected %d, got %d", preDevCount+1, postDevCount)
		}

		// Verify Lab A isolation: none of Lab B's unique serials exist in Lab A
		var devInA int
		env.DB_A.QueryRow("SELECT COUNT(*) FROM devices WHERE serial_number=?", devSerialB).Scan(&devInA)
		if devInA != 0 {
			t.Error("Lab A leaked Lab B device data — isolation breach")
		}
	})
}

// seedGABCatAndType inserts a category + device_type (same ID) for device CRUD by GAB.
func seedGABCatAndType(db *database.DB, id int, catName, catPrefix, dtName, dtPrefix string) {
	db.Exec("INSERT OR IGNORE INTO categories (id, name, label_prefix) VALUES (?, ?, ?)", id, catName, catPrefix)
	db.Exec("INSERT OR IGNORE INTO device_types (id, category_id, name, brand, model, label_prefix, usage_type, default_location) VALUES (?, ?, ?, 'GAB', 'Type', ?, 'loanable', 'Lab')",
		id, id, dtName, dtPrefix)
}

// ============================================
// Fase 4B — Super Admin full CRUD across labs
// ============================================

// TestSA_FullCRUD — Fase 4B: Super Admin full CRUD across labs.
//
//  1. sa_create_lab_login_main_account    Create lab baru + login sebagai main account
//  2. sa_crud_all_labs                    Login sbg SA → CRUD di semua lab (A, B, baru)
//  3. sa_delete_lab_verify_isolation      Delete lab baru + verifikasi isolasi
func TestSA_FullCRUD(t *testing.T) {
	env := setupTestEnvironment(t)

	// Set EnvPath — required by lab creation handler (writes .env)
	envFile := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(envFile, []byte("EXISTING_VAR=1\n"), 0644)
	env.Config.EnvPath = envFile

	adminLogin(env)

	newLabURL := "testlab1"

	// ———————————————————— Create lab + main account ————————————————————

	t.Run("sa_create_lab_login_main_account", func(t *testing.T) {
		saCreateLab(t, env, newLabURL)

		if !env.LabA.login(newLabURL, newLabURL+"123") {
			t.Fatal("login as main account failed")
		}
		resp, err := env.LabA.getURL(env.TS.URL + "/" + newLabURL + "/dashboard")
		if err != nil {
			t.Fatalf("GET /%s/dashboard: %v", newLabURL, err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200 for main account dashboard, got %d", resp.StatusCode)
		}
	})

	// ———————————————————— Full CRUD di semua lab ————————————————————

	t.Run("sa_crud_all_labs", func(t *testing.T) {
		// Re-login as admin (main account login in subtest 1 changed cookies)
		env.GlobalDB.Exec("UPDATE global_users SET session_token = '' WHERE username = 'admin'")
		adminLogin(env)

		newLabDB := env.GlobalHandler.LabsDB[newLabURL]
		if newLabDB == nil {
			t.Fatal("new lab DB not found in GlobalHandler")
		}

		// Helper: copy admin cookies to another lab for same-session CRUD
		copyLab := func(dst *testLab) {
			dst.cookies = make(map[string]string)
			for k, v := range env.LabA.cookies {
				dst.cookies[k] = v
			}
			dst.csrf = env.LabA.csrf
		}

		saCRUDInLab(t, env.LabA, env.DB_A, "A", env.Config.UploadPath)

		copyLab(env.LabB)
		saCRUDInLab(t, env.LabB, env.DB_B, "B", env.Config.UploadPath)

		newLab := saNewLab(env, newLabURL, env.LabA, t)
		saCRUDInLab(t, newLab, newLabDB, "NEW", env.Config.UploadPath)
	})

	// ———————————————————— Delete lab + isolasi ————————————————————

	t.Run("sa_delete_lab_verify_isolation", func(t *testing.T) {
		env.GlobalDB.Exec("UPDATE global_users SET session_token = '' WHERE username = 'admin'")
		adminLogin(env)

		// Re-login only sets env.LabA.cookies; copy to env.LabB too
		env.LabB.cookies = copyMap(env.LabA.cookies)
		env.LabB.csrf = env.LabA.csrf

		existingDir := filepath.Dir(env.Config.Labs[0].DBPath)
		dbPath := filepath.Join(existingDir, "lab_"+newLabURL+".db")

		saDeleteLab(t, env, newLabURL)

		if _, err := os.Stat(dbPath + ".deleted"); os.IsNotExist(err) {
			t.Error("expected DB to be renamed to .deleted")
		}

		if !env.LabA.refreshCSRF() {
			t.Fatal("refresh CSRF after delete failed")
		}
		saVerifyLabAccessible(t, env.LabA, env.TS.URL+"/lab-kom-mi/dashboard")

		if !env.LabB.refreshCSRF() {
			t.Fatal("Lab B refresh CSRF failed")
		}
		saVerifyLabAccessible(t, env.LabB, env.TS.URL+"/vokasi/dashboard")

		resp, _ := env.LabA.getURL(env.TS.URL + "/" + newLabURL + "/dashboard")
		if resp != nil {
			resp.Body.Close()
			if resp.StatusCode != 302 {
				t.Errorf("expected 302 for deleted lab, got %d", resp.StatusCode)
			}
		}
	})

	t.Run("sa_delete_lab_cleans_global_user", func(t *testing.T) {
		var count int
		env.GlobalDB.QueryRow("SELECT COUNT(*) FROM global_users WHERE username = ?", newLabURL).Scan(&count)
		if count != 0 {
			t.Errorf("expected main account '%s' to be deleted, but found %d row(s)", newLabURL, count)
		}

		// Super admin should still exist
		var adminCount int
		env.GlobalDB.QueryRow("SELECT COUNT(*) FROM global_users WHERE username = 'admin'").Scan(&adminCount)
		if adminCount != 1 {
			t.Errorf("expected super admin to remain, got %d", adminCount)
		}
	})
}

// ————— helpers —————

func adminLogin(env *TestEnvironment) {
	loginAsAdmin(env)
}

func saCreateLab(t *testing.T, env *TestEnvironment, urlPath string) {
	if !env.LabA.refreshCSRF() {
		t.Fatal("refresh CSRF failed")
	}
	formData := url.Values{
		"_csrf": {env.LabA.csrf},
		"id":    {"TEST-" + urlPath},
		"title": {"Test Lab " + urlPath},
		"url":   {urlPath},
		"rows":  {"2"},
		"cols":  {"8,8"},
	}.Encode()
	req, _ := http.NewRequest("POST", env.TS.URL+"/labs/create", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	env.LabA.addCookies(req)
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatalf("POST /labs/create: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 302 {
		t.Fatalf("expected 302 when creating lab, got %d", resp.StatusCode)
	}
}

func saDeleteLab(t *testing.T, env *TestEnvironment, urlPath string) {
	if !env.LabA.refreshCSRF() {
		t.Fatal("refresh CSRF failed")
	}
	formData := url.Values{"_csrf": {env.LabA.csrf}}.Encode()
	req, _ := http.NewRequest("POST", env.TS.URL+"/labs/"+urlPath+"/delete", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	env.LabA.addCookies(req)
	resp, err := env.Client.Do(req)
	if err != nil {
		t.Fatalf("POST /labs/%s/delete: %v", urlPath, err)
	}
	resp.Body.Close()
	if resp.StatusCode != 302 {
		t.Errorf("expected 302 when deleting lab, got %d", resp.StatusCode)
	}
}

func saVerifyLabAccessible(t *testing.T, lab *testLab, url string) {
	t.Helper()
	resp, err := lab.getURL(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for %s, got %d", url, resp.StatusCode)
	}
}

func saNewLab(env *TestEnvironment, urlPath string, src *testLab, t *testing.T) *testLab {
	return &testLab{
		url:     urlPath,
		prefix:  "/" + urlPath,
		cookies: copyMap(src.cookies),
		csrf:    src.csrf,
		ts:      env.TS,
		t:       t,
		client:  env.Client,
	}
}

func copyMap(m map[string]string) map[string]string {
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// saCRUDInLab performs all CRUD operations in one lab as Super Admin.
func saCRUDInLab(t *testing.T, lab *testLab, db *database.DB, label, uploadDir string) {
	t.Helper()
	now := fmt.Sprintf("%d", time.Now().UnixMilli())

	// Seed category + device_type for device CRUD
	catID := saCatID(label)
	db.Exec("INSERT OR IGNORE INTO categories (id, name, label_prefix) VALUES (?, ?, ?)", catID, label+"-Cat", label+"C")
	db.Exec("INSERT OR IGNORE INTO device_types (id, category_id, name, brand, model, label_prefix, usage_type, default_location) VALUES (?, ?, ?, 'SABrand', 'SAModel', ?, 'loanable', 'Lab')",
		catID, catID, label+"-Type", label+"D")

	// ============ PC ============
	pcSerial := fmt.Sprintf("SA-%s-PC-%s", label, now)
	postForm(t, lab, "/pc/create", url.Values{
		"row": {"1"}, "column": {"1"},
		"status": {"normal"}, "placement": {"dipakai"},
		"is_mahasiswa": {"true"},
		"serial_number": {pcSerial},
		"operating_system": {"Win11"}, "pc_type": {"PC"},
		"brand_model": {"Dell"}, "accessories": {"KB"},
		"processor": {"i7"}, "ram": {"16GB"}, "storage": {"512GB"},
	}.Encode(), 302)
	var pcLabel string
	db.QueryRow("SELECT label FROM pcs WHERE serial_number=?", pcSerial).Scan(&pcLabel)
	if pcLabel == "" {
		t.Fatalf("[%s] PC not found after create", label)
	}

	postForm(t, lab, "/pc/"+pcLabel+"/edit",
		"status=warning&placement=dipakai&serial_number="+pcSerial+"&operating_system=Win11&pc_type=PC&brand_model=Dell&accessories=KB&processor=i7&ram=16GB&storage=512GB&notes=SA+edited",
		302)
	var pcStatus string
	db.QueryRow("SELECT status FROM pcs WHERE label=?", pcLabel).Scan(&pcStatus)
	if pcStatus != "warning" {
		t.Errorf("[%s] expected PC status 'warning', got %q", label, pcStatus)
	}

	lab.refreshCSRF()
	postJSON(t, lab, "/pc/batch-delete", fmt.Sprintf(`{"ids":["%s"]}`, pcLabel), 200)
	var pcCount int
	db.QueryRow("SELECT COUNT(*) FROM pcs WHERE label=?", pcLabel).Scan(&pcCount)
	if pcCount != 0 {
		t.Errorf("[%s] PC should be deleted", label)
	}
	lab.refreshCSRF()

	// ============ Software ============
	swName := fmt.Sprintf("SA-%s-SW-%s", label, now)
	postForm(t, lab, "/software/create", "name="+swName+"&category=other&description=SA+test", 302)
	var swID int
	db.QueryRow("SELECT id FROM software_catalog WHERE name=?", swName).Scan(&swID)
	if swID == 0 {
		t.Fatalf("[%s] Software not found after create", label)
	}
	var swSlug string
	db.QueryRow("SELECT slug FROM software_catalog WHERE id=?", swID).Scan(&swSlug)

	postForm(t, lab, "/software/"+swSlug+"/edit",
		"name="+swName+"&category=educational&description=Edited+by+SA", 302)

	lab.refreshCSRF()
	postJSON(t, lab, "/software/batch-delete", fmt.Sprintf(`{"ids":["%d"]}`, swID), 200)
	var swCount int
	db.QueryRow("SELECT COUNT(*) FROM software_catalog WHERE id=?", swID).Scan(&swCount)
	if swCount != 0 {
		t.Errorf("[%s] Software should be deleted", label)
	}
	lab.refreshCSRF()

	// ============ Schedule ============
	courseName := fmt.Sprintf("SA-%s-Sched-%s", label, now)
	postForm(t, lab, "/schedules/create",
		"course_name="+courseName+"&lecturer=Dr.SA&day=Senin&class=IF-SA&time_start=08:00&time_end=09:40", 302)
	db.Flush()
	var scID int
	db.QueryRow("SELECT id FROM course_schedules WHERE course_name=?", courseName).Scan(&scID)
	if scID == 0 {
		t.Fatalf("[%s] Schedule not found after create", label)
	}

	postForm(t, lab, fmt.Sprintf("/schedules/%d/edit", scID),
		"course_name="+courseName+"&lecturer=Prof.SA&day=Selasa&class=IF-SA&time_start=10:00&time_end=11:40", 302)

	lab.refreshCSRF()
	postForm(t, lab, fmt.Sprintf("/schedules/%d/delete", scID), "", 302)
	var scCount int
	db.QueryRow("SELECT COUNT(*) FROM course_schedules WHERE id=?", scID).Scan(&scCount)
	if scCount != 0 {
		t.Errorf("[%s] Schedule should be deleted", label)
	}
	lab.refreshCSRF()

	// ============ Device ============
	devSerial := fmt.Sprintf("SA-%s-DEV-%s", label, now)
	postForm(t, lab, "/devices/create",
		fmt.Sprintf("device_type_id=%d&serial_number=%s&condition=normal&location=Lab&purchase_date=&notes=SA+device", catID, devSerial), 302)
	var devID int
	db.QueryRow("SELECT id FROM devices WHERE serial_number=?", devSerial).Scan(&devID)
	if devID == 0 {
		t.Fatalf("[%s] Device not found after create", label)
	}
	var devLabel string
	db.QueryRow("SELECT label FROM devices WHERE id=?", devID).Scan(&devLabel)

	postForm(t, lab, fmt.Sprintf("/devices/%s/edit", devLabel),
		fmt.Sprintf("device_type_id=%d&serial_number=%s&condition=rusak&location=Gudang&purchase_date=&notes=SA+edited", catID, devSerial), 302)
	var devCond string
	db.QueryRow("SELECT condition FROM devices WHERE id=?", devID).Scan(&devCond)
	if devCond != "rusak" {
		t.Errorf("[%s] expected device condition 'rusak', got %q", label, devCond)
	}

	lab.refreshCSRF()
	postJSON(t, lab, "/devices/batch-delete", fmt.Sprintf(`{"ids":["%d"]}`, devID), 200)
	var devCount int
	db.QueryRow("SELECT COUNT(*) FROM devices WHERE id=?", devID).Scan(&devCount)
	if devCount != 0 {
		t.Errorf("[%s] Device should be deleted", label)
	}
	lab.refreshCSRF()

	// ============ Device Loan ============
	// Re-create a device for loan/usage/installation tests
	loanDevSerial := fmt.Sprintf("SA-%s-LDEV-%s", label, now)
	postForm(t, lab, "/devices/create",
		fmt.Sprintf("device_type_id=%d&serial_number=%s&condition=normal&location=Lab&purchase_date=&notes=SA+loan+device", catID, loanDevSerial), 302)
	var loanDevID int
	db.QueryRow("SELECT id FROM devices WHERE serial_number=?", loanDevSerial).Scan(&loanDevID)
	if loanDevID == 0 {
		t.Fatalf("[%s] Loan device not found after create", label)
	}

	lab.refreshCSRF()
	postForm(t, lab, "/device-loans/create",
		fmt.Sprintf("device_id=%d&borrower_name=SA+Student+%s&borrower_type=mahasiswa&loan_date=2026-07-01&return_date=2026-07-05&purpose=Test", loanDevID, label), 302)
	db.Flush()
	var loanID int
	db.QueryRow("SELECT id FROM device_loans ORDER BY id DESC LIMIT 1").Scan(&loanID)
	if loanID == 0 {
		t.Fatalf("[%s] Device loan not found after create", label)
	}

	lab.refreshCSRF()
	// extend returns JSON 200, not redirect 302
	respExt, errExt := lab.post(fmt.Sprintf("/device-loans/%d/extend", loanID), "return_date=2026-07-12")
	if errExt != nil {
		t.Errorf("[%s] POST /device-loans/%d/extend: %v", label, loanID, errExt)
	} else {
		respExt.Body.Close()
		if respExt.StatusCode != 200 && respExt.StatusCode != 302 {
			t.Errorf("[%s] POST /device-loans/%d/extend: expected 200/302, got %d", label, loanID, respExt.StatusCode)
		}
	}

	lab.refreshCSRF()
	postForm(t, lab, fmt.Sprintf("/device-loans/%d/delete", loanID), "", 302)
	var loanCount int
	db.QueryRow("SELECT COUNT(*) FROM device_loans WHERE id=?", loanID).Scan(&loanCount)
	if loanCount != 0 {
		t.Errorf("[%s] Device loan should be deleted", label)
	}
	lab.refreshCSRF()

	// ============ Device Usage ============
	lab.refreshCSRF()
	postForm(t, lab, "/device-usages/create",
		fmt.Sprintf("device_id=%d&user_name=SA+Dosen+%s&user_type=dosen&usage_date=2026-07-01&is_available=yes&purpose=Demo", loanDevID, label), 302)
	var usageID int
	db.QueryRow("SELECT id FROM device_usages ORDER BY id DESC LIMIT 1").Scan(&usageID)
	if usageID == 0 {
		t.Fatalf("[%s] Device usage not found after create", label)
	}

	lab.refreshCSRF()
	postForm(t, lab, fmt.Sprintf("/device-usages/%d/edit", usageID),
		fmt.Sprintf("device_id=%d&user_name=SA+Dosen+%s&user_type=dosen&usage_date=2026-07-02&is_available=no&purpose=Demo+edited", loanDevID, label), 302)
	var usageDate string
	db.QueryRow("SELECT usage_date FROM device_usages WHERE id=?", usageID).Scan(&usageDate)
	if usageDate == "" {
		t.Errorf("[%s] Device usage date should not be empty after edit", label)
	}

	lab.refreshCSRF()
	postForm(t, lab, fmt.Sprintf("/device-usages/%d/delete", usageID), "", 302)
	var usageCount int
	db.QueryRow("SELECT COUNT(*) FROM device_usages WHERE id=?", usageID).Scan(&usageCount)
	if usageCount != 0 {
		t.Errorf("[%s] Device usage should be deleted", label)
	}
	lab.refreshCSRF()

	// ============ Installation ============
	lab.refreshCSRF()
	postForm(t, lab, "/installations/create",
		fmt.Sprintf("device_id=%d&location_installed=Lab+SA&installation_start_date=2026-07-01&notes=SA+installation", loanDevID), 302)
	var installID int
	db.QueryRow("SELECT id FROM device_installations ORDER BY id DESC LIMIT 1").Scan(&installID)
	if installID == 0 {
		t.Fatalf("[%s] Installation not found after create", label)
	}

	lab.refreshCSRF()
	postForm(t, lab, fmt.Sprintf("/installations/%d/edit", installID),
		fmt.Sprintf("device_id=%d&location_installed=Lab+SA+Edited&installation_start_date=2026-07-02&notes=SA+edited+installation", loanDevID), 302)
	var installLoc string
	db.QueryRow("SELECT location_installed FROM device_installations WHERE id=?", installID).Scan(&installLoc)
	if installLoc != "Lab SA Edited" {
		t.Errorf("[%s] expected installation location 'Lab SA Edited', got %q", label, installLoc)
	}

	lab.refreshCSRF()
	postForm(t, lab, fmt.Sprintf("/installations/%d/delete", installID), "", 302)
	var installCount int
	db.QueryRow("SELECT COUNT(*) FROM device_installations WHERE id=?", installID).Scan(&installCount)
	if installCount != 0 {
		t.Errorf("[%s] Installation should be deleted", label)
	}
	lab.refreshCSRF()

	// Clean up the loan device
	lab.refreshCSRF()
	postJSON(t, lab, "/devices/batch-delete", fmt.Sprintf(`{"ids":["%d"]}`, loanDevID), 200)
	lab.refreshCSRF()

	// ============ Upload photo ============
	uploadPCSerial := fmt.Sprintf("SA-%s-UPC-%s", label, now)
	postForm(t, lab, "/pc/create", url.Values{
		"row": {"1"}, "column": {"1"},
		"status": {"normal"}, "placement": {"dipakai"},
		"is_mahasiswa": {"true"},
		"serial_number": {uploadPCSerial},
		"operating_system": {"Win11"}, "pc_type": {"PC"},
		"brand_model": {"Dell"}, "accessories": {"KB"},
		"processor": {"i7"}, "ram": {"16GB"}, "storage": {"512GB"},
	}.Encode(), 302)

	imgData, err := createTestJPEG(10, 10)
	if err != nil {
		t.Fatalf("[%s] create JPEG: %v", label, err)
	}
	resp, err := uploadMultipart(lab, lab.ts.URL, imgData, uploadPCSerial+".jpg", "serial")
	if err != nil {
		t.Fatalf("[%s] upload: %v", label, err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("[%s] upload: expected 200, got %d", label, resp.StatusCode)
	}
	uploadResult, decErr := decodeUploadResponse(resp)
	if decErr != nil {
		t.Errorf("[%s] decode upload response: %v", label, decErr)
	} else if uploadResult != nil {
		if _, ok := uploadResult["file_ref"].(string); !ok {
			t.Errorf("[%s] upload response missing file_ref", label)
		}
	}

	// ============ Export Excel ============
	checkExport(t, lab, "/pc/export", "pc_export_")
	checkExport(t, lab, "/software/export", "software_catalog_export_")
	checkExport(t, lab, "/devices/export", "devices_export_")

	// ============ Print sticker ============
	var printPCLabel string
	db.QueryRow("SELECT label FROM pcs WHERE serial_number=?", uploadPCSerial).Scan(&printPCLabel)
	if printPCLabel != "" {
		resp, err = lab.get(fmt.Sprintf("/print/generate?type=pc&pc_labels=%s&font_size=0.5&padding_h=0.3&padding_v=0.3&paper_size=A4&num_sheets=1", printPCLabel))
		if err != nil {
			t.Fatalf("[%s] GET /print/generate: %v", label, err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("[%s] expected 200 on print, got %d", label, resp.StatusCode)
		}
	}
}

// postForm is a thin helper that POSTs URL-encoded form data and verifies status.
func postForm(t *testing.T, lab *testLab, path, data string, wantStatus int) {
	t.Helper()
	resp, err := lab.post(path, data)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	resp.Body.Close()
	if resp.StatusCode != wantStatus {
		t.Errorf("POST %s: expected %d, got %d", path, wantStatus, resp.StatusCode)
	}
}

// postJSON is a thin helper that POSTs JSON data and verifies status.
func postJSON(t *testing.T, lab *testLab, path, data string, wantStatus int) {
	t.Helper()
	resp, err := lab.postJSON(path, data)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	resp.Body.Close()
	if resp.StatusCode != wantStatus {
		t.Errorf("POST %s: expected %d, got %d", path, wantStatus, resp.StatusCode)
	}
}

// saCatID returns a unique category ID for a lab label.
func saCatID(label string) int {
	switch label {
	case "A":
		return 300
	case "B":
		return 400
	default:
		return 500
	}
}

// ——————————————————— Fase 4C: Concurrent Operations ———————————————————

// TestConcurrentPC_Creation — 10 goroutines across 2 labs, bounded by errgroup.
func TestConcurrentPC_Creation(t *testing.T) {
	env := wrapSharedEnv(t)

	cookies, csrf := loginAsAdmin(env)

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(10)

	noRedirect := func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	// 5 goroutines → Lab A
	for i := 0; i < 5; i++ {
		i := i
		g.Go(func() error {
			client := &http.Client{CheckRedirect: noRedirect}
			label := fmt.Sprintf("pc-conc-a-%d", 100+i)
			serial := fmt.Sprintf("SN-CONC-A-%d", 100+i)
			body := url.Values{
				"_csrf":            {csrf},
				"label":            {label},
				"serial_number":    {serial},
				"operating_system": {"Win11"},
				"status":           {"normal"},
				"row":              {fmt.Sprintf("%d", i+1)},
				"column":           {"1"},
			}.Encode()
			req, err := http.NewRequestWithContext(ctx, "POST",
				env.TS.URL+"/lab-kom-mi/pc/create",
				strings.NewReader(body))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			for k, v := range cookies {
				req.AddCookie(&http.Cookie{Name: k, Value: v})
			}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusOK {
				return fmt.Errorf("PC %s: unexpected status %d", label, resp.StatusCode)
			}
			return nil
		})
	}

	// 5 goroutines → Lab B
	for i := 0; i < 5; i++ {
		i := i
		g.Go(func() error {
			client := &http.Client{CheckRedirect: noRedirect}
			label := fmt.Sprintf("pc-conc-b-%d", 200+i)
			serial := fmt.Sprintf("SN-CONC-B-%d", 200+i)
			body := url.Values{
				"_csrf":            {csrf},
				"label":            {label},
				"serial_number":    {serial},
				"operating_system": {"Win11"},
				"status":           {"normal"},
				"row":              {fmt.Sprintf("%d", i+1)},
				"column":           {"1"},
			}.Encode()
			req, err := http.NewRequestWithContext(ctx, "POST",
				env.TS.URL+"/vokasi/pc/create",
				strings.NewReader(body))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			for k, v := range cookies {
				req.AddCookie(&http.Cookie{Name: k, Value: v})
			}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusOK {
				return fmt.Errorf("PC %s: unexpected status %d", label, resp.StatusCode)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		t.Fatalf("concurrent PC creation failed: %v", err)
	}

	var countA, countB int
	env.DB_A.QueryRow("SELECT COUNT(*) FROM pcs WHERE label LIKE 'pc-conc-a-%'").Scan(&countA)
	env.DB_B.QueryRow("SELECT COUNT(*) FROM pcs WHERE label LIKE 'pc-conc-b-%'").Scan(&countB)
	if countA != 5 {
		t.Errorf("expected 5 new PCs in Lab A, got %d", countA)
	}
	if countB != 5 {
		t.Errorf("expected 5 new PCs in Lab B, got %d", countB)
	}
}

// TestConcurrentUserCreation — 10 goroutines creating users in the same lab.
func TestConcurrentUserCreation(t *testing.T) {
	env := wrapSharedEnv(t)

	cookies, csrf := loginAsAdmin(env)
	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(10)

	noRedirect := func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	for i := 0; i < 10; i++ {
		i := i
		g.Go(func() error {
			client := &http.Client{CheckRedirect: noRedirect}
			username := fmt.Sprintf("conc_user_%d", i)
			body := url.Values{
				"_csrf":     {csrf},
				"username":  {username},
				"password":  {"test123"},
				"full_name": {fmt.Sprintf("User %d", i)},
			}.Encode()
			req, err := http.NewRequestWithContext(ctx, "POST",
				env.TS.URL+"/labs/admin/users/create",
				strings.NewReader(body))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			for k, v := range cookies {
				req.AddCookie(&http.Cookie{Name: k, Value: v})
			}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusBadRequest {
				return fmt.Errorf("user %s: unexpected status %d", username, resp.StatusCode)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		t.Fatal("concurrent user creation failed:", err)
	}

	var count int
	env.GlobalDB.QueryRow("SELECT COUNT(*) FROM global_users WHERE username LIKE 'conc_user_%'").Scan(&count)
	t.Logf("Users created: %d (expected ≤10)", count)
}

// ——————————————————— Fase 3: Upload Folder Cleanup ————————————————————

// TestLabDelete_CleansUpUploadDir verifies that deleting a lab removes its upload directory.
func TestLabDelete_CleansUpUploadDir(t *testing.T) {
	env := wrapSharedEnv(t)

	// Set EnvPath — required by lab creation handler (writes .env)
	envFile := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(envFile, []byte("EXISTING_VAR=1\n"), 0644)
	savedEnvPath := env.Config.EnvPath
	env.Config.EnvPath = envFile

	loginAsAdmin(env)

	uniqueID := fmt.Sprintf("F3-%d", time.Now().UnixMilli())
	uniqueURL := fmt.Sprintf("f3-%d", time.Now().UnixMilli())

	// Create a new lab
	resp := adminPost(env, "/labs/create",
		fmt.Sprintf("id=%s&title=Test+Fase+3&url=%s&rows=1&cols=8", uniqueID, uniqueURL))
	resp.Body.Close()
	if resp.StatusCode != 302 {
		t.Fatalf("expected 302 when creating lab, got %d", resp.StatusCode)
	}

	// Get the upload dir path from config
	var uploadDir string
	for _, l := range env.Config.Labs {
		if l.URLPath == uniqueURL {
			uploadDir = l.UploadDir
			break
		}
	}
	if uploadDir == "" {
		env.Config.EnvPath = savedEnvPath
		t.Fatal("upload dir not found in config after lab creation")
	}

	// Register cleanup in case test fails mid-way
	t.Cleanup(func() {
		env.Config.EnvPath = savedEnvPath
		os.RemoveAll(uploadDir)
	})

	// Verify upload dir exists with subdirs
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		env.Config.EnvPath = savedEnvPath
		t.Fatalf("upload dir %s should exist after lab creation", uploadDir)
	}
	for _, sub := range []string{"pc", "device_types", "temp", "device_installations"} {
		subDir := filepath.Join(uploadDir, sub)
		if _, err := os.Stat(subDir); os.IsNotExist(err) {
			t.Errorf("upload subdir %s should exist after lab creation", subDir)
		}
	}

	// Delete the lab
	resp = adminPost(env, "/labs/"+uniqueURL+"/delete", "")
	resp.Body.Close()
	if resp.StatusCode != 302 {
		env.Config.EnvPath = savedEnvPath
		t.Fatalf("expected 302 when deleting lab, got %d", resp.StatusCode)
	}

	// Verify upload dir is removed
	if _, err := os.Stat(uploadDir); !os.IsNotExist(err) {
		t.Errorf("upload dir %s should be removed after lab delete, got err: %v", uploadDir, err)
	}
}

// ——————————————————— Fase 2: Lab Update Endpoint ——————————————————————

// TestLabEdit_UpdatesTitle verifies the lab edit page renders, title updates persist,
// and validation works.
func TestLabEdit_UpdatesTitle(t *testing.T) {
	env := wrapSharedEnv(t)

	// Set EnvPath — required by lab creation + edit handlers (writes .env)
	envFile := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(envFile, []byte("EXISTING_VAR=1\n"), 0644)
	savedEnvPath := env.Config.EnvPath
	env.Config.EnvPath = envFile

	loginAsAdmin(env)

	uniqueID := fmt.Sprintf("F2-%d", time.Now().UnixMilli())
	uniqueURL := fmt.Sprintf("f2-%d", time.Now().UnixMilli())

	// Create a new lab (adds LABS_<N>_* to .env)
	resp := adminPost(env, "/labs/create",
		fmt.Sprintf("id=%s&title=Before+Edit&url=%s&rows=1&cols=8", uniqueID, uniqueURL))
	resp.Body.Close()
	if resp.StatusCode != 302 {
		env.Config.EnvPath = savedEnvPath
		t.Fatalf("expected 302 when creating lab, got %d", resp.StatusCode)
	}

	// Find EnvIndex
	var labEnvIndex int
	for _, l := range env.Config.Labs {
		if l.URLPath == uniqueURL {
			labEnvIndex = l.EnvIndex
			break
		}
	}
	if labEnvIndex == 0 {
		env.Config.EnvPath = savedEnvPath
		t.Fatal("EnvIndex should be >0 after lab creation")
	}

	// Cleanup: restore EnvPath + delete lab (removes from config)
	t.Cleanup(func() {
		env.Config.EnvPath = savedEnvPath
		del := adminPost(env, "/labs/"+uniqueURL+"/delete", "")
		del.Body.Close()
	})

	// === 1. GET edit page → 200 with current title in form ===
	resp = adminGet(env, "/labs/"+uniqueURL+"/edit")
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 on GET edit page, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "Before Edit") {
		t.Error("edit page should contain current lab title in form")
	}
	if !strings.Contains(string(body), "Update") {
		t.Error("edit page should contain Update button")
	}
	if !strings.Contains(string(body), uniqueID) {
		t.Error("edit page should contain lab ID")
	}
	if !strings.Contains(string(body), uniqueURL) {
		t.Error("edit page should contain lab URL path")
	}

	// === 2. POST new title → 302 ===
	newTitle := "After Edit"
	resp = adminPost(env, "/labs/"+uniqueURL+"/edit", "title="+url.QueryEscape(newTitle))
	resp.Body.Close()
	if resp.StatusCode != 302 {
		t.Fatalf("expected 302 on POST edit, got %d", resp.StatusCode)
	}

	// Verify config updated
	var updatedTitle string
	for _, l := range env.Config.Labs {
		if l.URLPath == uniqueURL {
			updatedTitle = l.Title
			break
		}
	}
	if updatedTitle != newTitle {
		t.Errorf("expected title %q in config, got %q", newTitle, updatedTitle)
	}

	// Verify .env file updated
	envData, _ := os.ReadFile(envFile)
	expectedLine := fmt.Sprintf("LABS_%d_TITLE=%s", labEnvIndex, newTitle)
	if !strings.Contains(string(envData), expectedLine) {
		t.Errorf(".env should contain %q", expectedLine)
	}
	// Verify old title no longer in .env
	if strings.Contains(string(envData), "Before Edit") {
		t.Error(".env should NOT contain old title")
	}

	// === 3. POST empty title → 400 ===
	resp = adminPost(env, "/labs/"+uniqueURL+"/edit", "title=")
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("expected 400 for empty title, got %d", resp.StatusCode)
	}
}

// Ensure unused import suppression — these are used in test code above.
var _ = io.Discard
var _ = json.Marshal
