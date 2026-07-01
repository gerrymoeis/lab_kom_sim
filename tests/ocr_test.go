package tests

import (
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/repository"
	"inventaris-lab-kom/internal/services"
)

// ============================================
// Helpers
// ============================================

func createTempJPEG(t *testing.T) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{255, 255, 255, 255})
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jpg")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return path
}

func geminiResp(ocrText string) string {
	b, _ := json.Marshal(map[string]any{
		"candidates": []any{map[string]any{
			"content": map[string]any{
				"parts": []any{map[string]any{"text": ocrText}},
			},
		}},
	})
	return string(b)
}

func openRouterResp(ocrText string) string {
	b, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{
			"message": map[string]any{"content": ocrText},
		}},
	})
	return string(b)
}

func mockGemini(t *testing.T, respBody string, statusCode int, counter *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if counter != nil {
			counter.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte(respBody))
	}))
}

func mockOpenRouter(t *testing.T, respBody string, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte(respBody))
	}))
}

func validOCRJSON() string {
	return `{"entries":[{"date":"2026-06-23","student_name":"Budi Santoso","nim":"24091397001","time_in":"08:00","time_out":"10:00","purpose":"Praktikum"}]}`
}

func validOCRMultiJSON() string {
	return `{"entries":[
		{"date":"2026-06-23","student_name":"Budi Santoso","nim":"24091397001","time_in":"08:00","time_out":"10:00","purpose":"Praktikum"},
		{"date":"2026-06-24","student_name":"Siti Aminah","nim":"24091397002","time_in":"09:00","time_out":"11:00","purpose":"Praktikum Web"}
	]}`
}

// ============================================
// 2B.1 - 2B.8: OCRService unit tests
// ============================================

