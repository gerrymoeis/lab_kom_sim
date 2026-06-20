package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// ============================================
// TestAPIUpload — image upload, delete temp, cleanup + fail
// ============================================

func TestAPIUpload(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	tsURL := env.TS.URL

	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	// Read test image once
	imagePath := filepath.Join("tests", "resources", "logbook.jpeg")
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatalf("read test image: %v", err)
	}

	t.Run("success_upload_image", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}

		var photoBuf bytes.Buffer
		mw := multipart.NewWriter(&photoBuf)
		fw, _ := mw.CreateFormFile("image", "logbook.jpeg")
		fw.Write(imageData)
		mw.WriteField("type", "serial")
		mw.Close()

		req, _ := http.NewRequest("POST", tsURL+lab.prefix+"/api/upload-image", &photoBuf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("X-CSRF-Token", lab.csrf)
		lab.addCookies(req)

		resp, err := lab.client.Do(req)
		if err != nil {
			t.Fatalf("upload request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}

		var uploadRes struct {
			Success bool   `json:"success"`
			FileRef string `json:"file_ref"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&uploadRes); err != nil {
			t.Fatalf("decode upload response: %v", err)
		}
		if !uploadRes.Success {
			t.Error("upload success=false")
		}
		if uploadRes.FileRef == "" {
			t.Error("file_ref is empty")
		}

		// Save file_ref for delete/cleanup tests
		_ = uploadRes.FileRef
	})

	t.Run("success_delete_temp", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		body := `{"file_ref":"non-existent-test-file.jpeg"}`
		resp, err := lab.postJSON("/api/delete-temp-file", body)
		if err != nil {
			t.Fatalf("POST /api/delete-temp-file: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var dr struct {
			Success bool `json:"success"`
		}
		json.NewDecoder(resp.Body).Decode(&dr)
		if !dr.Success {
			t.Error("delete-temp-file success=false")
		}
	})

	t.Run("success_cleanup_temp", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		body := `{"file_refs":["file1.jpeg","file2.jpeg"]}`
		resp, err := lab.postJSON("/api/cleanup-temp-files", body)
		if err != nil {
			t.Fatalf("POST /api/cleanup-temp-files: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var cr struct {
			Success bool `json:"success"`
		}
		json.NewDecoder(resp.Body).Decode(&cr)
		if !cr.Success {
			t.Error("cleanup-temp-files success=false")
		}
	})

	t.Run("fail_no_file", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		mw.WriteField("type", "serial")
		mw.Close()

		req, _ := http.NewRequest("POST", tsURL+lab.prefix+"/api/upload-image", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("X-CSRF-Token", lab.csrf)
		lab.addCookies(req)

		resp, err := lab.client.Do(req)
		if err != nil {
			t.Fatalf("upload request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_invalid_format", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("image", "test.txt")
		fw.Write([]byte("not an image"))
		mw.WriteField("type", "serial")
		mw.Close()

		req, _ := http.NewRequest("POST", tsURL+lab.prefix+"/api/upload-image", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("X-CSRF-Token", lab.csrf)
		lab.addCookies(req)

		resp, err := lab.client.Do(req)
		if err != nil {
			t.Fatalf("upload request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// TestAPIPCOperations — status, update, layout, move, swap, place + fail
// ============================================

func TestAPIPCOperations(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA

	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	var cadanganLabel string

	t.Run("status", func(t *testing.T) {
		resp, err := lab.get("/api/pc/status")
		if err != nil {
			t.Fatalf("GET /api/pc/status: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var statusRes struct {
			Counts map[string]int `json:"counts"`
			PCs    []any          `json:"pcs"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&statusRes); err != nil {
			t.Fatalf("decode status: %v", err)
		}
		if len(statusRes.PCs) == 0 {
			t.Error("expected non-empty PCs list")
		}
	})

	t.Run("update_status", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		body := `{"status":"warning"}`
		resp, err := lab.postJSON("/api/pc/pc-1/status", body)
		if err != nil {
			t.Fatalf("POST /api/pc/pc-1/status: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var statusRes struct {
			Success bool `json:"success"`
		}
		json.NewDecoder(resp.Body).Decode(&statusRes)
		if !statusRes.Success {
			t.Error("update status success=false")
		}
	})

	t.Run("layout", func(t *testing.T) {
		resp, err := lab.get("/api/pc/layout")
		if err != nil {
			t.Fatalf("GET /api/pc/layout: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var layoutRes struct {
			Grid     any `json:"grid"`
			Cadangan any `json:"cadangan"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&layoutRes); err != nil {
			t.Fatalf("decode layout: %v", err)
		}
	})

	t.Run("move_to_cadangan", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		body := `{"label":"pc-2"}`
		resp, err := lab.postJSON("/api/pc/move-to-cadangan", body)
		if err != nil {
			t.Fatalf("POST /api/pc/move-to-cadangan: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var moveRes struct {
			Success bool `json:"success"`
			Changes []struct {
				OldLabel string `json:"old_label"`
				NewLabel string `json:"new_label"`
			} `json:"changes"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&moveRes); err != nil {
			t.Fatalf("decode move response: %v", err)
		}
		if !moveRes.Success {
			t.Error("move-to-cadangan success=false")
		}
		if len(moveRes.Changes) > 0 {
			cadanganLabel = moveRes.Changes[0].NewLabel
		}
	})

	t.Run("place", func(t *testing.T) {
		if cadanganLabel == "" {
			t.Skip("no cadangan label from move_to_cadangan")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		body := fmt.Sprintf(`{"label":"%s","row":1,"col":5}`, cadanganLabel)
		resp, err := lab.postJSON("/api/pc/place", body)
		if err != nil {
			t.Fatalf("POST /api/pc/place: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var placeRes struct {
			Success bool `json:"success"`
		}
		json.NewDecoder(resp.Body).Decode(&placeRes)
		if !placeRes.Success {
			t.Error("place success=false")
		}
	})

	t.Run("swap", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		body := `{"a":"pc-3","b":"pc-4"}`
		resp, err := lab.postJSON("/api/pc/swap", body)
		if err != nil {
			t.Fatalf("POST /api/pc/swap: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var swapRes struct {
			Success bool `json:"success"`
		}
		json.NewDecoder(resp.Body).Decode(&swapRes)
		if !swapRes.Success {
			t.Error("swap success=false")
		}
	})

	t.Run("move", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		body := `{"label":"pc-5","row":2,"col":3}`
		resp, err := lab.postJSON("/api/pc/move", body)
		if err != nil {
			t.Fatalf("POST /api/pc/move: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var moveRes struct {
			Success bool `json:"success"`
		}
		json.NewDecoder(resp.Body).Decode(&moveRes)
		if !moveRes.Success {
			t.Error("move success=false")
		}
	})

	t.Run("replace", func(t *testing.T) {
		if cadanganLabel == "" {
			t.Skip("no cadangan label from move_to_cadangan")
		}
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		body := fmt.Sprintf(`{"target":"pc-1","spare":"%s"}`, cadanganLabel)
		resp, err := lab.postJSON("/api/pc/replace", body)
		if err != nil {
			t.Fatalf("POST /api/pc/replace: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var replaceRes struct {
			Success bool `json:"success"`
		}
		json.NewDecoder(resp.Body).Decode(&replaceRes)
		if !replaceRes.Success {
			t.Error("replace success=false")
		}
	})

	t.Run("fail_missing_fields", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.postJSON("/api/pc/swap", `{}`)
		if err != nil {
			t.Fatalf("POST /api/pc/swap empty: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("fail_not_found", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		body := `{"status":"warning"}`
		resp, err := lab.postJSON("/api/pc/nonexistent-pc-xyz/status", body)
		if err != nil {
			t.Fatalf("POST /api/pc/nonexistent/status: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 404 {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// TestAPIPCMoveRow — PCMoveRowToCadangan + fail
// ============================================

func TestAPIPCMoveRow(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	t.Run("move_row_success", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		data := url.Values{
			"row": {"8"}, "column": {"1"},
			"status": {"normal"}, "placement": {"dipakai"},
			"is_mahasiswa": {"true"},
			"serial_number": {"SN-MOVE-ROW-TEST"},
			"operating_system": {"Win11"}, "pc_type": {"PC"},
			"brand_model": {"Dell"}, "accessories": {"KB"},
			"processor": {"i5"}, "ram": {"8GB"}, "storage": {"256GB"},
		}.Encode()
		resp, err := lab.post("/pc/create", data)
		if err != nil {
			t.Fatalf("POST /pc/create: %v", err)
		}
		resp.Body.Close()
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err = lab.postJSON("/api/pc/move-row", `{"row":8}`)
		if err != nil {
			t.Fatalf("POST /api/pc/move-row: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var moveRes struct {
			Success bool `json:"success"`
		}
		json.NewDecoder(resp.Body).Decode(&moveRes)
		if !moveRes.Success {
			t.Error("move-row success=false")
		}
	})

	t.Run("fail_invalid_json", func(t *testing.T) {
		if !lab.refreshCSRF() {
			t.Fatal("failed to refresh CSRF")
		}
		resp, err := lab.postJSON("/api/pc/move-row", `{}`)
		if err != nil {
			t.Fatalf("POST /api/pc/move-row empty: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for missing row, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// TestAPIGetNextAssetCode — GetNextAssetCode + GetNextAssetCodes
// ============================================

func TestAPIGetNextAssetCode(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	t.Run("next_asset_code", func(t *testing.T) {
		resp, err := lab.get("/api/devices/next-asset-code?prefix=TEST")
		if err != nil {
			t.Fatalf("GET /api/devices/next-asset-code: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var res struct {
			NextCode string `json:"next_code"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if res.NextCode == "" {
			t.Error("next_code is empty")
		}
	})

	t.Run("next_asset_codes", func(t *testing.T) {
		resp, err := lab.get("/api/devices/next-asset-codes?prefix=TEST&count=3")
		if err != nil {
			t.Fatalf("GET /api/devices/next-asset-codes: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var res struct {
			Codes []string `json:"codes"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(res.Codes) == 0 {
			t.Error("codes is empty")
		}
	})

	t.Run("default_count_one", func(t *testing.T) {
		resp, err := lab.get("/api/devices/next-asset-codes?prefix=TEST")
		if err != nil {
			t.Fatalf("GET /api/devices/next-asset-codes: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var res struct {
			Codes []string `json:"codes"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(res.Codes) != 1 {
			t.Errorf("expected 1 code, got %d", len(res.Codes))
		}
	})

	t.Run("empty_prefix", func(t *testing.T) {
		resp, err := lab.get("/api/devices/next-asset-code?prefix=")
		if err != nil {
			t.Fatalf("GET /api/devices/next-asset-code empty prefix: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}
