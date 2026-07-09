package tests

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"inventaris-lab-kom/internal/database"
)

func TestBatchDownloadPCPhotos(t *testing.T) {
	env := wrapSharedEnv(t)
	lab := env.LabA
	db := env.DB_A

	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	uploadDir := lab.cfg.UploadDir
	pcDir := filepath.Join(uploadDir, "pc")
	os.MkdirAll(pcDir, 0755)

	ts := time.Now().UnixMilli()
	pc1Label := fmt.Sprintf("dl-pc1-%d", ts)

	photo1 := fmt.Sprintf("%s_serial_010726.jpeg", pc1Label)
	photo2 := fmt.Sprintf("%s_front_010726.jpeg", pc1Label)

	for _, name := range []string{photo1, photo2} {
		createTestJPEGFile(t, filepath.Join(pcDir, name))
	}

	cleanup := createPCOrDie(t, db, pc1Label, "dipakai", photo1, photo2)
	defer cleanup()

	resp, err := lab.get("/api/download-photos?type=pc")
	if err != nil {
		t.Fatalf("GET download-photos: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/zip" {
		t.Errorf("expected Content-Type application/zip, got %q", ct)
	}
	cd := resp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, ".zip") {
		t.Errorf("Content-Disposition should contain .zip, got %q", cd)
	}
	if !strings.Contains(cd, "foto_pcs") {
		t.Errorf("Content-Disposition should contain 'foto_pcs', got %q", cd)
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Fatal("empty response body")
	}

	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}

	found := map[string]bool{photo1: false, photo2: false}
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, photo1) {
			found[photo1] = true
		}
		if strings.HasSuffix(f.Name, photo2) {
			found[photo2] = true
		}
	}
	for name, ok := range found {
		if !ok {
			t.Errorf("test photo missing from zip: %s", name)
		}
	}
}

func TestBatchDownloadDevicePhotos(t *testing.T) {
	env := wrapSharedEnv(t)
	lab := env.LabA
	db := env.DB_A

	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	uploadDir := lab.cfg.UploadDir
	dtDir := filepath.Join(uploadDir, "device_types")
	instDir := filepath.Join(uploadDir, "device_installations")
	os.MkdirAll(dtDir, 0755)
	os.MkdirAll(instDir, 0755)

	ts := time.Now().UnixMilli()
	photoDT := fmt.Sprintf("mouse-dl-test-%d.jpeg", ts)
	photoInst := fmt.Sprintf("device-inst-dl-%d.jpeg", ts)

	createTestJPEGFile(t, filepath.Join(dtDir, photoDT))
	createTestJPEGFile(t, filepath.Join(instDir, photoInst))

	categoryID := insertCategoryOrDie(t, db, "Peripheral-test-dl")
	dtID := insertDeviceTypeOrDie(t, db, categoryID, "Mouse DL Test", "MOUSE-DL-TEST", photoDT)
	defer db.Exec("DELETE FROM device_types WHERE id = ?", dtID)

	deviceID := insertDeviceOrDie(t, db, dtID, "device-dl-test")
	instID := insertInstallationOrDie(t, db, deviceID, photoInst)
	defer db.Exec("DELETE FROM device_installations WHERE id = ?", instID)
	defer db.Exec("DELETE FROM devices WHERE id = ?", deviceID)

	resp, err := lab.get("/api/download-photos?type=devices")
	if err != nil {
		t.Fatalf("GET download-photos: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	cd := resp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "foto_devices") {
		t.Errorf("Content-Disposition should contain 'foto_devices', got %q", cd)
	}

	body, _ := io.ReadAll(resp.Body)
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}

	found := map[string]bool{photoDT: false, photoInst: false}
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, photoDT) {
			found[photoDT] = true
		}
		if strings.HasSuffix(f.Name, photoInst) {
			found[photoInst] = true
		}
	}
	for name, ok := range found {
		if !ok {
			t.Errorf("test photo missing from zip: %s", name)
		}
	}
}

