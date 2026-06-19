package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"testing"

	"inventaris-lab-kom/internal/database"
)

// loginAndRefresh logs in as the given user and refreshes CSRF token from authenticated page.
func loginAndRefresh(lab *testLab, username, password string) bool {
	if !lab.login(username, password) {
		return false
	}
	return lab.refreshCSRF()
}

// ============================================
// TestPC — list, detail, create, edit, delete + fail
// ============================================

func TestPC(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	db := env.DB_A
	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	t.Run("list_pcs", func(t *testing.T) {
		resp, err := lab.get("/pc")
		if err != nil {
			t.Fatalf("GET /pc: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("detail_existing_pc", func(t *testing.T) {
		resp, err := lab.get("/pc/pc-1")
		if err != nil {
			t.Fatalf("GET /pc/pc-1: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("edit_page_existing_pc", func(t *testing.T) {
		resp, err := lab.get("/pc/pc-1/edit")
		if err != nil {
			t.Fatalf("GET /pc/pc-1/edit: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("create_pc", func(t *testing.T) {
		data := url.Values{
			"row": {"5"}, "column": {"8"},
			"status": {"normal"}, "placement": {"dipakai"},
			"is_mahasiswa": {"true"},
			"serial_number": {"SN-TEST-PC-001"},
			"operating_system": {"Win11"}, "pc_type": {"PC"},
			"brand_model": {"Dell"}, "accessories": {"KB"},
			"processor": {"i7"}, "ram": {"16GB"}, "storage": {"512GB"},
		}.Encode()
		resp, err := lab.post("/pc/create", data)
		if err != nil {
			t.Fatalf("POST /pc/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var newID int
		db.QueryRow("SELECT id FROM pcs WHERE serial_number='SN-TEST-PC-001'").Scan(&newID)
		if newID == 0 {
			t.Error("PC not found in DB after create")
		}
	})

	t.Run("edit_pc", func(t *testing.T) {
		resp, err := lab.post("/pc/pc-1/edit",
			"status=warning&placement=dipakai&serial_number=SN001-UPDATED&operating_system=Win11&pc_type=PC&brand_model=Dell&accessories=KB&processor=i7&ram=16GB&storage=512GB&notes=Updated")
		if err != nil {
			t.Fatalf("POST /pc/pc-1/edit: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var status string
		db.QueryRow("SELECT status FROM pcs WHERE label='pc-1'").Scan(&status)
		if status != "warning" {
			t.Errorf("expected status 'warning', got %q", status)
		}
	})

	t.Run("fail_create_missing_fields", func(t *testing.T) {
		resp, err := lab.post("/pc/create", "")
		if err != nil {
			t.Fatalf("POST /pc/create empty: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for missing fields, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_detail_not_found", func(t *testing.T) {
		resp, err := lab.get("/pc/nonexistent-pc-xyz")
		if err != nil {
			t.Fatalf("GET /pc/nonexistent-pc-xyz: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 500 && resp.StatusCode != 404 {
			t.Errorf("expected 500 or 404, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_batch_delete_empty", func(t *testing.T) {
		resp, err := lab.postJSON("/pc/batch-delete", `{"ids":[]}`)
		if err != nil {
			t.Fatalf("POST /pc/batch-delete empty: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for empty batch, got %d", resp.StatusCode)
		}
	})

	t.Run("delete_pc", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		data := url.Values{
			"row": {"6"}, "column": {"1"},
			"status": {"normal"}, "placement": {"dipakai"},
			"is_mahasiswa": {"true"},
			"serial_number": {"SN-TO-DELETE"},
			"operating_system": {"Win11"}, "pc_type": {"PC"},
			"brand_model": {"Dell"}, "accessories": {"KB"},
			"processor": {"i5"}, "ram": {"8GB"}, "storage": {"256GB"},
		}.Encode()
		resp, err := lab.post("/pc/create", data)
		if err != nil {
			t.Fatalf("POST /pc/create: %v", err)
		}
		defer resp.Body.Close()
		var delLabel string
		db.QueryRow("SELECT label FROM pcs WHERE serial_number='SN-TO-DELETE'").Scan(&delLabel)
		if delLabel == "" {
			t.Fatal("PC to delete not found after create")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err = lab.post("/pc/"+delLabel+"/delete", "")
		if err != nil {
			t.Fatalf("POST /pc/%s/delete: %v", delLabel, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var count int
		db.QueryRow("SELECT COUNT(*) FROM pcs WHERE label=?", delLabel).Scan(&count)
		if count != 0 {
			t.Errorf("expected PC deleted, count=%d", count)
		}
	})
}

// ============================================
// TestSoftware — list, create, edit, delete, export + fail
// ============================================

func TestSoftware(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	db := env.DB_A
	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	t.Run("list_software", func(t *testing.T) {
		resp, err := lab.get("/software")
		if err != nil {
			t.Fatalf("GET /software: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("create_software", func(t *testing.T) {
		resp, err := lab.post("/software/create", "name=TestSW&category=other&description=Test+software")
		if err != nil {
			t.Fatalf("POST /software/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var swID int
		db.QueryRow("SELECT id FROM software_catalog WHERE name='Testsw'").Scan(&swID)
		if swID == 0 {
			t.Error("Software not found after create")
		}
	})

	t.Run("edit_software", func(t *testing.T) {
		var swSlug string
		db.QueryRow("SELECT slug FROM software_catalog ORDER BY id LIMIT 1").Scan(&swSlug)
		if swSlug == "" {
			t.Skip("no software to edit")
		}
		resp, err := lab.post("/software/"+swSlug+"/edit", "name=SWUpdated&category=required&description=Updated")
		if err != nil {
			t.Fatalf("POST /software/%s/edit: %v", swSlug, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_create_empty_name", func(t *testing.T) {
		resp, err := lab.post("/software/create", "name=&category=other&description=")
		if err != nil {
			t.Fatalf("POST /software/create empty name: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 && resp.StatusCode != 400 {
			t.Errorf("expected 302 or 400 for empty name, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_create_duplicate_name", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		lab.post("/software/create", "name=UniqueSW&category=other&description=First")
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/software/create", "name=UniqueSW&category=other&description=Duplicate")
		if err != nil {
			t.Fatalf("POST /software/create duplicate: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 && resp.StatusCode != 400 {
			t.Errorf("expected 302 or 400 for duplicate, got %d", resp.StatusCode)
		}
	})

	t.Run("delete_software", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/software/create", "name=DeleteMeSW&category=other&description=To+delete")
		if err != nil {
			t.Fatalf("POST /software/create: %v", err)
		}
		defer resp.Body.Close()
		var swID int
		var swSlug string
		db.QueryRow("SELECT id, slug FROM software_catalog WHERE name='Deletemesw'").Scan(&swID, &swSlug)
		if swID == 0 {
			t.Fatal("Software to delete not found")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err = lab.post("/software/"+swSlug+"/delete", "")
		if err != nil {
			t.Fatalf("POST /software/%s/delete: %v", swSlug, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		db.QueryRow("SELECT COUNT(*) FROM software_catalog WHERE id=?", swID).Scan(&swID)
		if swID != 0 {
			t.Errorf("expected deleted, id=%d", swID)
		}
	})

	t.Run("export_software", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.get("/software/export")
		if err != nil {
			t.Fatalf("GET /software/export: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
			t.Errorf("expected excel content type, got %q", ct)
		}
	})
}

// ============================================
// TestSchedule — list, create, edit, delete + fail
// ============================================

func TestSchedule(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	db := env.DB_A
	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	t.Run("list_schedules", func(t *testing.T) {
		resp, err := lab.get("/schedules")
		if err != nil {
			t.Fatalf("GET /schedules: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("create_schedule", func(t *testing.T) {
		resp, err := lab.post("/schedules/create",
			"course_name=TestAlgo&lecturer=Dr.T&day=Senin&class=IF-1&time_start=08:00&time_end=09:40")
		if err != nil {
			t.Fatalf("POST /schedules/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		db.Flush()
		var count int
		db.QueryRow("SELECT COUNT(*) FROM course_schedules").Scan(&count)
		if count < 1 {
			t.Fatal("no schedules in DB")
		}
		var scID int
		db.QueryRow("SELECT id FROM course_schedules ORDER BY id DESC LIMIT 1").Scan(&scID)
		if scID == 0 {
			t.Error("Schedule not found after create")
		}
	})

	t.Run("edit_schedule", func(t *testing.T) {
		var scID int
		db.QueryRow("SELECT id FROM course_schedules ORDER BY id LIMIT 1").Scan(&scID)
		if scID == 0 {
			t.Skip("no schedule to edit")
		}
		resp, err := lab.post("/schedules/"+fmt.Sprint(scID)+"/edit",
			"course_name=AlgoUpdated&lecturer=Dr.T&day=Senin&class=IF-1&time_start=08:00&time_end=09:40")
		if err != nil {
			t.Fatalf("POST /schedules/%d/edit: %v", scID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
	})

	t.Run("delete_schedule", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/schedules/create",
			"course_name=DeleteMeSched&lecturer=Dr.X&day=Selasa&class=IF-2&time_start=10:00&time_end=11:40")
		if err != nil {
			t.Fatalf("POST /schedules/create: %v", err)
		}
		defer resp.Body.Close()
		db.Flush()
		var scID int
		db.QueryRow("SELECT id FROM course_schedules ORDER BY id DESC LIMIT 1").Scan(&scID)
		if scID == 0 {
			t.Fatal("Schedule to delete not found")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err = lab.post("/schedules/"+fmt.Sprint(scID)+"/delete", "")
		if err != nil {
			t.Fatalf("POST /schedules/%d/delete: %v", scID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var count int
		db.QueryRow("SELECT COUNT(*) FROM course_schedules WHERE id=?", scID).Scan(&count)
		if count != 0 {
			t.Errorf("expected deleted, count=%d", count)
		}
	})

	t.Run("fail_create_empty_fields", func(t *testing.T) {
		resp, err := lab.post("/schedules/create",
			"course_name=&lecturer=&day=&class=&time_start=&time_end=")
		if err != nil {
			t.Fatalf("POST /schedules/create empty: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 && resp.StatusCode != 400 {
			t.Errorf("expected 302 or 400, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_edit_not_found", func(t *testing.T) {
		resp, err := lab.post("/schedules/99999/edit",
			"course_name=NotFound&lecturer=None&day=Senin&class=X&time_start=08:00&time_end=09:40")
		if err != nil {
			t.Fatalf("POST /schedules/99999/edit: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 500 && resp.StatusCode != 404 && resp.StatusCode != 302 {
			t.Errorf("expected 500, 404, or 302, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// TestDevice — list, create, batch, edit, detail, delete + fail
// ============================================

func TestDevice(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	db := env.DB_A
	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	// Seed category + device type
	db.Exec("INSERT OR IGNORE INTO categories (id, name, default_prefix) VALUES (1, 'Pentab', 'PENTAB')")
	db.Exec("INSERT OR IGNORE INTO device_types (id, category_id, name, brand, model, asset_code_prefix, usage_type, default_location) VALUES (1, 1, 'Pentab', 'Wacom', 'One', 'PENTAB', 'loanable', 'Lab')")

	t.Run("list_devices", func(t *testing.T) {
		resp, err := lab.get("/devices")
		if err != nil {
			t.Fatalf("GET /devices: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("create_device", func(t *testing.T) {
		resp, err := lab.post("/devices/create",
			"device_type_id=1&serial_number=SN-DEV-001&condition=normal&location=Lab&purchase_date=&notes=Test+device")
		if err != nil {
			t.Fatalf("POST /devices/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var devID int
		db.QueryRow("SELECT id FROM devices WHERE serial_number='SN-DEV-001'").Scan(&devID)
		if devID == 0 {
			t.Error("Device not found after create")
		}
	})

	t.Run("detail_device", func(t *testing.T) {
		var devAssetCode, catPrefix, typePrefix string
		db.QueryRow(`SELECT d.asset_code, COALESCE(c.default_prefix,''), COALESCE(dt.asset_code_prefix,'')
			FROM devices d
			JOIN device_types dt ON dt.id = d.device_type_id
			JOIN categories c ON c.id = dt.category_id
			WHERE d.serial_number='SN-DEV-001'`).Scan(&devAssetCode, &catPrefix, &typePrefix)
		if devAssetCode == "" {
			t.Skip("device not found for detail")
		}
		devSlug := strings.ToLower(devAssetCode)
		catSlug := strings.ToLower(catPrefix)
		typeSlug := strings.ToLower(typePrefix)
		nestedURL := "/devices/" + catSlug + "/" + typeSlug + "/" + devSlug
		resp, err := lab.get(nestedURL)
		if err != nil {
			t.Fatalf("GET %s: %v", nestedURL, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("edit_device", func(t *testing.T) {
		var devAssetCode string
		db.QueryRow("SELECT asset_code FROM devices WHERE serial_number='SN-DEV-001'").Scan(&devAssetCode)
		if devAssetCode == "" {
			t.Skip("device not found for edit")
		}
		devSlug := strings.ToLower(devAssetCode)
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/devices/"+devSlug+"/edit",
			"device_type_id=1&asset_code="+devAssetCode+"&serial_number=SN-DEV-002&condition=rusak&location=Lab2&purchase_date=&notes=Updated")
		if err != nil {
			t.Fatalf("POST /devices/%s/edit: %v", devSlug, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		db.Flush()
		var serial string
		db.QueryRow("SELECT serial_number FROM devices WHERE asset_code=?", devAssetCode).Scan(&serial)
		if serial != "SN-DEV-002" {
			t.Errorf("expected serial SN-DEV-002, got %q", serial)
		}
	})

	t.Run("batch_create_devices", func(t *testing.T) {
		var dtID int
		db.QueryRow("SELECT id FROM device_types ORDER BY id LIMIT 1").Scan(&dtID)
		if dtID == 0 {
			t.Skip("no device type")
		}
		body := fmt.Sprintf(`{"device_type_id":%d,"devices":[{"serial_number":"SN-BATCH1","condition":"normal","location":"Lab"},{"serial_number":"SN-BATCH2","condition":"rusak","location":"Lab"}]}`, dtID)
		resp, err := lab.postJSON("/devices/batch-create", body)
		if err != nil {
			t.Fatalf("POST /devices/batch-create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var br struct {
			Success bool     `json:"success"`
			Codes   []string `json:"codes"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&br); err != nil {
			t.Fatalf("decode batch response: %v", err)
		}
		if !br.Success {
			t.Error("batch create success=false")
		}
		if len(br.Codes) != 2 {
			t.Errorf("expected 2 codes, got %d", len(br.Codes))
		}
	})

	t.Run("delete_device", func(t *testing.T) {
		var devAssetCode string
		db.QueryRow("SELECT asset_code FROM devices WHERE serial_number='SN-DEV-002'").Scan(&devAssetCode)
		if devAssetCode == "" {
			t.Skip("device not found for delete")
		}
		devSlug := strings.ToLower(devAssetCode)
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/devices/"+devSlug+"/delete", "")
		if err != nil {
			t.Fatalf("POST /devices/%s/delete: %v", devSlug, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var count int
		db.QueryRow("SELECT COUNT(*) FROM devices WHERE asset_code=?", devAssetCode).Scan(&count)
		if count != 0 {
			t.Errorf("expected deleted, count=%d", count)
		}
	})

	t.Run("fail_create_missing_type", func(t *testing.T) {
		resp, err := lab.post("/devices/create",
			"device_type_id=&serial_number=SN-NO-TYPE&condition=normal&location=Lab&notes=")
		if err != nil {
			t.Fatalf("POST /devices/create no type: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for missing type, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_detail_not_found", func(t *testing.T) {
		resp, err := lab.get("/devices/cat/type/nonexistent-device")
		if err != nil {
			t.Fatalf("GET /devices/cat/type/nonexistent-device: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 500 && resp.StatusCode != 404 {
			t.Errorf("expected 500 or 404, got %d", resp.StatusCode)
		}
	})
}

// seedDevice creates a category, device_type, and device via POST handler for testing.
// Returns the device asset_code slug.
func seedDeviceViaHandler(lab *testLab, db *database.DB, serial, catName, catPrefix, dtName, dtBrand, dtModel, dtPrefix string) string {
	db.Exec("INSERT OR IGNORE INTO categories (id, name, default_prefix) VALUES (1, ?, ?)", catName, catPrefix)
	db.Exec("INSERT OR IGNORE INTO device_types (id, category_id, name, brand, model, asset_code_prefix, usage_type, default_location) VALUES (1, 1, ?, ?, ?, ?, 'loanable', 'Lab')", dtName, dtBrand, dtModel, dtPrefix)
	if !lab.refreshCSRF() {
		return ""
	}
	lab.post("/devices/create",
		fmt.Sprintf("device_type_id=1&serial_number=%s&condition=normal&location=Lab&purchase_date=&notes=", serial))
	var assetCode string
	db.QueryRow("SELECT asset_code FROM devices WHERE serial_number=?", serial).Scan(&assetCode)
	return strings.ToLower(assetCode)
}

// ============================================
// TestDeviceLoan — create, edit, detail, extend, delete + fail
// ============================================

func TestDeviceLoan(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	db := env.DB_A
	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	assetSlug := seedDeviceViaHandler(lab, db, "SN-LOAN-DEV", "Monitor", "MONITOR", "LCD", "Dell", "22in", "MONITOR")
	if assetSlug == "" {
		t.Fatal("failed to seed device for loan")
	}

	t.Run("create_loan", func(t *testing.T) {
		var devID int
		db.QueryRow("SELECT id FROM devices ORDER BY id LIMIT 1").Scan(&devID)
		if devID == 0 {
			t.Fatal("no device for loan")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/device-loans/create",
			fmt.Sprintf("device_id=%d&borrower_name=Mahasiswa+Test&borrower_type=mahasiswa&loan_date=2026-06-01&return_date=2026-06-05&purpose=Praktikum", devID))
		if err != nil {
			t.Fatalf("POST /device-loans/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var loanCount int
		db.QueryRow("SELECT COUNT(*) FROM device_loans").Scan(&loanCount)
		if loanCount == 0 {
			t.Error("Loan not created")
		}
	})

	t.Run("detail_loan", func(t *testing.T) {
		var loanID int
		db.QueryRow("SELECT id FROM device_loans ORDER BY id DESC LIMIT 1").Scan(&loanID)
		if loanID == 0 {
			t.Skip("no loan for detail")
		}
		resp, err := lab.get("/device-loans/" + fmt.Sprint(loanID))
		if err != nil {
			t.Fatalf("GET /device-loans/%d: %v", loanID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("edit_loan", func(t *testing.T) {
		var loanID int
		db.QueryRow("SELECT id FROM device_loans ORDER BY id DESC LIMIT 1").Scan(&loanID)
		if loanID == 0 {
			t.Skip("no loan for edit")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/device-loans/"+fmt.Sprint(loanID)+"/edit",
			"borrower_name=Mahasiswa+Test&borrower_type=mahasiswa&loan_date=2026-06-01&purpose=Praktikum&status=returned&actual_return_date=2026-06-03&notes=Returned+early")
		if err != nil {
			t.Fatalf("POST /device-loans/%d/edit: %v", loanID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var retDate string
		db.QueryRow("SELECT COALESCE(actual_return_date,'') FROM device_loans WHERE id=?", loanID).Scan(&retDate)
		if retDate == "" {
			t.Error("actual_return_date not saved")
		}
	})

	t.Run("fail_create_missing_fields", func(t *testing.T) {
		resp, err := lab.post("/device-loans/create",
			"device_id=&borrower_name=&borrower_type=&loan_date=&return_date=&purpose=")
		if err != nil {
			t.Fatalf("POST /device-loans/create empty: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 && resp.StatusCode != 400 {
			t.Errorf("expected 302 or 400, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_detail_not_found", func(t *testing.T) {
		resp, err := lab.get("/device-loans/99999")
		if err != nil {
			t.Fatalf("GET /device-loans/99999: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 500 && resp.StatusCode != 404 {
			t.Errorf("expected 500 or 404, got %d", resp.StatusCode)
		}
	})

	t.Run("extend_loan", func(t *testing.T) {
		var devID int
		db.QueryRow("SELECT id FROM devices ORDER BY id LIMIT 1").Scan(&devID)
		if devID == 0 {
			t.Skip("no device")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		lab.post("/device-loans/create",
			fmt.Sprintf("device_id=%d&borrower_name=Extend+Test&borrower_type=mahasiswa&loan_date=2026-06-01&return_date=2026-06-05&purpose=Test", devID))
		var loanID int
		db.QueryRow("SELECT id FROM device_loans ORDER BY id DESC LIMIT 1").Scan(&loanID)
		if loanID == 0 {
			t.Fatal("loan not created for extend")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/device-loans/"+fmt.Sprint(loanID)+"/extend", "return_date=2026-06-10")
		if err != nil {
			t.Fatalf("POST /device-loans/%d/extend: %v", loanID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 && resp.StatusCode != 200 {
			t.Errorf("expected 302 or 200, got %d", resp.StatusCode)
		}
	})

	t.Run("delete_loan", func(t *testing.T) {
		var loanID int
		db.QueryRow("SELECT id FROM device_loans ORDER BY id DESC LIMIT 1").Scan(&loanID)
		if loanID == 0 {
			t.Skip("no loan to delete")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/device-loans/"+fmt.Sprint(loanID)+"/delete", "")
		if err != nil {
			t.Fatalf("POST /device-loans/%d/delete: %v", loanID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var count int
		db.QueryRow("SELECT COUNT(*) FROM device_loans WHERE id=?", loanID).Scan(&count)
		if count != 0 {
			t.Errorf("expected deleted, count=%d", count)
		}
	})
}

// ============================================
// TestDeviceUsage — create, edit, detail, delete + fail
// ============================================

func TestDeviceUsage(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	db := env.DB_A
	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	assetSlug := seedDeviceViaHandler(lab, db, "SN-USAGE-DEV", "Tablet", "TAB", "Tablet", "Samsung", "Tab", "TAB")
	if assetSlug == "" {
		t.Fatal("failed to seed device for usage")
	}

	t.Run("create_usage", func(t *testing.T) {
		var devID int
		db.QueryRow("SELECT id FROM devices ORDER BY id LIMIT 1").Scan(&devID)
		if devID == 0 {
			t.Fatal("no device for usage")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/device-usages/create",
			fmt.Sprintf("device_id=%d&user_name=Dosen+Test&user_type=dosen&usage_date=2026-06-01&is_available=yes&purpose=Demo", devID))
		if err != nil {
			t.Fatalf("POST /device-usages/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var usageCount int
		db.QueryRow("SELECT COUNT(*) FROM device_usages").Scan(&usageCount)
		if usageCount == 0 {
			t.Error("Usage not created")
		}
	})

	t.Run("detail_usage", func(t *testing.T) {
		var usageID int
		db.QueryRow("SELECT id FROM device_usages ORDER BY id DESC LIMIT 1").Scan(&usageID)
		if usageID == 0 {
			t.Skip("no usage for detail")
		}
		resp, err := lab.get("/device-usages/" + fmt.Sprint(usageID))
		if err != nil {
			t.Fatalf("GET /device-usages/%d: %v", usageID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("edit_usage", func(t *testing.T) {
		var usageID int
		db.QueryRow("SELECT id FROM device_usages ORDER BY id DESC LIMIT 1").Scan(&usageID)
		if usageID == 0 {
			t.Skip("no usage for edit")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/device-usages/"+fmt.Sprint(usageID)+"/edit",
			"user_name=Dosen+Updated&user_type=dosen&usage_date=2026-06-01&is_available=yes&purpose=Demo+updated")
		if err != nil {
			t.Fatalf("POST /device-usages/%d/edit: %v", usageID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var userName string
		db.QueryRow("SELECT user_name FROM device_usages WHERE id=?", usageID).Scan(&userName)
		if userName != "Dosen Updated" {
			t.Errorf("expected 'Dosen Updated', got %q", userName)
		}
	})

	t.Run("delete_usage", func(t *testing.T) {
		var usageID int
		db.QueryRow("SELECT id FROM device_usages ORDER BY id DESC LIMIT 1").Scan(&usageID)
		if usageID == 0 {
			t.Skip("no usage to delete")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/device-usages/"+fmt.Sprint(usageID)+"/delete", "")
		if err != nil {
			t.Fatalf("POST /device-usages/%d/delete: %v", usageID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var count int
		db.QueryRow("SELECT COUNT(*) FROM device_usages WHERE id=?", usageID).Scan(&count)
		if count != 0 {
			t.Errorf("expected deleted, count=%d", count)
		}
	})

	t.Run("fail_create_missing_fields", func(t *testing.T) {
		resp, err := lab.post("/device-usages/create",
			"device_id=&user_name=&user_type=&usage_date=&is_available=&purpose=")
		if err != nil {
			t.Fatalf("POST /device-usages/create empty: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 && resp.StatusCode != 400 {
			t.Errorf("expected 302 or 400, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_detail_not_found", func(t *testing.T) {
		resp, err := lab.get("/device-usages/99999")
		if err != nil {
			t.Fatalf("GET /device-usages/99999: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 500 && resp.StatusCode != 404 {
			t.Errorf("expected 500 or 404, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// TestInstallation — create, edit, detail, delete + fail
// ============================================

func TestInstallation(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	db := env.DB_A
	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	assetSlug := seedDeviceViaHandler(lab, db, "SN-INST-DEV", "Server", "SRV", "ProLiant", "HP", "ML350", "SRV")
	if assetSlug == "" {
		t.Fatal("failed to seed device for installation")
	}

	t.Run("create_installation", func(t *testing.T) {
		var devID int
		db.QueryRow("SELECT id FROM devices ORDER BY id LIMIT 1").Scan(&devID)
		if devID == 0 {
			t.Fatal("no device for installation")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/installations/create",
			fmt.Sprintf("device_id=%d&location_installed=Lab+Utama&installation_start_date=2026-06-01&notes=Installed+test", devID))
		if err != nil {
			t.Fatalf("POST /installations/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var installCount int
		db.QueryRow("SELECT COUNT(*) FROM device_installations").Scan(&installCount)
		if installCount == 0 {
			t.Error("Installation not created")
		}
	})

	t.Run("detail_installation", func(t *testing.T) {
		var installID int
		db.QueryRow("SELECT id FROM device_installations ORDER BY id DESC LIMIT 1").Scan(&installID)
		if installID == 0 {
			t.Skip("no installation for detail")
		}
		resp, err := lab.get("/installations/" + fmt.Sprint(installID))
		if err != nil {
			t.Fatalf("GET /installations/%d: %v", installID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("edit_installation", func(t *testing.T) {
		var installID int
		db.QueryRow("SELECT id FROM device_installations ORDER BY id DESC LIMIT 1").Scan(&installID)
		if installID == 0 {
			t.Skip("no installation for edit")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/installations/"+fmt.Sprint(installID)+"/edit",
			"location_installed=Lab+Cadangan&installation_start_date=2026-06-01&installation_finish_date=2026-06-10&notes=Updated")
		if err != nil {
			t.Fatalf("POST /installations/%d/edit: %v", installID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var loc string
		db.QueryRow("SELECT location_installed FROM device_installations WHERE id=?", installID).Scan(&loc)
		if loc != "Lab Cadangan" {
			t.Errorf("expected 'Lab Cadangan', got %q", loc)
		}
	})

	t.Run("delete_installation", func(t *testing.T) {
		var installID int
		db.QueryRow("SELECT id FROM device_installations ORDER BY id DESC LIMIT 1").Scan(&installID)
		if installID == 0 {
			t.Skip("no installation to delete")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/installations/"+fmt.Sprint(installID)+"/delete", "")
		if err != nil {
			t.Fatalf("POST /installations/%d/delete: %v", installID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var count int
		db.QueryRow("SELECT COUNT(*) FROM device_installations WHERE id=?", installID).Scan(&count)
		if count != 0 {
			t.Errorf("expected deleted, count=%d", count)
		}
	})

	t.Run("fail_create_missing_location", func(t *testing.T) {
		var devID int
		db.QueryRow("SELECT id FROM devices ORDER BY id LIMIT 1").Scan(&devID)
		if devID == 0 {
			t.Skip("no device")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/installations/create",
			fmt.Sprintf("device_id=%d&location_installed=&installation_start_date=&notes=", devID))
		if err != nil {
			t.Fatalf("POST /installations/create empty: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 && resp.StatusCode != 400 {
			t.Errorf("expected 302 or 400, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_detail_not_found", func(t *testing.T) {
		resp, err := lab.get("/installations/99999")
		if err != nil {
			t.Fatalf("GET /installations/99999: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 500 && resp.StatusCode != 404 {
			t.Errorf("expected 500 or 404, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// TestLogbook — list, create, edit, save, upload, delete + fail
// ============================================

func TestLogbook(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	db := env.DB_A
	cfg := env.Config
	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	t.Run("list_logbook", func(t *testing.T) {
		resp, err := lab.get("/logbook")
		if err != nil {
			t.Fatalf("GET /logbook: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("create_logbook", func(t *testing.T) {
		resp, err := lab.post("/logbook/create",
			"date=2026-06-01&student_name=Mahasiswa+Test&nim=24091234567&time_in=08:00&time_out=09:40&purpose=Praktikum")
		if err != nil {
			t.Fatalf("POST /logbook/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var lbCount int
		db.QueryRow("SELECT COUNT(*) FROM logbook_entries").Scan(&lbCount)
		if lbCount == 0 {
			t.Error("Logbook entry not created")
		}
	})

	t.Run("edit_logbook", func(t *testing.T) {
		var lbID int
		db.QueryRow("SELECT id FROM logbook_entries ORDER BY id DESC LIMIT 1").Scan(&lbID)
		if lbID == 0 {
			t.Skip("no logbook entry for edit")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/logbook/"+fmt.Sprint(lbID)+"/edit",
			"date=2026-06-01&student_name=Mahasiswa+Updated&nim=24091234567&time_in=08:00&time_out=10:00&purpose=Praktikum+updated")
		if err != nil {
			t.Fatalf("POST /logbook/%d/edit: %v", lbID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_create_invalid_nim", func(t *testing.T) {
		resp, err := lab.post("/logbook/create",
			"date=2026-06-01&student_name=Test&nim=123&time_in=08:00&time_out=09:00&purpose=Test")
		if err != nil {
			t.Fatalf("POST /logbook/create invalid NIM: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 && resp.StatusCode != 400 {
			t.Errorf("expected 302 or 400 for invalid NIM, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_create_duplicate", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		lab.post("/logbook/create",
			"date=2026-06-02&student_name=Duplicate+Test&nim=24099999999&time_in=08:00&time_out=09:00&purpose=Test")
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/logbook/create",
			"date=2026-06-02&student_name=Duplicate+Test&nim=24099999999&time_in=08:00&time_out=09:00&purpose=Test")
		if err != nil {
			t.Fatalf("POST /logbook/create duplicate: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 && resp.StatusCode != 400 && resp.StatusCode != 409 {
			t.Errorf("expected 302/400/409 for duplicate, got %d", resp.StatusCode)
		}
	})

	t.Run("delete_logbook", func(t *testing.T) {
		var lbID int
		db.QueryRow("SELECT id FROM logbook_entries ORDER BY id DESC LIMIT 1").Scan(&lbID)
		if lbID == 0 {
			t.Skip("no logbook entry to delete")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/logbook/"+fmt.Sprint(lbID)+"/delete", "")
		if err != nil {
			t.Fatalf("POST /logbook/%d/delete: %v", lbID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var count int
		db.QueryRow("SELECT COUNT(*) FROM logbook_entries WHERE id=?", lbID).Scan(&count)
		if count != 0 {
			t.Errorf("expected deleted, count=%d", count)
		}
	})

	t.Run("fail_upload_no_api_key", func(t *testing.T) {
		if cfg.GeminiAPIKey != "" || cfg.OpenRouterAPIKey != "" {
			t.Skip("API key present, skipping no-api-key test")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.get("/logbook/upload")
		if err != nil {
			t.Fatalf("GET /logbook/upload: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("logbook_save_json", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/logbook/save",
			"source_file=test&date[]=2026-06-03&student_name[]=Mhs+Save&nim[]=24091111111&time_in[]=10:00&time_out[]=11:40&purpose[]=Praktikum")
		if err != nil {
			t.Fatalf("POST /logbook/save: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var sr struct {
			Success bool `json:"success"`
			Saved   int  `json:"saved"`
		}
		if err := json.Unmarshal(body, &sr); err != nil {
			t.Fatalf("decode save response: %v", err)
		}
		if !sr.Success {
			t.Errorf("save success=false, response: %s", string(body))
		}
		if sr.Saved != 1 {
			t.Errorf("expected 1 saved, got %d", sr.Saved)
		}
	})
}
