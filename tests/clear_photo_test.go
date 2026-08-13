package tests

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── Helpers ───────────────────────────────────────────────────────

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// ensurePhotoFile creates a dummy file under uploads/<lab>/<subDir>/<name>.
func ensurePhotoFile(t *testing.T, lab *testLab, subDir, name string) string {
	t.Helper()
	dir := filepath.Join(sharedEnv.Config.UploadPath, lab.url, subDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write photo file %s: %v", p, err)
	}
	return p
}

func clearPhoto(lab *testLab, body string) (*http.Response, error) {
	return lab.postJSON("/api/clear-photo", body)
}

// seedPCWithPhoto inserts a PC row + dummy file for photo_serial / photo_front.
func seedPCWithPhoto(t *testing.T, env *TestEnvironment, lab *testLab, photoField, photoCol string) (label, photoPath string) {
	t.Helper()
	label = fmt.Sprintf("cp-pc-%d", time.Now().UnixMilli())
	photoName := label + "_" + photoField + "_test.jpg"
	photoPath = ensurePhotoFile(t, lab, "pc", photoName)
	_, err := env.DB_A.Exec(`INSERT INTO pcs (label, placement, operating_system, `+photoCol+`)
		VALUES (?, 'dipakai', 'Windows 10', ?)`, label, photoName)
	if err != nil {
		t.Fatalf("seed pc: %v", err)
	}
	t.Cleanup(func() {
		env.DB_A.Exec("DELETE FROM pcs WHERE label = ?", label)
		os.Remove(photoPath)
	})
	return label, photoPath
}

// seedDeviceTypeWithPhoto inserts a device_type row + dummy file, returns (id, slug, photoPath).
func seedDeviceTypeWithPhoto(t *testing.T, env *TestEnvironment, lab *testLab) (id int, slug, photoPath string) {
	t.Helper()
	env.DB_A.Exec("INSERT OR IGNORE INTO categories (id, name, label_prefix) VALUES (990, 'CPTestCat', 'CPTESTCAT')")
	prefix := fmt.Sprintf("cpdt%d", time.Now().UnixMilli()%100000)
	photoName := prefix + ".jpg"
	photoPath = ensurePhotoFile(t, lab, "device_types", photoName)
	res, err := env.DB_A.Exec(`INSERT INTO device_types (category_id, name, brand, model, label_prefix, usage_type, photo)
		VALUES (990, 'CP Test DT', 'Brand', 'Model', ?, 'loanable', ?)`, prefix, photoName)
	if err != nil {
		t.Fatalf("seed device_type: %v", err)
	}
	id64, _ := res.LastInsertId()
	t.Cleanup(func() {
		env.DB_A.Exec("DELETE FROM device_types WHERE id = ?", int(id64))
		os.Remove(photoPath)
	})
	return int(id64), prefix, photoPath
}

// seedInstallationWithPhoto inserts device + installation rows + dummy file, returns (id, photoPath).
func seedInstallationWithPhoto(t *testing.T, env *TestEnvironment, lab *testLab) (id int, photoPath string) {
	t.Helper()
	env.DB_A.Exec("INSERT OR IGNORE INTO categories (id, name, label_prefix) VALUES (991, 'CITestCat', 'CITESTCAT')")
	env.DB_A.Exec(`INSERT OR IGNORE INTO device_types (id, category_id, name, brand, model, label_prefix, usage_type)
		VALUES (991, 991, 'CI DT', 'Brand', 'Model', 'CIDT', 'installable')`)
	res, err := env.DB_A.Exec(`INSERT INTO devices (device_type_id, label, condition) VALUES (991, ?, 'normal')`,
		fmt.Sprintf("cp-dev-%d", time.Now().UnixMilli()))
	if err != nil {
		t.Fatalf("seed device: %v", err)
	}
	devID64, _ := res.LastInsertId()
	photoName := fmt.Sprintf("inst-%d.jpg", time.Now().UnixMilli())
	photoPath = ensurePhotoFile(t, lab, "device_installations", photoName)
	res, err = env.DB_A.Exec(`INSERT INTO device_installations (device_id, location_installed, photo)
		VALUES (?, 'Lab 1', ?)`, int(devID64), photoName)
	if err != nil {
		t.Fatalf("seed installation: %v", err)
	}
	id64, _ := res.LastInsertId()
	t.Cleanup(func() {
		env.DB_A.Exec("DELETE FROM device_installations WHERE id = ?", int(id64))
		env.DB_A.Exec("DELETE FROM devices WHERE id = ?", int(devID64))
		env.DB_A.Exec("DELETE FROM device_types WHERE id = 991")
		os.Remove(photoPath)
	})
	return int(id64), photoPath
}

// ── Tests ──────────────────────────────────────────────────────────

