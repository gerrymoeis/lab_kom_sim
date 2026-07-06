package tests

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"inventaris-lab-kom/internal/database"
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
