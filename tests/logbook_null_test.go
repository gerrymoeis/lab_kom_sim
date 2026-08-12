package tests

import (
	"path/filepath"
	"testing"
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/repository"
)

func newLogbookNullDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.InitDB(filepath.Join(t.TempDir(), "lb_null_test.db"), "")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	stmt := `CREATE TABLE IF NOT EXISTS logbook_entries (
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
	)`
	if _, err := db.Exec(stmt); err != nil {
		db.Close()
		t.Fatalf("create logbook_entries: %v", err)
	}
	return db
}

// Regression: logbook 500 saat time_out/purpose/source_file NULL (data ETL).
// Query harus memakai COALESCE agar scan ke string tidak gagal (doc 004).
func TestLogbookListNullNullableCols(t *testing.T) {
	db := newLogbookNullDB(t)
	defer db.Close()

	if _, err := db.Exec(`INSERT INTO logbook_entries (date, student_name, nim, time_in, time_out, purpose, source_file) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"2026-08-01", "Andi", "21010100001", "08:00", nil, nil, nil); err != nil {
		t.Fatalf("insert NULL row: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO logbook_entries (date, student_name, nim, time_in, time_out, purpose, source_file) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"2026-08-02", "Budi", "21010100002", "09:00", "12:00", "Praktikum", "log2.jpg"); err != nil {
		t.Fatalf("insert full row: %v", err)
	}

	repo := repository.NewLogbookRepository(db)

	entries, total, err := repo.List(repository.LogbookFilters{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List with NULL cols: %v", err)
	}
	if total != 2 {
		t.Fatalf("List total = %d, want 2", total)
	}
	found := false
	for _, e := range entries {
		if e.NIM == "21010100001" {
			found = true
			if e.TimeOut != "" || e.Purpose != "" || e.SourceFile != "" {
				t.Fatalf("NULL row harus jadi string kosong, got TimeOut=%q Purpose=%q SourceFile=%q", e.TimeOut, e.Purpose, e.SourceFile)
			}
		}
	}
	if !found {
		t.Fatalf("NULL row tidak ditemukan di hasil List")
	}

	all, err := repo.ListAll(repository.LogbookFilters{})
	if err != nil {
		t.Fatalf("ListAll with NULL cols: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("ListAll len = %d, want 2", len(all))
	}

	one, err := repo.GetByID(1)
	if err != nil {
		t.Fatalf("GetByID NULL row: %v", err)
	}
	if one.TimeOut != "" || one.Purpose != "" {
		t.Fatalf("GetByID NULL row TimeOut=%q Purpose=%q, want empty", one.TimeOut, one.Purpose)
	}

	exp, err := repo.Export(repository.ExportFilters{})
	if err != nil {
		t.Fatalf("Export with NULL cols: %v", err)
	}
	if len(exp) != 2 {
		t.Fatalf("Export len = %d, want 2", len(exp))
	}

	cur, hasMore, err := repo.ListCursor(repository.LogbookFilters{PageSize: 10})
	if err != nil {
		t.Fatalf("ListCursor with NULL cols: %v", err)
	}
	if hasMore || len(cur) != 2 {
		t.Fatalf("ListCursor len = %d hasMore=%v, want 2 false", len(cur), hasMore)
	}
}

// Pastikan duplikat tetap terdeteksi saat time_out NULL (GetDuplicateCheck tidak error).
func TestLogbookDuplicateCheckNullTimeOut(t *testing.T) {
	db := newLogbookNullDB(t)
	defer db.Close()

	if _, err := db.Exec(`INSERT INTO logbook_entries (date, student_name, nim, time_in) VALUES (?, ?, ?, ?)`,
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), "Andi", "21010100001", "08:00"); err != nil {
		t.Fatalf("insert NULL row: %v", err)
	}

	repo := repository.NewLogbookRepository(db)
	entries, err := repo.GetDuplicateCheck(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetDuplicateCheck with NULL row: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("GetDuplicateCheck len = %d, want 1", len(entries))
	}
}