func TestOCRService(t *testing.T) {
	imgPath := createTempJPEG(t)

	// ============================================
	// 2B.1: OCR success — fake API return valid JSON entries
	// ============================================
	t.Run("2B.1_success", func(t *testing.T) {
		ts := mockGemini(t, geminiResp(validOCRJSON()), 200, nil)
		defer ts.Close()

		s := services.NewOCRService("test-key", "",
			services.WithGeminiBaseURL(ts.URL))

		result, err := s.ExtractLogbookFromImage(imgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Fatal("expected success")
		}
		if len(result.Entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result.Entries))
		}
		if result.Entries[0].StudentName != "Budi Santoso" {
			t.Errorf("expected Budi Santoso, got %s", result.Entries[0].StudentName)
		}
		if result.Entries[0].NIM != "24091397001" {
			t.Errorf("expected 24091397001, got %s", result.Entries[0].NIM)
		}
	})

	// ============================================
	// 2B.2: OCR fallback — Gemini gagal → OpenRouter sukses
	// ============================================
	t.Run("2B.2_fallback", func(t *testing.T) {
		geminiTS := mockGemini(t, `{"error":"bad request"}`, 400, nil)
		defer geminiTS.Close()

		orTS := mockOpenRouter(t, openRouterResp(validOCRJSON()), 200)
		defer orTS.Close()

		s := services.NewOCRService("gemini-key", "or-key",
			services.WithGeminiBaseURL(geminiTS.URL),
			services.WithOpenRouterBaseURL(orTS.URL))

		result, err := s.ExtractLogbookFromImage(imgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Fatal("expected success via fallback")
		}
		if len(result.Entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result.Entries))
		}
		if result.Entries[0].StudentName != "Budi Santoso" {
			t.Errorf("expected Budi Santoso (from OpenRouter), got %s", result.Entries[0].StudentName)
		}
	})

	// ============================================
	// 2B.3: OCR retry — API error 429/500 → retry 3x → success
	// ============================================
	t.Run("2B.3_retry_success", func(t *testing.T) {
		var callCount atomic.Int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := callCount.Add(1)
			if count <= 2 {
				w.WriteHeader(429)
				w.Write([]byte(`{"error":"rate limited"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write([]byte(geminiResp(validOCRJSON())))
		}))
		defer ts.Close()

		s := services.NewOCRService("test-key", "",
			services.WithGeminiBaseURL(ts.URL))

		result, err := s.ExtractLogbookFromImage(imgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Fatal("expected success after retries")
		}
		if c := callCount.Load(); c != 3 {
			t.Errorf("expected 3 API calls (1 initial + 2 retries), got %d", c)
		}
	})

	// ============================================
	// 2B.4: OCR retry exhausted — all retries fail → error
	// ============================================
	t.Run("2B.4_retry_exhausted", func(t *testing.T) {
		var callCount atomic.Int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount.Add(1)
			w.WriteHeader(500)
			w.Write([]byte(`{"error":"internal error"}`))
		}))
		defer ts.Close()

		s := services.NewOCRService("test-key", "",
			services.WithGeminiBaseURL(ts.URL))

		_, err := s.ExtractLogbookFromImage(imgPath)
		if err == nil {
			t.Fatal("expected error after retries exhausted")
		}
		if c := callCount.Load(); c != 4 {
			t.Errorf("expected 4 API calls (1 initial + 3 retries), got %d", c)
		}
	})

	// ============================================
	// 2B.5: OCR timeout — 60s timeout → error
	// ============================================
	t.Run("2B.5_timeout", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(200)
			w.Write([]byte(geminiResp(validOCRJSON())))
		}))
		defer ts.Close()

		client := &http.Client{Timeout: 50 * time.Millisecond}
		s := services.NewOCRService("test-key", "",
			services.WithHTTPClient(client),
			services.WithGeminiBaseURL(ts.URL))

		_, err := s.ExtractLogbookFromImage(imgPath)
		if err == nil {
			t.Fatal("expected timeout error")
		}
	})

	// ============================================
	// 2B.6: No API key — error "tidak dikonfigurasi"
	// ============================================
	t.Run("2B.6_no_api_key", func(t *testing.T) {
		s := services.NewOCRService("", "")
		_, err := s.ExtractLogbookFromImage(imgPath)
		if err == nil {
			t.Fatal("expected error for missing API key")
		}
		if !strings.Contains(err.Error(), "tidak dikonfigurasi") {
			t.Errorf("expected 'tidak dikonfigurasi', got: %v", err)
		}
	})

	// ============================================
	// 2B.7: OCR parsing — markdown code fence, raw JSON, empty entries
	// ============================================
	t.Run("2B.7_parsing", func(t *testing.T) {
		t.Run("markdown_code_fence", func(t *testing.T) {
			mdText := "```json\n" + validOCRJSON() + "\n```"
			ts := mockGemini(t, geminiResp(mdText), 200, nil)
			defer ts.Close()

			s := services.NewOCRService("key", "",
				services.WithGeminiBaseURL(ts.URL))

			result, err := s.ExtractLogbookFromImage(imgPath)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.Success || len(result.Entries) != 1 {
				t.Fatal("expected success with 1 entry")
			}
			if result.Entries[0].NIM != "24091397001" {
				t.Errorf("expected NIM 24091397001, got %s", result.Entries[0].NIM)
			}
		})

		t.Run("raw_json_no_markdown", func(t *testing.T) {
			ts := mockGemini(t, geminiResp(validOCRJSON()), 200, nil)
			defer ts.Close()

			s := services.NewOCRService("key", "",
				services.WithGeminiBaseURL(ts.URL))

			result, err := s.ExtractLogbookFromImage(imgPath)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.Success || len(result.Entries) != 1 {
				t.Fatal("expected success with 1 entry")
			}
		})

		t.Run("empty_entries", func(t *testing.T) {
			ts := mockGemini(t, geminiResp(`{"entries":[]}`), 200, nil)
			defer ts.Close()

			s := services.NewOCRService("key", "",
				services.WithGeminiBaseURL(ts.URL))

			result, err := s.ExtractLogbookFromImage(imgPath)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Success {
				t.Fatal("expected failure for empty entries")
			}
			if !strings.Contains(result.Error, "tidak ada entry") {
				t.Errorf("expected 'tidak ada entry', got: %s", result.Error)
			}
		})

		t.Run("non_json_response", func(t *testing.T) {
			ts := mockGemini(t, geminiResp("This is not JSON at all"), 200, nil)
			defer ts.Close()

			s := services.NewOCRService("key", "",
				services.WithGeminiBaseURL(ts.URL))

			result, err := s.ExtractLogbookFromImage(imgPath)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Success {
				t.Fatal("expected failure for non-JSON")
			}
			if !strings.Contains(result.Error, "Gagal parse JSON") {
				t.Errorf("expected 'Gagal parse JSON', got: %s", result.Error)
			}
		})
	})

	// ============================================
	// 2B.8: OCR post-processing — normalisasi waktu, nama, NIM, forward-fill purpose
	// ============================================
	t.Run("2B.8_postprocessing", func(t *testing.T) {
		postJSON := `{"entries":[
			{"date":"1 2026-06-23","student_name":"BUDI SANTOSO","nim":" 24091397001 ","time_in":"13.00 - 14.40","time_out":"","purpose":"Praktikum WEB"},
			{"date":"  2026-06-23","student_name":"HERMAN SW","nim":"24091397002","time_in":"9:00","time_out":"10:30","purpose":""}
		]}`
		ts := mockGemini(t, geminiResp(postJSON), 200, nil)
		defer ts.Close()

		s := services.NewOCRService("key", "",
			services.WithGeminiBaseURL(ts.URL))

		result, err := s.ExtractLogbookFromImage(imgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success || len(result.Entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(result.Entries))
		}

		e1 := result.Entries[0]
		if e1.Date != "2026-06-23" {
			t.Errorf("entry1 date: expected 2026-06-23, got %s", e1.Date)
		}
		if e1.TimeIn != "13:00" {
			t.Errorf("entry1 time_in: expected 13:00, got %s", e1.TimeIn)
		}
		if e1.TimeOut != "14:40" {
			t.Errorf("entry1 time_out: expected 14:40, got %s", e1.TimeOut)
		}
		if e1.StudentName != "BUDI SANTOSO" {
			t.Errorf("entry1 name: expected 'BUDI SANTOSO', got '%s'", e1.StudentName)
		}
		if e1.NIM != "24091397001" {
			t.Errorf("entry1 NIM: expected 24091397001, got %s", e1.NIM)
		}
		if e1.Purpose != "Praktikum WEB" {
			t.Errorf("entry1 purpose: expected 'Praktikum WEB', got '%s'", e1.Purpose)
		}

		e2 := result.Entries[1]
		if e2.TimeIn != "09:00" {
			t.Errorf("entry2 time_in: expected 09:00, got %s", e2.TimeIn)
		}
		if e2.TimeOut != "10:30" {
			t.Errorf("entry2 time_out: expected 10:30, got %s", e2.TimeOut)
		}
		if e2.Purpose != "Praktikum WEB" {
			t.Errorf("entry2 purpose: expected 'Praktikum WEB' (forward-filled), got '%s'", e2.Purpose)
		}
		if e2.StudentName != "HERMAN S.W" {
			t.Errorf("entry2 name: expected 'HERMAN S.W', got '%s'", e2.StudentName)
		}
		if e2.NIM != "24091397002" {
			t.Errorf("entry2 NIM: expected 24091397002, got %s", e2.NIM)
		}
		if e2.Date != "2026-06-23" {
			t.Errorf("entry2 date: expected 2026-06-23, got %s", e2.Date)
		}
	})
}

// ============================================
// Helper: minimal DB + LogbookService for 2B.9 & 2B.10
// ============================================

func setupLogbookTest(t *testing.T) (*services.LogbookService, *database.DB, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "logbook_test.db")
	db, err := database.InitDB(dbPath, "")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	cleanup := func() { db.Close() }

	statements := []string{
		`CREATE TABLE IF NOT EXISTS logbook_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date DATE NOT NULL,
			student_name TEXT NOT NULL,
			nim TEXT NOT NULL CHECK(length(nim) = 11),
			time_in TEXT NOT NULL,
			time_out TEXT,
			purpose TEXT,
			source_file TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_date ON logbook_entries(date)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_nim ON logbook_entries(nim)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_logbook_unique ON logbook_entries(date, LOWER(TRIM(student_name)), time_in)`,
		`CREATE TABLE IF NOT EXISTS activity_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			username TEXT,
			user_role TEXT,
			action TEXT,
			entity_type TEXT,
			entity_id INTEGER,
			description TEXT,
			old_values TEXT,
			new_values TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			ip_address TEXT,
			user_agent TEXT,
			status TEXT,
			error_message TEXT
		)`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			cleanup()
			t.Fatalf("create table: %v", err)
		}
	}

	repo := repository.NewLogbookRepository(db)
	activityLog := services.NewActivityLogService(db, services.DummyNotifier{}, 0, 0)
	svc := services.NewLogbookService(repo, activityLog)
	return svc, db, cleanup
}

// ============================================
// 2B.9: Duplicate detection — Jaro-Winkler intra-batch + cross-DB
// ============================================

func TestDuplicateDetection(t *testing.T) {
	svc, db, cleanup := setupLogbookTest(t)
	defer cleanup()

	// Seed existing entries (cross-DB duplicates)
	now := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)
	_, err := db.Exec(`INSERT INTO logbook_entries (date, student_name, nim, time_in, time_out, purpose) VALUES (?, ?, ?, ?, ?, ?)`,
		now, "Budi Santoso", "24091397001", "08:00", "10:00", "Praktikum")
	if err != nil {
		t.Fatalf("seed existing entry: %v", err)
	}
	_, err = db.Exec(`INSERT INTO logbook_entries (date, student_name, nim, time_in, time_out, purpose) VALUES (?, ?, ?, ?, ?, ?)`,
		now, "Siti Aminah", "24091397002", "09:00", "11:00", "Praktikum Web")
	if err != nil {
		t.Fatalf("seed existing entry: %v", err)
	}

	tests := []struct {
		name     string
		entries  []models.LogbookEntry
		wantDups int
		wantNo   bool
	}{
		{
			name: "intra_batch_duplicate",
			entries: []models.LogbookEntry{
				{Date: now, StudentName: "Budi Santoso", NIM: "24091397001", TimeIn: "08:00", TimeOut: "10:00"},
				{Date: now, StudentName: "Budi Santoso", NIM: "24091397001", TimeIn: "08:00", TimeOut: "10:00"},
			},
			wantDups: 1,
		},
		{
			name: "cross_db_duplicate",
			entries: []models.LogbookEntry{
				{Date: now, StudentName: "Budi Santoso", NIM: "24091397001", TimeIn: "08:00", TimeOut: "10:00"},
			},
			wantDups: 1,
		},
		{
			name: "similar_name_jaro_winkler",
			entries: []models.LogbookEntry{
				{Date: now, StudentName: "Budi Santoso", NIM: "24091397001", TimeIn: "08:00", TimeOut: "10:00"},
				{Date: now, StudentName: "Budi Santos", NIM: "24091397001", TimeIn: "08:00", TimeOut: "10:00"},
			},
			wantDups: 1,
		},
		{
			name: "different_time_no_dup",
			entries: []models.LogbookEntry{
				{Date: now, StudentName: "Budi Santoso", NIM: "24091397001", TimeIn: "10:00", TimeOut: "12:00"},
			},
			wantNo: true,
		},
		{
			name: "different_date_no_dup",
			entries: []models.LogbookEntry{
				{Date: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), StudentName: "Budi Santoso", NIM: "24091397001", TimeIn: "08:00", TimeOut: "10:00"},
			},
			wantNo: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := svc.CheckDuplicates(tt.entries)
			if tt.wantNo {
				if len(groups) > 0 {
					t.Errorf("expected no duplicate groups, got %d", len(groups))
					for _, g := range groups {
						t.Logf("  group %s type=%s members=%v", g.GroupID, g.Type, g.Members)
					}
				}
				return
			}
			if len(groups) == 0 {
				t.Errorf("expected at least 1 duplicate group, got 0")
			} else {
				var totalDups int
				for _, g := range groups {
					totalDups += len(g.Members)
				}
				if totalDups < tt.wantDups {
					t.Errorf("expected at least %d members flagged as dup, got %d", tt.wantDups, totalDups)
				}
			}
		})
	}
}

// ============================================
// 2B.10: BulkSave — validasi NIM 11 digit, duplicate skip, transaction rollback
// ============================================

func TestBulkSave(t *testing.T) {
	svc, db, cleanup := setupLogbookTest(t)
	defer cleanup()

	// Seed existing entries
	now := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)
	_, err := db.Exec(`INSERT INTO logbook_entries (date, student_name, nim, time_in, time_out, purpose) VALUES (?, ?, ?, ?, ?, ?)`,
		now, "Budi Santoso", "24091397001", "08:00", "10:00", "Praktikum")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	t.Run("all_valid_entries", func(t *testing.T) {
		defer db.Exec("DELETE FROM logbook_entries WHERE id > 1")

		entries := []repository.BulkEntry{
			{Date: time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC), StudentName: "Siti Aminah", NIM: "24091397002", TimeIn: "09:00", TimeOut: "11:00", Purpose: "Praktikum Web"},
			{Date: time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC), StudentName: "Joko Widodo", NIM: "24091397003", TimeIn: "10:00", TimeOut: "12:00", Purpose: "Research"},
		}
		saved, dups, err := svc.BulkSave(entries, "test.jpg", map[int]bool{}, 1, "admin", "admin", "127.0.0.1", "test")
		if err != nil {
			t.Fatalf("BulkSave: %v", err)
		}
		if saved != 2 {
			t.Errorf("expected 2 saved, got %d", saved)
		}
		if dups != 0 {
			t.Errorf("expected 0 duplicates, got %d", dups)
		}

		var count int
		db.QueryRow("SELECT COUNT(*) FROM logbook_entries").Scan(&count)
		if count != 3 {
			t.Errorf("expected 3 total entries, got %d", count)
		}
	})

	t.Run("skip_invalid_nim", func(t *testing.T) {
		defer db.Exec("DELETE FROM logbook_entries WHERE id > 1")

		entries := []repository.BulkEntry{
			{Date: time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC), StudentName: "Valid NIM", NIM: "24091397010", TimeIn: "09:00", TimeOut: "10:00", Purpose: "Test"},
			{Date: time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC), StudentName: "Invalid NIM", NIM: "12345", TimeIn: "10:00", TimeOut: "11:00", Purpose: "Test"},
			{Date: time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC), StudentName: "Empty NIM", NIM: "", TimeIn: "11:00", TimeOut: "12:00", Purpose: "Test"},
		}
		saved, dups, err := svc.BulkSave(entries, "test.jpg", map[int]bool{}, 1, "admin", "admin", "127.0.0.1", "test")
		if err != nil {
			t.Fatalf("BulkSave: %v", err)
		}
		if saved != 1 {
			t.Errorf("expected 1 saved, got %d", saved)
		}
		if dups != 0 {
			t.Errorf("expected 0 duplicates, got %d", dups)
		}
	})

	t.Run("skip_duplicate_entries", func(t *testing.T) {
		defer db.Exec("DELETE FROM logbook_entries WHERE id > 1")

		entries := []repository.BulkEntry{
			{Date: now, StudentName: "Budi Santoso", NIM: "24091397001", TimeIn: "08:00", TimeOut: "10:00", Purpose: "Praktikum"},
			{Date: time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC), StudentName: "Siti Aminah", NIM: "24091397002", TimeIn: "09:00", TimeOut: "11:00", Purpose: "Praktikum Web"},
		}
		saved, dups, err := svc.BulkSave(entries, "test.jpg", map[int]bool{}, 1, "admin", "admin", "127.0.0.1", "test")
		if err != nil {
			t.Fatalf("BulkSave: %v", err)
		}
		if saved != 1 {
			t.Errorf("expected 1 saved, got %d", saved)
		}
		if dups != 1 {
			t.Errorf("expected 1 duplicate, got %d", dups)
		}
	})

	t.Run("verified_exact_dup_still_detected", func(t *testing.T) {
		defer db.Exec("DELETE FROM logbook_entries WHERE id > 1")

		entries := []repository.BulkEntry{
			{Date: now, StudentName: "Budi Santoso", NIM: "24091397001", TimeIn: "08:00", TimeOut: "10:00", Purpose: "Praktikum"},
		}
		saved, dups, err := svc.BulkSave(entries, "test.jpg", map[int]bool{0: true}, 1, "admin", "admin", "127.0.0.1", "test")
		if err != nil {
			t.Fatalf("BulkSave: %v", err)
		}
		if saved != 0 {
			t.Errorf("expected 0 saved (exact match is still dup), got %d", saved)
		}
		if dups != 1 {
			t.Errorf("expected 1 dup for exact match, got %d", dups)
		}
	})

	t.Run("verified_diff_time_not_dup", func(t *testing.T) {
		defer db.Exec("DELETE FROM logbook_entries WHERE id > 1")

		entries := []repository.BulkEntry{
			{Date: now, StudentName: "Budi Santoso", NIM: "24091397001", TimeIn: "10:00", TimeOut: "12:00", Purpose: "Praktikum"},
		}
		saved, dups, err := svc.BulkSave(entries, "test.jpg", map[int]bool{0: true}, 1, "admin", "admin", "127.0.0.1", "test")
		if err != nil {
			t.Fatalf("BulkSave: %v", err)
		}
		if saved != 1 {
			t.Errorf("expected 1 saved (different time, verified), got %d", saved)
		}
		if dups != 0 {
			t.Errorf("expected 0 dups for different time, got %d", dups)
		}
	})

	t.Run("all_duplicates_returns_zero_saved", func(t *testing.T) {
		defer db.Exec("DELETE FROM logbook_entries WHERE id > 1")

		entries := []repository.BulkEntry{
			{Date: now, StudentName: "Budi Santoso", NIM: "24091397001", TimeIn: "08:00", TimeOut: "10:00", Purpose: "Praktikum"},
			{Date: now, StudentName: "Budi Santoso", NIM: "24091397001", TimeIn: "08:00", TimeOut: "10:00", Purpose: "Praktikum"},
		}
		saved, dups, err := svc.BulkSave(entries, "test.jpg", map[int]bool{}, 1, "admin", "admin", "127.0.0.1", "test")
		if err != nil {
			t.Fatalf("BulkSave: %v", err)
		}
		if saved != 0 {
			t.Errorf("expected 0 saved (all duplicates), got %d", saved)
		}
		if dups != 2 {
			t.Errorf("expected 2 duplicates, got %d", dups)
		}
		_ = dups
	})
}
