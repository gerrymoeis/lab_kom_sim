package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func createTestJPEG(width, height int) ([]byte, error) {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	return buf.Bytes(), err
}

func createTestPNG(width, height int) ([]byte, error) {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	return buf.Bytes(), err
}

func uploadMultipart(lab *testLab, tsURL string, fileData []byte, filename, imgType string) (*http.Response, error) {
	if !lab.refreshCSRF() {
		return nil, fmt.Errorf("refresh CSRF failed")
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("image", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := fw.Write(fileData); err != nil {
		return nil, fmt.Errorf("write file data: %w", err)
	}
	if err := mw.WriteField("type", imgType); err != nil {
		return nil, fmt.Errorf("write type field: %w", err)
	}
	mw.Close()

	req, err := http.NewRequest("POST", tsURL+lab.prefix+"/api/upload-image", &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-CSRF-Token", lab.csrf)
	lab.addCookies(req)
	return lab.client.Do(req)
}

func decodeUploadResponse(resp *http.Response) (map[string]interface{}, error) {
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode JSON: %w (body: %s)", err, string(body))
	}
	return result, nil
}

func isJPEGMagic(buf []byte) bool {
	return len(buf) >= 2 && buf[0] == 0xFF && buf[1] == 0xD8
}

// ============================================
// 2A.1 + 2A.2 + 2A.3 + 2A.6 + 2A.7
// Image upload variations (default Android=false)
// ============================================

func TestUploadImageVariations(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	tsURL := env.TS.URL

	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	wd, _ := os.Getwd()
	projectRoot := findProjectRoot(wd)

	// 2A.1: HEIC upload → JPEG conversion + file verification
	t.Run("heic_to_jpeg_conversion", func(t *testing.T) {
		heicData, err := os.ReadFile(filepath.Join("tests", "resources", "pc1_sn.heic"))
		if err != nil {
			t.Fatalf("read HEIC test file: %v", err)
		}
		resp, err := uploadMultipart(lab, tsURL, heicData, "pc1_sn.heic", "serial")
		if err != nil {
			t.Fatalf("upload request: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		result, err := decodeUploadResponse(resp)
		if err != nil {
			t.Fatalf("decode response: %v", err)
		}
		success, _ := result["success"].(bool)
		if !success {
			t.Error("upload success=false")
		}
		fileRef, _ := result["file_ref"].(string)
		if fileRef == "" {
			t.Fatal("file_ref is empty")
		}
		if !strings.HasSuffix(fileRef, ".jpeg") && !strings.HasSuffix(fileRef, ".jpg") {
			t.Errorf("expected .jpeg file_ref, got %s", fileRef)
		}
		finalPath := filepath.Join(projectRoot, "uploads", lab.url, "temp", fileRef)
		if _, err := os.Stat(finalPath); os.IsNotExist(err) {
			t.Errorf("final JPEG not found on disk: %s", finalPath)
		}
		finalData, err := os.ReadFile(finalPath)
		if err == nil && !isJPEGMagic(finalData) {
			t.Error("saved file is not a valid JPEG (magic bytes mismatch)")
		}
	})

	// 2A.2: PNG and JPEG formats
	t.Run("png_and_jpeg_formats", func(t *testing.T) {
		pngData, err := createTestPNG(100, 100)
		if err != nil {
			t.Fatalf("create PNG: %v", err)
		}
		resp, err := uploadMultipart(lab, tsURL, pngData, "test.png", "device_type")
		if err != nil {
			t.Fatalf("upload PNG request: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("PNG upload: expected 200, got %d", resp.StatusCode)
		}
		result, err := decodeUploadResponse(resp)
		if err != nil {
			t.Fatalf("decode PNG response: %v", err)
		}
		success, _ := result["success"].(bool)
		if !success {
			t.Error("PNG upload success=false")
		}

		jpegData, err := createTestJPEG(100, 100)
		if err != nil {
			t.Fatalf("create JPEG: %v", err)
		}
		resp, err = uploadMultipart(lab, tsURL, jpegData, "test.jpeg", "serial")
		if err != nil {
			t.Fatalf("upload JPEG request: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("JPEG upload: expected 200, got %d", resp.StatusCode)
		}
		result, err = decodeUploadResponse(resp)
		if err != nil {
			t.Fatalf("decode JPEG response: %v", err)
		}
		success, _ = result["success"].(bool)
		if !success {
			t.Error("JPEG upload success=false")
		}

		jpgData, err := createTestJPEG(100, 100)
		if err != nil {
			t.Fatalf("create JPG: %v", err)
		}
		resp, err = uploadMultipart(lab, tsURL, jpgData, "test.jpg", "front")
		if err != nil {
			t.Fatalf("upload JPG request: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("JPG upload: expected 200, got %d", resp.StatusCode)
		}
		result, err = decodeUploadResponse(resp)
		if err != nil {
			t.Fatalf("decode JPG response: %v", err)
		}
		success, _ = result["success"].(bool)
		if !success {
			t.Error("JPG upload success=false")
		}
	})

	// 2A.3: Temp folder — file created in correct path, original cleaned up after compress
	t.Run("temp_folder_correctness", func(t *testing.T) {
		heicData, err := os.ReadFile(filepath.Join("tests", "resources", "pc1_sn.heic"))
		if err != nil {
			t.Fatalf("read HEIC test file: %v", err)
		}
		resp, err := uploadMultipart(lab, tsURL, heicData, "pc1_sn.heic", "serial")
		if err != nil {
			t.Fatalf("upload request: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		result, err := decodeUploadResponse(resp)
		if err != nil {
			t.Fatalf("decode response: %v", err)
		}
		fileRef, _ := result["file_ref"].(string)
		tempDir := filepath.Join(projectRoot, "uploads", lab.url, "temp")
		finalPath := filepath.Join(tempDir, fileRef)
		if _, err := os.Stat(finalPath); os.IsNotExist(err) {
			t.Errorf("file not in temp dir: %s", finalPath)
		}
		entries, err := os.ReadDir(tempDir)
		if err != nil {
			t.Fatalf("read temp dir: %v", err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "original_") {
				t.Errorf("original temp file not cleaned up: %s", entry.Name())
			}
		}
	})

	// 2A.6: Max file size 5MB rejection
	t.Run("file_size_exceeds_5mb", func(t *testing.T) {
		largeData := make([]byte, 6*1024*1024)
		resp, err := uploadMultipart(lab, tsURL, largeData, "large.jpg", "serial")
		if err != nil {
			t.Fatalf("upload large file: %v", err)
		}
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for large file, got %d", resp.StatusCode)
		}
		result, err := decodeUploadResponse(resp)
		if err != nil {
			t.Fatalf("decode response: %v", err)
		}
		success, _ := result["success"].(bool)
		if success {
			t.Error("expected success=false for large file")
		}
		msg, _ := result["message"].(string)
		if !strings.Contains(strings.ToLower(msg), "besar") &&
			!strings.Contains(strings.ToLower(msg), "5mb") &&
			!strings.Contains(strings.ToLower(msg), "5 mb") {
			t.Logf("file size error message: %s", msg)
		}
	})

	// 2A.7: Invalid MIME type — magic bytes mismatch
	t.Run("invalid_mime_type", func(t *testing.T) {
		textData := []byte("not an image file content")
		resp, err := uploadMultipart(lab, tsURL, textData, "test.jpg", "serial")
		if err != nil {
			t.Fatalf("upload invalid file: %v", err)
		}
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for invalid MIME, got %d", resp.StatusCode)
		}
		result, err := decodeUploadResponse(resp)
		if err != nil {
			t.Fatalf("decode response: %v", err)
		}
		success, _ := result["success"].(bool)
		if success {
			t.Error("expected success=false for invalid MIME")
		}
		msg, _ := result["message"].(string)
		if !strings.Contains(strings.ToLower(msg), "gambar") &&
			!strings.Contains(strings.ToLower(msg), "image") {
			t.Errorf("expected error about image/gambar, got: %s", msg)
		}

		// Also test with .png extension but text content
		resp, err = uploadMultipart(lab, tsURL, textData, "image.png", "serial")
		if err != nil {
			t.Fatalf("upload invalid PNG request: %v", err)
		}
		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for invalid PNG MIME, got %d", resp.StatusCode)
		}
	})
}

// ============================================
// 2A.4 + 2A.5: ANDROID flag behavior
// ============================================

func TestUploadAndroidFlag(t *testing.T) {
	// 2A.4: ANDROID=true — HEIC rejected, JPG accepted directly
	t.Run("android_true_heic_rejected_jpg_direct", func(t *testing.T) {
		env := setupTestEnvironment(t, TestConfigOverrides{Android: true})
		lab := env.LabA
		tsURL := env.TS.URL

		if !loginAndRefresh(lab, "labA_only", "test123") {
			t.Fatal("login failed")
		}

		wd, _ := os.Getwd()
		projectRoot := findProjectRoot(wd)

		// HEIC should be rejected (extension not allowed)
		heicData, err := os.ReadFile(filepath.Join("tests", "resources", "pc1_sn.heic"))
		if err != nil {
			t.Fatalf("read HEIC test file: %v", err)
		}
		resp, err := uploadMultipart(lab, tsURL, heicData, "pc1_sn.heic", "serial")
		if err != nil {
			t.Fatalf("upload HEIC request: %v", err)
		}
		if resp.StatusCode != 400 {
			t.Errorf("HEIC upload with Android=true: expected 400, got %d", resp.StatusCode)
		}
		result, err := decodeUploadResponse(resp)
		if err != nil {
			t.Fatalf("decode HEIC response: %v", err)
		}
		success, _ := result["success"].(bool)
		if success {
			t.Error("expected success=false for HEIC on Android")
		}

		// JPEG should be accepted directly (no compression)
		jpegData, err := createTestJPEG(50, 50)
		if err != nil {
			t.Fatalf("create test JPEG: %v", err)
		}
		resp, err = uploadMultipart(lab, tsURL, jpegData, "photo.jpg", "serial")
		if err != nil {
			t.Fatalf("upload JPEG request: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("JPEG upload with Android=true: expected 200, got %d", resp.StatusCode)
		}
		result, err = decodeUploadResponse(resp)
		if err != nil {
			t.Fatalf("decode JPEG response: %v", err)
		}
		success, _ = result["success"].(bool)
		if !success {
			t.Error("JPEG upload success=false on Android")
		}
		fileRef, _ := result["file_ref"].(string)
		if fileRef == "" {
			t.Fatal("file_ref is empty for Android JPEG upload")
		}

		// Verify file is on disk
		finalPath := filepath.Join(projectRoot, "uploads", lab.url, "temp", fileRef)
		savedData, err := os.ReadFile(finalPath)
		if err != nil {
			t.Fatalf("read saved file: %v", err)
		}
		if !isJPEGMagic(savedData) {
			t.Error("saved file is not a valid JPEG")
		}

		// Verify no original_ prefix file exists (file saved directly, not via compress)
		tempDir := filepath.Join(projectRoot, "uploads", lab.url, "temp")
		entries, err := os.ReadDir(tempDir)
		if err != nil {
			t.Fatalf("read temp dir: %v", err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "original_") {
				t.Errorf("unexpected original_ file when Android=true: %s", entry.Name())
			}
		}
	})

	// 2A.5: ANDROID=false (default) — HEIC accepted, compressed, original deleted
	// This is already tested by TestUploadImageVariations/heic_to_jpeg_conversion
	// but we add an explicit verification of original deletion here.
	t.Run("android_false_heic_compressed_original_cleanup", func(t *testing.T) {
		env := setupTestEnvironment(t)
		lab := env.LabA
		tsURL := env.TS.URL

		if !loginAndRefresh(lab, "labA_only", "test123") {
			t.Fatal("login failed")
		}

		wd, _ := os.Getwd()
		projectRoot := findProjectRoot(wd)

		heicData, err := os.ReadFile(filepath.Join("tests", "resources", "pc1_sn.heic"))
		if err != nil {
			t.Fatalf("read HEIC test file: %v", err)
		}
		resp, err := uploadMultipart(lab, tsURL, heicData, "pc1_sn.heic", "serial")
		if err != nil {
			t.Fatalf("upload HEIC request: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		result, err := decodeUploadResponse(resp)
		if err != nil {
			t.Fatalf("decode response: %v", err)
		}
		fileRef, _ := result["file_ref"].(string)
		if fileRef == "" {
			t.Fatal("file_ref is empty")
		}
		if !strings.HasSuffix(fileRef, ".jpeg") && !strings.HasSuffix(fileRef, ".jpg") {
			t.Errorf("expected JPEG output, got %s", fileRef)
		}

		// Verify final JPEG exists
		finalPath := filepath.Join(projectRoot, "uploads", lab.url, "temp", fileRef)
		if _, err := os.Stat(finalPath); os.IsNotExist(err) {
			t.Errorf("final JPEG not found: %s", finalPath)
		}

		// Verify original HEIC is deleted
		tempDir := filepath.Join(projectRoot, "uploads", lab.url, "temp")
		entries, err := os.ReadDir(tempDir)
		if err != nil {
			t.Fatalf("read temp dir: %v", err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "original_") {
				t.Errorf("original HEIC not cleaned up: %s (fileRef=%s)", entry.Name(), fileRef)
			}
		}
	})
}

// ============================================
// 2A.8: PC delete cascade — foto files removed from disk
// ============================================

func TestUploadCascadeDelete(t *testing.T) {
	env := setupTestEnvironment(t)
	lab := env.LabA
	tsURL := env.TS.URL

	if !loginAndRefresh(lab, "labA_only", "test123") {
		t.Fatal("login failed")
	}

	wd, _ := os.Getwd()
	projectRoot := findProjectRoot(wd)
	pcLabel := fmt.Sprintf("cascade-%d", time.Now().UnixMilli())

	// Step 1: Upload a photo — pass the PC label so file is named {label}_serial_{date}.jpeg.
	// This lets movePCPhotos() in the create handler find and move it automatically.
	heicData, err := os.ReadFile(filepath.Join("tests", "resources", "pc1_sn.heic"))
	if err != nil {
		t.Fatalf("read HEIC test file: %v", err)
	}

	// We need a multipart upload that includes both "label" and "type" fields.
	// The existing uploadMultipart helper doesn't support extra fields easily,
	// so we build the request manually.
	if !lab.refreshCSRF() {
		t.Fatal("refresh CSRF failed")
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("image", "pc1_sn.heic")
	fw.Write(heicData)
	mw.WriteField("type", "serial")
	mw.WriteField("label", pcLabel)
	mw.Close()

	req, _ := http.NewRequest("POST", tsURL+lab.prefix+"/api/upload-image", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-CSRF-Token", lab.csrf)
	lab.addCookies(req)
	resp, err := lab.client.Do(req)
	if err != nil {
		t.Fatalf("upload HEIC: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("upload expected 200, got %d", resp.StatusCode)
	}
	uploadResult, err := decodeUploadResponse(resp)
	if err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	fileRef, _ := uploadResult["file_ref"].(string)
	if fileRef == "" {
		t.Fatal("file_ref is empty")
	}
	t.Logf("Uploaded file_ref=%s", fileRef)

	// Step 2: Create a PC — the label matches the uploaded file prefix,
	// so movePCPhotos will automatically find and copy it from temp/ to pc/.
	if !lab.refreshCSRF() {
		t.Fatal("refresh CSRF failed before PC create")
	}

	formData := fmt.Sprintf(
		"label=%s&row=5&col=5&placement=dipakai&serial_number=SN-CASCADE-001&operating_system=Windows+10&serial_file_ref=%s",
		pcLabel, fileRef,
	)
	resp, err = lab.post("/pc/create", formData)
	if err != nil {
		t.Fatalf("create PC: %v", err)
	}
	if resp.StatusCode != 302 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("create PC expected 302, got %d: body=%s", resp.StatusCode, string(body))
	}
	resp.Body.Close()

	// Step 3: Verify PC was created in DB with correct data
	var labelInDB, photoSerialInDB string
	err = env.DB_A.QueryRow("SELECT label, photo_serial FROM pcs WHERE label=?", pcLabel).Scan(&labelInDB, &photoSerialInDB)
	if err != nil {
		t.Fatalf("query created PC: %v (label=%q)", err, pcLabel)
	}
	t.Logf("PC created: label=%q, photo_serial=%q", labelInDB, photoSerialInDB)

	// Step 4: Verify photo moved from temp/ to pc/ directory
	pcDir := filepath.Join(projectRoot, "uploads", lab.url, "pc")
	pcFiles, err := os.ReadDir(pcDir)
	if err != nil {
		t.Fatalf("read pc dir: %v", err)
	}
	var foundInPC bool
	for _, entry := range pcFiles {
		if strings.Contains(entry.Name(), pcLabel) {
			foundInPC = true
			t.Logf("Found photo in pc/: %s", entry.Name())
			break
		}
	}
	if !foundInPC {
		if photoSerialInDB != "" {
			// Photo was recorded in DB but file not in pc/ — check alternative names
			for _, entry := range pcFiles {
				t.Logf("  pc/ file: %s", entry.Name())
			}
		}
		t.Errorf("photo not found in pc/ dir after PC create; expected label %q in filename", pcLabel)
	}

	// Temp file should have been removed after copy
	tempPath := filepath.Join(projectRoot, "uploads", lab.url, "temp", fileRef)
	if _, err := os.Stat(tempPath); err == nil {
		t.Log("photo still in temp/ after PC create")
	}

	// Step 4: Delete the PC — route is /{lab}/pc/{label}/delete
	if !lab.refreshCSRF() {
		t.Fatal("refresh CSRF failed before delete")
	}
	resp, err = lab.post(fmt.Sprintf("/pc/%s/delete", pcLabel), "")
	if err != nil {
		t.Fatalf("delete PC: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 302 {
		t.Fatalf("delete PC expected 302, got %d", resp.StatusCode)
	}

	// Step 5: Verify photo removed from pc/ directory
	pcFilesAfter, err := os.ReadDir(pcDir)
	if err != nil {
		t.Fatalf("read pc dir after delete: %v", err)
	}
	for _, entry := range pcFilesAfter {
		if strings.Contains(entry.Name(), pcLabel) {
			t.Errorf("photo still exists after PC delete: %s", entry.Name())
		}
	}
}