func TestClearPhoto(t *testing.T) {
	env := wrapSharedEnv(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	t.Run("pc_serial_success", func(t *testing.T) {
		label, photoPath := seedPCWithPhoto(t, env, lab, "serial", "photo_serial")
		if !fileExists(photoPath) {
			t.Fatalf("precondition: photo missing: %s", photoPath)
		}
		resp, err := clearPhoto(lab, fmt.Sprintf(`{"type":"pc","identifier":%q,"photo":"serial"}`, label))
		if err != nil {
			t.Fatalf("clear: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var val string
		if err := env.DB_A.QueryRow("SELECT COALESCE(photo_serial,'') FROM pcs WHERE label=?", label).Scan(&val); err != nil {
			t.Fatalf("query: %v", err)
		}
		if val != "" {
			t.Errorf("photo_serial should be empty, got %q", val)
		}
		if fileExists(photoPath) {
			t.Errorf("file should be deleted: %s", photoPath)
		}
	})

	t.Run("pc_front_success", func(t *testing.T) {
		label, photoPath := seedPCWithPhoto(t, env, lab, "front", "photo_front")
		if !fileExists(photoPath) {
			t.Fatalf("precondition: photo missing: %s", photoPath)
		}
		resp, err := clearPhoto(lab, fmt.Sprintf(`{"type":"pc","identifier":%q,"photo":"front"}`, label))
		if err != nil {
			t.Fatalf("clear: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var val string
		if err := env.DB_A.QueryRow("SELECT COALESCE(photo_front,'') FROM pcs WHERE label=?", label).Scan(&val); err != nil {
			t.Fatalf("query: %v", err)
		}
		if val != "" {
			t.Errorf("photo_front should be empty, got %q", val)
		}
		if fileExists(photoPath) {
			t.Errorf("file should be deleted: %s", photoPath)
		}
	})

	t.Run("pc_not_found", func(t *testing.T) {
		resp, err := clearPhoto(lab, `{"type":"pc","identifier":"cp-nonexistent-xyz","photo":"serial"}`)
		if err != nil {
			t.Fatalf("clear: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("pc_invalid_photo_field", func(t *testing.T) {
		label, _ := seedPCWithPhoto(t, env, lab, "serial", "photo_serial")
		resp, err := clearPhoto(lab, fmt.Sprintf(`{"type":"pc","identifier":%q,"photo":"bogus"}`, label))
		if err != nil {
			t.Fatalf("clear: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("device_type_success", func(t *testing.T) {
		id, slug, photoPath := seedDeviceTypeWithPhoto(t, env, lab)
		if !fileExists(photoPath) {
			t.Fatalf("precondition: photo missing: %s", photoPath)
		}
		resp, err := clearPhoto(lab, fmt.Sprintf(`{"type":"device_type","identifier":%q,"photo":""}`, slug))
		if err != nil {
			t.Fatalf("clear: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var val string
		if err := env.DB_A.QueryRow("SELECT COALESCE(photo,'') FROM device_types WHERE id=?", id).Scan(&val); err != nil {
			t.Fatalf("query: %v", err)
		}
		if val != "" {
			t.Errorf("dt photo should be empty, got %q", val)
		}
		if fileExists(photoPath) {
			t.Errorf("file should be deleted: %s", photoPath)
		}
	})

	t.Run("device_type_not_found", func(t *testing.T) {
		resp, err := clearPhoto(lab, `{"type":"device_type","identifier":"cp-nope-dt","photo":""}`)
		if err != nil {
			t.Fatalf("clear: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("installation_success", func(t *testing.T) {
		id, photoPath := seedInstallationWithPhoto(t, env, lab)
		if !fileExists(photoPath) {
			t.Fatalf("precondition: photo missing: %s", photoPath)
		}
		resp, err := clearPhoto(lab, fmt.Sprintf(`{"type":"device_installation","identifier":"%d","photo":""}`, id))
		if err != nil {
			t.Fatalf("clear: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var val string
		if err := env.DB_A.QueryRow("SELECT COALESCE(photo,'') FROM device_installations WHERE id=?", id).Scan(&val); err != nil {
			t.Fatalf("query: %v", err)
		}
		if val != "" {
			t.Errorf("inst photo should be empty, got %q", val)
		}
		if fileExists(photoPath) {
			t.Errorf("file should be deleted: %s", photoPath)
		}
	})

	t.Run("installation_not_found", func(t *testing.T) {
		resp, err := clearPhoto(lab, `{"type":"device_installation","identifier":"999999","photo":""}`)
		if err != nil {
			t.Fatalf("clear: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("installation_invalid_id", func(t *testing.T) {
		resp, err := clearPhoto(lab, `{"type":"device_installation","identifier":"abc","photo":""}`)
		if err != nil {
			t.Fatalf("clear: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("invalid_entity_type", func(t *testing.T) {
		resp, err := clearPhoto(lab, `{"type":"bogus","identifier":"x","photo":""}`)
		if err != nil {
			t.Fatalf("clear: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("invalid_body", func(t *testing.T) {
		resp, err := clearPhoto(lab, `{not json`)
		if err != nil {
			t.Fatalf("clear: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
	})
}

func TestClearPhotoAuthz(t *testing.T) {
	env := wrapSharedEnv(t)
	lab := env.LabA

	// Unauthenticated: postJSON with no cookies → should be rejected (not 200).
	resp, err := lab.postJSON("/api/clear-photo", `{"type":"pc","identifier":"x","photo":"serial"}`)
	if err != nil {
		t.Fatalf("clear unauth: %v", err)
	}
	defer resp.Body.Close()
	loc := lab.getLocation(resp)
	if resp.StatusCode == 200 {
		t.Errorf("expected auth rejection, got 200 (loc=%s)", loc)
	}
	if !strings.Contains(loc, "/login") && resp.StatusCode != http.StatusFound {
		t.Logf("unauthenticated rejected; status=%d loc=%s", resp.StatusCode, loc)
	}
}