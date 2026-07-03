package tests

import (
	"os"
	"path/filepath"
	"testing"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/services"
)

func TestDummyNotifier(t *testing.T) {
	dn := services.DummyNotifier{}
	dn.NotifyChange()
}

func TestBackupServiceDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := database.InitDB(dbPath, "")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	svc := services.NewBackupService(db, config.BackupConfig{Enabled: false})
	err = svc.BackupNow()
	if err == nil {
		t.Fatal("expected error for disabled backup")
	}
}

func TestBackupServiceEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := database.InitDB(dbPath, "")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	backupDir := filepath.Join(tmpDir, "backups")
	svc := services.NewBackupService(db, config.BackupConfig{
		Enabled:   true,
		Interval:  10,
		Dir:       []string{backupDir},
		Retention: 3,
		Compress:  false,
	})

	err = svc.BackupNow()
	if err != nil {
		t.Fatalf("BackupNow: %v", err)
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least 1 backup file")
	}

	lastBackup, _ := svc.Stats()
	if lastBackup.IsZero() {
		t.Error("expected lastBackup to be set")
	}
}
