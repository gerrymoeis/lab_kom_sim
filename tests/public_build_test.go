package tests

import (
	"os"
	"path/filepath"
	"testing"

	"inventaris-lab-kom/internal/config"
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/services"
)

func findRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("project root not found")
		}
		dir = parent
	}
}

func TestRunPublicBuild(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "public-build-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}

	projectRoot := findRoot(t)
	origWd, _ := os.Getwd()
	t.Cleanup(func() {
		os.Chdir(origWd)
	})
	os.Chdir(projectRoot)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := database.InitDB(dbPath, "")
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("InitDB: %v", err)
	}

	if err := database.RunMigrations(db, false, "TEST-1", "testlab", filepath.Join(tmpDir, "uploads"), false); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("RunMigrations: %v", err)
	}

	// Insert test PCs
	if _, err := db.Exec(`INSERT INTO pcs (row, column, status, label, placement) VALUES (1, 1, 'normal', 'pc-1', 'dipakai')`); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("insert pc-1: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO pcs (row, column, status, label, placement) VALUES (0, 0, 'normal', 'pc-cadangan', 'cadangan')`); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("insert pc-cadangan: %v", err)
	}

	// Insert test software
	if _, err := db.Exec(`INSERT INTO software_catalog (name, category, description, slug) VALUES ('Test Software', 'other', 'A test software entry', 'test-software')`); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("insert software: %v", err)
	}

	// Insert test schedule
	if _, err := db.Exec(`INSERT INTO course_schedules (course_name, lecturer, day, class, time_start, time_end) VALUES ('Pemrograman', 'Dosen A', 'Senin', 'A', '08:00', '09:40')`); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("insert schedule: %v", err)
	}

	outDir := filepath.Join(tmpDir, "dist")
	uploadPath := filepath.Join(tmpDir, "uploads")

	err = services.RunPublicBuild(db, config.PublicBuildConfig{
		TemplateDir: filepath.Join(projectRoot, "web", "templates", "public"),
		StaticDir:   filepath.Join(projectRoot, "web", "static"),
		OutDir:      outDir,
		Enabled:     true,
		Interval:    30,
		Branch:      "main",
	}, "testlab", "Test Lab", uploadPath)

	db.Close()

	if err != nil {
		t.Fatalf("RunPublicBuild: %v", err)
	}

	labDir := filepath.Join(outDir, "testlab")

	// Verify essential output files exist
	checkFile(t, labDir, "dashboard.html")
	checkFile(t, labDir, "index.html")
	checkFile(t, labDir, "pc", "list.html")
	checkFile(t, labDir, "pc", "detail", "pc-1.html")
	checkFile(t, labDir, "devices", "list.html")
	checkFile(t, labDir, "schedules", "list.html")
	checkFile(t, labDir, "data", "pc.json")
	checkFile(t, labDir, "data", "devices.json")
	checkFile(t, labDir, "data", "schedules.json")

	// Static files should be copied
	checkFile(t, filepath.Join(outDir, "static"), "css", "style.css")

	os.RemoveAll(tmpDir)
}

func checkFile(t *testing.T, parts ...string) {
	t.Helper()
	path := filepath.Join(parts...)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file not found: %s", path)
	}
}