func TestBatchDownloadPCNoPhotos(t *testing.T) {
	env := wrapSharedEnv(t)
	lab := env.LabA

	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	resp, err := lab.get("/api/download-photos?type=pc")
	if err != nil {
		t.Fatalf("GET download-photos: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Fatal("empty response body")
	}
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	if len(zr.File) == 0 {
		t.Log("zip is empty (no PCs with photos)")
	} else {
		cd := resp.Header.Get("Content-Disposition")
		if !strings.Contains(cd, "foto_pcs") {
			t.Errorf("Content-Disposition should contain 'foto_pcs', got %q", cd)
		}
		t.Logf("zip contains %d file(s) (seed data present)", len(zr.File))
	}
}

func TestBatchDownloadDeviceNoPhotos(t *testing.T) {
	env := wrapSharedEnv(t)
	lab := env.LabA

	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	resp, err := lab.get("/api/download-photos?type=devices")
	if err != nil {
		t.Fatalf("GET download-photos: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	if len(zr.File) == 0 {
		t.Log("zip is empty (no device types/installations with photos)")
	}
}

func TestBatchDownloadPCSomeMissingFiles(t *testing.T) {
	env := wrapSharedEnv(t)
	lab := env.LabA
	db := env.DB_A

	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	uploadDir := lab.cfg.UploadDir
	pcDir := filepath.Join(uploadDir, "pc")
	os.MkdirAll(pcDir, 0755)

	ts := time.Now().UnixMilli()
	pcLabel := fmt.Sprintf("dl-miss-%d", ts)
	photo1 := fmt.Sprintf("%s_serial_010726.jpeg", pcLabel)
	photo2 := fmt.Sprintf("%s_front_010726.jpeg", pcLabel)

	createTestJPEGFile(t, filepath.Join(pcDir, photo1))
	createTestJPEGFile(t, filepath.Join(pcDir, photo2))

	cleanup := createPCOrDie(t, db, pcLabel, "dipakai", photo1, photo2)
	defer cleanup()

	os.Remove(filepath.Join(pcDir, photo1))

	resp, err := lab.get("/api/download-photos?type=pc")
	if err != nil {
		t.Fatalf("GET download-photos: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}

	var foundMissing, foundExisting bool
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, photo1) {
			foundMissing = true
		}
		if strings.HasSuffix(f.Name, photo2) {
			foundExisting = true
		}
	}
	if foundMissing {
		t.Error("deleted photo should NOT be in zip")
	}
	if !foundExisting {
		t.Error("existing photo should be in zip")
	}
}

func TestBatchDownloadUnauthenticated(t *testing.T) {
	env := wrapSharedEnv(t)
	lab := env.LabA

	noRedirect := func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	client := &http.Client{CheckRedirect: noRedirect}

	req, _ := http.NewRequest("GET", lab.ts.URL+lab.prefix+"/api/download-photos?type=pc", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 302 or 401 for unauthenticated, got %d", resp.StatusCode)
	}
}

func TestBatchDownloadInvalidType(t *testing.T) {
	env := wrapSharedEnv(t)
	lab := env.LabA

	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	resp, err := lab.get("/api/download-photos?type=invalid")
	if err != nil {
		t.Fatalf("GET download-photos: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid type, got %d", resp.StatusCode)
	}
}

// --- helpers ---

func createPCOrDie(t *testing.T, db *database.DB, label, placement, photoSerial, photoFront string) func() {
	t.Helper()
	_, err := db.Exec(`INSERT INTO pcs (label, "row", "column", status, placement, photo_serial, photo_front, operating_system)
		VALUES (?, 1, 1, 'normal', ?, ?, ?, 'Windows 10')`, label, placement, photoSerial, photoFront)
	if err != nil {
		t.Fatalf("insert PC: %v", err)
	}
	return func() { db.Exec("DELETE FROM pcs WHERE label = ?", label) }
}

func insertCategoryOrDie(t *testing.T, db *database.DB, name string) int {
	t.Helper()
	res, err := db.Exec("INSERT INTO categories (name, label_prefix) VALUES (?, ?)", name, strings.ToUpper(name[:3]))
	if err != nil {
		t.Fatalf("insert category: %v", err)
	}
	id64, _ := res.LastInsertId()
	return int(id64)
}

func insertDeviceTypeOrDie(t *testing.T, db *database.DB, categoryID int, name, labelPrefix, photo string) int {
	t.Helper()
	res, err := db.Exec(`INSERT INTO device_types (category_id, name, brand, model, label_prefix, usage_type, default_location, photo)
		VALUES (?, ?, '', '', ?, 'loanable', '', ?)`, categoryID, name, labelPrefix, photo)
	if err != nil {
		t.Fatalf("insert device_type: %v", err)
	}
	id64, _ := res.LastInsertId()
	return int(id64)
}

func insertDeviceOrDie(t *testing.T, db *database.DB, deviceTypeID int, label string) int {
	t.Helper()
	res, err := db.Exec("INSERT INTO devices (device_type_id, label, condition) VALUES (?, ?, 'normal')", deviceTypeID, label)
	if err != nil {
		t.Fatalf("insert device: %v", err)
	}
	id64, _ := res.LastInsertId()
	return int(id64)
}

func insertInstallationOrDie(t *testing.T, db *database.DB, deviceID int, photo string) int {
	t.Helper()
	res, err := db.Exec("INSERT INTO device_installations (device_id, location_installed, photo) VALUES (?, ?, ?)", deviceID, "Lab Testing", photo)
	if err != nil {
		t.Fatalf("insert installation: %v", err)
	}
	id64, _ := res.LastInsertId()
	return int(id64)
}

func createTestJPEGFile(t *testing.T, path string) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	for x := 0; x < 10; x++ {
		for y := 0; y < 10; y++ {
			img.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test jpeg %s: %v", path, err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatalf("encode test jpeg %s: %v", path, err)
	}
}
