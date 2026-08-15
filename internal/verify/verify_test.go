package verify

import (
	"os"
	"path/filepath"
	"testing"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
)

// setupFixture membuat global DB + 1 lab DB realistis (migrasi + seed) dan
// struktur uploads, lalu mengembalikan config yang menunjuk ke sana.
func setupFixture(t *testing.T) (*config.Config, string) {
	t.Helper()
	dir := t.TempDir()

	uploadPath := filepath.Join(dir, "uploads")
	globalPath := filepath.Join(dir, "global.db")
	labPath := filepath.Join(dir, "lab_mi.db")

	gdb, err := database.InitDB(globalPath, "")
	if err != nil {
		t.Fatalf("InitDB global: %v", err)
	}
	defer gdb.Close()
	labs := []config.LabConfig{{ID: "MI-1", Title: "Lab Kom MI 1", DBPath: labPath, URLPath: "lab-mi", UploadDir: filepath.Join(uploadPath, "lab-mi")}}
	if err := database.SetupGlobalDB(gdb, labs); err != nil {
		t.Fatalf("SetupGlobalDB: %v", err)
	}

	ldb, err := database.InitDB(labPath, "")
	if err != nil {
		t.Fatalf("InitDB lab: %v", err)
	}
	defer ldb.Close()
	if err := database.RunMigrations(ldb, false, "MI-1", "lab-mi", uploadPath, true); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	if err := database.SeedDefaultUser(ldb); err != nil {
		t.Fatalf("SeedDefaultUser: %v", err)
	}

	for _, sub := range []string{"pc", "device_types", "device_installations", "logbook", "temp"} {
		if err := os.MkdirAll(filepath.Join(uploadPath, "lab-mi", sub), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}

	return &config.Config{
		GlobalDBPath: globalPath,
		UploadPath:   uploadPath,
		Labs:         labs,
	}, uploadPath
}

func TestRunSehat(t *testing.T) {
	cfg, _ := setupFixture(t)
	rep, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Failed {
		t.Fatalf("DB sehat harusnya lolos, tapi FAIL: %v", rep.Errors)
	}
	if rep.GlobalCounts["super_admin"] < 1 {
		t.Errorf("super_admin harus >=1, got %d", rep.GlobalCounts["super_admin"])
	}
	if len(rep.Labs) != 1 {
		t.Fatalf("harus 1 lab, got %d", len(rep.Labs))
	}
	for _, sub := range []string{"pc", "device_types", "device_installations", "logbook", "temp"} {
		found := false
		prefix := sub + "/ ok"
		for _, u := range rep.Labs[0].Uploads {
			if len(u) >= len(prefix) && u[:len(prefix)] == prefix {
				found = true
			}
		}
		if !found {
			t.Errorf("uploads %s harus ok", sub)
		}
	}
}

func TestRunOrphanTerdeteksi(t *testing.T) {
	cfg, _ := setupFixture(t)
	ldb, err := database.InitDB(cfg.Labs[0].DBPath, "")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer ldb.Close()
	// Matikan FK sementara agar orphan device_type_id bisa dimasukkan.
	if _, err := ldb.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable FK: %v", err)
	}
	// Insert orphan FK: device_type_id mengambang.
	if _, err := ldb.Exec(`INSERT INTO devices (serial_number, label, device_type_id) VALUES ('X-ORPHAN', 'X', 99999)`); err != nil {
		t.Fatalf("insert orphan: %v", err)
	}
	rep, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !rep.Failed {
		t.Fatal("orphan FK harusnya terdeteksi sebagai FAIL")
	}
}

func TestRunGlobalMissing(t *testing.T) {
	cfg, _ := setupFixture(t)
	// Global DB dihapus → harus fatal.
	os.Remove(cfg.GlobalDBPath)
	if _, err := Run(cfg); err == nil {
		t.Fatal("global DB hilang harusnya error")
	}
}