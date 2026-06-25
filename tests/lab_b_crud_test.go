package tests

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func itoa(n int) string {
	return strconv.Itoa(n)
}

func TestLabB_EmptyState(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabB

	if !loginAndRefresh(lab, "labB_only", "test123") {
		t.Fatal("login failed")
	}

	t.Run("pc_list_empty", func(t *testing.T) {
		resp, err := lab.get("/pc")
		if err != nil {
			t.Fatalf("GET /pc: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("software_list_empty", func(t *testing.T) {
		resp, err := lab.get("/software")
		if err != nil {
			t.Fatalf("GET /software: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("schedules_list_empty", func(t *testing.T) {
		resp, err := lab.get("/schedules")
		if err != nil {
			t.Fatalf("GET /schedules: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestLabB_PC_CreateFromScratch(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabB
	db := env.DB_B

	if !loginAndRefresh(lab, "labB_only", "test123") {
		t.Fatal("login failed")
	}

	t.Run("create_pc", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		data := url.Values{
			"row": {"1"}, "column": {"1"},
			"status": {"normal"}, "placement": {"dipakai"},
			"is_mahasiswa": {"true"},
			"serial_number": {"SN-LABB-PC-001"},
			"operating_system": {"Win11"}, "pc_type": {"PC"},
			"brand_model": {"Dell"}, "accessories": {"KB"},
			"processor": {"i5"}, "ram": {"8GB"}, "storage": {"256GB"},
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
		db.QueryRow("SELECT id FROM pcs WHERE serial_number='SN-LABB-PC-001'").Scan(&newID)
		if newID == 0 {
			t.Error("PC not found in DB after create")
		}
	})

	t.Run("list_after_create", func(t *testing.T) {
		resp, err := lab.get("/pc")
		if err != nil {
			t.Fatalf("GET /pc: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("edit_pc", func(t *testing.T) {
		var label string
		db.QueryRow("SELECT label FROM pcs WHERE serial_number='SN-LABB-PC-001'").Scan(&label)
		if label == "" {
			t.Skip("pc not found")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/pc/"+label+"/edit",
			"status=warning&placement=dipakai&serial_number=SN-LABB-PC-001&operating_system=Win11&pc_type=PC&brand_model=Dell&accessories=KB&processor=i5&ram=8GB&storage=256GB&notes=Edited")
		if err != nil {
			t.Fatalf("POST /pc/%s/edit: %v", label, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var status string
		db.QueryRow("SELECT status FROM pcs WHERE label=?", label).Scan(&status)
		if status != "warning" {
			t.Errorf("expected status 'warning', got %q", status)
		}
	})

	t.Run("delete_pc", func(t *testing.T) {
		var label string
		db.QueryRow("SELECT label FROM pcs WHERE serial_number='SN-LABB-PC-001'").Scan(&label)
		if label == "" {
			t.Skip("pc not found")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/pc/"+label+"/delete", "")
		if err != nil {
			t.Fatalf("POST /pc/%s/delete: %v", label, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var count int
		db.QueryRow("SELECT COUNT(*) FROM pcs WHERE label=?", label).Scan(&count)
		if count != 0 {
			t.Errorf("expected deleted, count=%d", count)
		}
	})
}

func TestLabB_Software_CreateFromScratch(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabB
	db := env.DB_B

	if !loginAndRefresh(lab, "labB_only", "test123") {
		t.Fatal("login failed")
	}

	// Capture slug from create for subsequent subtests
	var swSlug string

	t.Run("create_software", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/software/create", "name=LabB+Software")
		if err != nil {
			t.Fatalf("POST /software/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		db.Flush()
		var swID int
		db.QueryRow("SELECT id FROM software_catalog WHERE slug='labb-software'").Scan(&swID)
		if swID == 0 {
			t.Fatal("Software not found in DB after create")
		}
		swSlug = "labb-software"
	})

	t.Run("edit_software", func(t *testing.T) {
		if swSlug == "" {
			t.Skip("software not found")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/software/"+swSlug+"/edit", "name=LabB+Software+v2")
		if err != nil {
			t.Fatalf("POST /software/%s/edit: %v", swSlug, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		db.Flush()
		swSlug = "labb-software-v2"
	})

	t.Run("delete_software", func(t *testing.T) {
		if swSlug == "" {
			t.Skip("software not found")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/software/"+swSlug+"/delete", "")
		if err != nil {
			t.Fatalf("POST /software/%s/delete: %v", swSlug, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var count int
		db.QueryRow("SELECT COUNT(*) FROM software_catalog WHERE slug=?", swSlug).Scan(&count)
		if count != 0 {
			t.Errorf("expected deleted, count=%d", count)
		}
	})
}

func TestLabB_Schedule_CreateFromScratch(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabB
	db := env.DB_B

	if !loginAndRefresh(lab, "labB_only", "test123") {
		t.Fatal("login failed")
	}

	var schedID int

	t.Run("create_schedule", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/schedules/create",
			"course_name=LabB+Course&day=Senin&time_start=08:00&time_end=09:40&lecturer=Dosen+LabB&class=IF-1")
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
		db.QueryRow("SELECT id FROM course_schedules ORDER BY id DESC LIMIT 1").Scan(&schedID)
		if schedID == 0 {
			t.Error("Schedule not found after create")
		}
	})

	t.Run("edit_schedule", func(t *testing.T) {
		if schedID == 0 {
			t.Skip("schedule not found")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/schedules/"+itoa(schedID)+"/edit",
			"course_name=LabB+Course+Updated&day=Selasa&time_start=09:00&time_end=10:30&lecturer=Dosen+LabB&class=IF-1")
		if err != nil {
			t.Fatalf("POST /schedules/%d/edit: %v", schedID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
	})

	t.Run("delete_schedule", func(t *testing.T) {
		if schedID == 0 {
			t.Skip("schedule not found")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/schedules/"+itoa(schedID)+"/delete", "")
		if err != nil {
			t.Fatalf("POST /schedules/%d/delete: %v", schedID, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var count int
		db.QueryRow("SELECT COUNT(*) FROM course_schedules WHERE id=?", schedID).Scan(&count)
		if count != 0 {
			t.Errorf("expected deleted, count=%d", count)
		}
	})
}

func TestLabB_Device_CreateFromScratch(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabB
	db := env.DB_B

	if !loginAndRefresh(lab, "labB_only", "test123") {
		t.Fatal("login failed")
	}

	// Seed prerequisite: category + device type directly in Lab B's DB
	db.Exec("INSERT OR IGNORE INTO categories (name, label_prefix) VALUES ('Lab B Category', 'LABCAT')")
	var catID int
	db.QueryRow("SELECT id FROM categories WHERE label_prefix='LABCAT'").Scan(&catID)
	if catID == 0 {
		t.Fatal("failed to seed category")
	}
	var dtID int
	db.QueryRow("SELECT id FROM device_types WHERE category_id=? AND name='Lab B Type'", catID).Scan(&dtID)
	if dtID == 0 {
		db.Exec("INSERT INTO device_types (category_id, name, label_prefix, usage_type) VALUES (?, 'Lab B Type', 'LBT', 'loanable')", catID)
		db.QueryRow("SELECT id FROM device_types WHERE category_id=? AND name='Lab B Type'", catID).Scan(&dtID)
	}
	if dtID == 0 {
		t.Fatal("failed to seed device type")
	}

	t.Run("create_device", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		data := "device_type_id=" + itoa(dtID) + "&serial_number=SN-LABB-DEV-001&condition=normal&location=Lab+B&purchase_date=&notes=From+scratch"
		resp, err := lab.post("/devices/create", data)
		if err != nil {
			t.Fatalf("POST /devices/create: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var devID int
		db.QueryRow("SELECT id FROM devices WHERE serial_number='SN-LABB-DEV-001'").Scan(&devID)
		if devID == 0 {
			t.Error("Device not found in DB after create")
		}
	})

	t.Run("detail_device", func(t *testing.T) {
		var devLabel string
		db.QueryRow("SELECT label FROM devices WHERE serial_number='SN-LABB-DEV-001'").Scan(&devLabel)
		if devLabel == "" {
			t.Skip("device not found")
		}
		devSlug := strings.ToLower(devLabel)
		resp, err := lab.get("/devices/labcat/lbt/" + devSlug)
		if err != nil {
			t.Fatalf("GET /devices/labcat/lbt/%s: %v", devSlug, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("edit_device", func(t *testing.T) {
		var devLabel string
		db.QueryRow("SELECT label FROM devices WHERE serial_number='SN-LABB-DEV-001'").Scan(&devLabel)
		if devLabel == "" {
			t.Skip("device not found")
		}
		devSlug := strings.ToLower(devLabel)
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.post("/devices/"+devSlug+"/edit",
			"device_type_id="+itoa(dtID)+"&label="+devLabel+"&serial_number=SN-LABB-DEV-002&condition=rusak&location=Lab+B&purchase_date=&notes=Updated")
		if err != nil {
			t.Fatalf("POST /devices/%s/edit: %v", devSlug, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 302 {
			t.Errorf("expected 302, got %d", resp.StatusCode)
		}
		var serial string
		db.QueryRow("SELECT serial_number FROM devices WHERE label=?", devLabel).Scan(&serial)
		if serial != "SN-LABB-DEV-002" {
			t.Errorf("expected serial SN-LABB-DEV-002, got %q", serial)
		}
	})

	t.Run("delete_device", func(t *testing.T) {
		var devLabel string
		db.QueryRow("SELECT label FROM devices WHERE serial_number='SN-LABB-DEV-002'").Scan(&devLabel)
		if devLabel == "" {
			t.Skip("device not found")
		}
		devSlug := strings.ToLower(devLabel)
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
		db.QueryRow("SELECT COUNT(*) FROM devices WHERE label=?", devLabel).Scan(&count)
		if count != 0 {
			t.Errorf("expected deleted, count=%d", count)
		}
	})

	t.Run("fail_create_missing_type", func(t *testing.T) {
		resp, err := lab.post("/devices/create",
			"device_type_id=&serial_number=SN-LABB-NO-TYPE&condition=normal&location=Lab+B&notes=")
		if err != nil {
			t.Fatalf("POST /devices/create no type: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for missing type, got %d", resp.StatusCode)
		}
	})
}
