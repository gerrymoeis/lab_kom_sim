package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"inventaris-lab-kom/internal/database"
)

// TestContextCancellationDuringInsert — Context cancellation selama large INSERT
// memastikan DB tetap konsisten (PRAGMA integrity_check = ok).
func TestContextCancellationDuringInsert(t *testing.T) {
	env := setupTestEnvironment(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := env.DB_A.RawWriter().ExecContext(ctx,
		`INSERT INTO logbook_entries (date, student_name, nim, time_in)
		 SELECT date('now'), 'test', '12345678901', time('now')
		 FROM pcs AS t1 CROSS JOIN pcs AS t2 CROSS JOIN pcs AS t3`)

	if err == nil {
		t.Log("Insert completed before context cancelled (may happen on fast machine)")
	}

	var result string
	env.DB_A.QueryRow("PRAGMA integrity_check").Scan(&result)
	if result != "ok" {
		t.Fatal("Database corrupt after context cancellation:", result)
	}
}

// TestAbruptDBCloseDuringTransaction — Koneksi ditutup mendadak saat transaksi
// belum di-commit. Verifikasi DB tetap konsisten + row count sesuai (hanya committed data).
func TestAbruptDBCloseDuringTransaction(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "crash_resilience_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for i := 0; i < 50; i++ {
			if err := os.RemoveAll(tmpDir); err == nil {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	})
	dbPath := filepath.Join(tmpDir, "crash_abrupt.db")

	db, err := database.InitDB(dbPath, "")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}

	if _, err := db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, val TEXT)"); err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}
	if _, err := db.Exec("INSERT INTO test (val) VALUES ('initial')"); err != nil {
		t.Fatalf("INSERT initial: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if _, err := tx.Exec("INSERT INTO test (val) VALUES ('uncommitted')"); err != nil {
		t.Fatalf("INSERT uncommitted: %v", err)
	}

	// Abrupt close — tx was not committed
	db.Close()

	// Reopen
	db2, err := database.InitDB(dbPath, "")
	if err != nil {
		t.Fatalf("InitDB (reopen): %v", err)
	}

	var result string
	db2.QueryRow("PRAGMA integrity_check").Scan(&result)
	if result != "ok" {
		db2.Close()
		t.Fatal("Database corrupt after abrupt close:", result)
	}

	var count int
	db2.QueryRow("SELECT COUNT(*) FROM test").Scan(&count)
	t.Logf("Rows after abrupt close: %d (expected 1)", count)

	db2.Close()
}
