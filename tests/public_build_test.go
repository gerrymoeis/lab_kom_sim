package tests

import (
	"os"
	"path/filepath"
	"strings"
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
	defer os.RemoveAll(tmpDir)

	projectRoot := findRoot(t)
	origWd, _ := os.Getwd()
	t.Cleanup(func() {
		os.Chdir(origWd)
	})
	os.Chdir(projectRoot)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := database.InitDB(dbPath, "")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db, false, "TEST-1", "testlab", filepath.Join(tmpDir, "uploads"), false); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Insert test PCs
	if _, err := db.Exec(`INSERT INTO pcs (row, column, status, label, placement) VALUES (1, 1, 'normal', 'pc-1', 'dipakai')`); err != nil {
		t.Fatalf("insert pc-1: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO pcs (row, column, status, label, placement) VALUES (0, 0, 'normal', 'pc-cadangan', 'cadangan')`); err != nil {
		t.Fatalf("insert pc-cadangan: %v", err)
	}

	// Insert test software
	if _, err := db.Exec(`INSERT INTO software_catalog (name, category, description, slug) VALUES ('Test Software', 'other', 'A test software entry', 'test-software')`); err != nil {
		t.Fatalf("insert software: %v", err)
	}

	// Insert test schedule
	if _, err := db.Exec(`INSERT INTO course_schedules (course_name, lecturer, day, class, time_start, time_end) VALUES ('Pemrograman', 'Dosen A', 'Senin', 'A', '08:00', '09:40')`); err != nil {
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

	// Static files should be copied (shared for lab selector + per-lab)
	checkFile(t, filepath.Join(outDir, "static"), "css", "style.css")
	checkFile(t, labDir, "static", "css", "style.css")
	checkFile(t, labDir, "static", "vendor", "bootstrap", "css", "bootstrap.min.css")
	checkFile(t, labDir, "static", "js", "public.js")
}

func TestPublicBuildContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "public-build-content-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectRoot := findRoot(t)
	origWd, _ := os.Getwd()
	t.Cleanup(func() {
		os.Chdir(origWd)
	})
	os.Chdir(projectRoot)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := database.InitDB(dbPath, "")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	labName := "testlab"
	labTitle := "Test Lab"
	basePath := "/" + labName

	if err := database.RunMigrations(db, false, "TEST-1", labName, filepath.Join(tmpDir, "uploads"), false); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Insert comprehensive test data
	// PCs: normal, warning, broken, cadangan, special labels
	pcs := []struct {
		row, col  int
		status, label, placement, sn, os string
	}{
		{1, 1, "normal", "pc-1", "dipakai", "SN001", "Windows 11"},
		{1, 2, "normal", "pc-2", "dipakai", "SN002", "Windows 10"},
		{2, 1, "warning", "pc-3", "dipakai", "SN003", "Windows 11"},
		{0, 0, "normal", "pc-cadangan-1", "cadangan", "", ""},
		{0, 0, "broken", "pc-33", "cadangan", "", ""},
		{0, 0, "normal", "pc-dosen", "dipakai", "SN-DOSEN", "Windows 11"},
		{0, 0, "normal", "pc-laboran", "dipakai", "SN-LAB", "Windows 10"},
		{0, 0, "normal", "pc-cctv", "dipakai", "SN-CCTV", "Windows 11"},
	}
	for _, p := range pcs {
		if _, err := db.Exec(`INSERT INTO pcs (row, column, status, label, placement, serial_number, operating_system) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			p.row, p.col, p.status, p.label, p.placement, p.sn, p.os); err != nil {
			t.Fatalf("insert pc %s: %v", p.label, err)
		}
	}

	// Software catalog
	swNames := []string{"Visual Studio Code", "Python", "Blender"}
	for _, name := range swNames {
		slug := strings.ReplaceAll(strings.ToLower(name), " ", "-")
		if _, err := db.Exec(`INSERT INTO software_catalog (name, category, description, slug) VALUES (?, 'required', ?, ?)`,
			name, "Software "+name, slug); err != nil {
			t.Fatalf("insert software %s: %v", name, err)
		}
	}

	// Schedules
	if _, err := db.Exec(`INSERT INTO course_schedules (course_name, lecturer, day, class, time_start, time_end) VALUES ('Pemrograman', 'Dosen A', 'Senin', 'A', '08:00', '09:40')`); err != nil {
		t.Fatalf("insert schedule 1: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO course_schedules (course_name, lecturer, day, class, time_start, time_end) VALUES ('Jaringan', 'Dosen B', 'Selasa', 'B', '10:00', '11:40')`); err != nil {
		t.Fatalf("insert schedule 2: %v", err)
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
	}, labName, labTitle, uploadPath)

	if err != nil {
		t.Fatalf("RunPublicBuild: %v", err)
	}

	labDir := filepath.Join(outDir, labName)

	// ── helper to read a generated HTML file ──
	readHTML := func(parts ...string) string {
		t.Helper()
		path := filepath.Join(labDir, filepath.Join(parts...))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return string(data)
	}

	// ── 1. PATH CORRECTNESS ──
	t.Run("path_correctness", func(t *testing.T) {
		dash := readHTML("dashboard.html")

		// staticURL paths include basePath
		if !strings.Contains(dash, basePath+"/static/css/style.css") {
			t.Error("dashboard.html: missing basePath in static/css/style.css")
		}
		if !strings.Contains(dash, basePath+"/static/vendor/bootstrap/css/bootstrap.min.css") {
			t.Error("dashboard.html: missing basePath in bootstrap.min.css")
		}
		if !strings.Contains(dash, basePath+"/static/vendor/bootstrap-icons/bootstrap-icons.min.css") {
			t.Error("dashboard.html: missing basePath in bootstrap-icons.min.css")
		}
		if !strings.Contains(dash, basePath+"/static/js/public.js") {
			t.Error("dashboard.html: missing basePath in public.js")
		}

		// Nav links use basePath
		if !strings.Contains(dash, basePath+"/dashboard.html") {
			t.Error("dashboard.html: nav link missing basePath for dashboard")
		}
		if !strings.Contains(dash, basePath+"/pc/list.html") {
			t.Error("dashboard.html: nav link missing basePath for pc list")
		}
		if !strings.Contains(dash, basePath+"/devices/list.html") {
			t.Error("dashboard.html: nav link missing basePath for devices list")
		}
		if !strings.Contains(dash, basePath+"/software/list.html") {
			t.Error("dashboard.html: nav link missing basePath for software list")
		}
		if !strings.Contains(dash, basePath+"/schedules/list.html") {
			t.Error("dashboard.html: nav link missing basePath for schedules list")
		}

		// Lab selector link in navbar
		if !strings.Contains(dash, `href="/`) && !strings.Contains(dash, `href="./`) {
			// The "Semua Lab" button should link back to root or relative
			if !strings.Contains(dash, "Semua Lab") {
				t.Error("dashboard.html: missing Semua Lab nav button")
			}
		}

		// PC detail page paths
		pcDetail := readHTML("pc", "detail", "pc-1.html")
		if !strings.Contains(pcDetail, basePath+"/pc/list.html") {
			t.Error("pc/detail/pc-1.html: back link missing basePath")
		}
	})

	// ── 2. NO DATA LEAKS ──
	t.Run("no_data_leaks", func(t *testing.T) {
		pages := []string{"dashboard.html", "pc/list.html", "pc/detail/pc-1.html",
			"devices/list.html", "software/list.html", "schedules/list.html"}
		for _, page := range pages {
			html := readHTML(page)
			if strings.Contains(html, "csrf-token") {
				t.Errorf("%s: leaked CSRF token", page)
			}
			if strings.Contains(html, "main.js") {
				t.Errorf("%s: leaked main.js reference", page)
			}
			if strings.Contains(html, "delete_confirm_modal") {
				t.Errorf("%s: leaked admin delete modal", page)
			}
			if strings.Contains(html, "/admin/") {
				t.Errorf("%s: contains admin link", page)
			}
		}
	})

	// ── 3. PUBLIC_BASE_PATH IS DEFINED ──
	t.Run("public_base_path_defined", func(t *testing.T) {
		pages := []string{"dashboard.html", "pc/list.html", "devices/list.html",
			"software/list.html", "schedules/list.html"}
		for _, page := range pages {
			html := readHTML(page)
			expected := `var PUBLIC_BASE_PATH = "` + basePath + `"`
			if !strings.Contains(html, expected) {
				t.Errorf("%s: PUBLIC_BASE_PATH not defined correctly (expected var PUBLIC_BASE_PATH = %q)", page, basePath)
			}
		}
	})

	// ── 4. DATA PRESENCE ──
	t.Run("data_presence", func(t *testing.T) {
		// Dashboard: status counts
		dash := readHTML("dashboard.html")
		if !strings.Contains(dash, "pc-1") {
			t.Error("dashboard.html: pc-1 label not found")
		}
		if !strings.Contains(dash, "pc-dosen") {
			t.Error("dashboard.html: pc-dosen label not found")
		}
		if !strings.Contains(dash, "pc-laboran") {
			t.Error("dashboard.html: pc-laboran label not found")
		}
		if !strings.Contains(dash, "pc-cctv") {
			t.Error("dashboard.html: pc-cctv label not found")
		}
		if !strings.Contains(dash, "Cadangan") {
			t.Error("dashboard.html: cadangan stat card not found")
		}

		// PC list page: data JSON
		pcList := readHTML("pc", "list.html")
		if !strings.Contains(pcList, "pc-1") {
			t.Error("pc/list.html: pc-1 not found in dataJSON")
		}
		if !strings.Contains(pcList, "pc-cadangan-1") {
			t.Error("pc/list.html: pc-cadangan-1 not found in dataJSON")
		}

		// PC detail page
		pcDetail := readHTML("pc", "detail", "pc-1.html")
		if !strings.Contains(pcDetail, "pc-1") {
			t.Error("pc/detail/pc-1.html: label pc-1 not found")
		}
		if !strings.Contains(pcDetail, "SN001") {
			t.Error("pc/detail/pc-1.html: serial number SN001 not found")
		}
		if !strings.Contains(pcDetail, "bg-success") || !strings.Contains(pcDetail, "normal") {
			t.Error("pc/detail/pc-1.html: normal status badge not found")
		}

		// Broken PC detail page
		pc33Detail := readHTML("pc", "detail", "pc-33.html")
		if !strings.Contains(pc33Detail, "pc-33") {
			t.Error("pc/detail/pc-33.html: label not found")
		}
		if !strings.Contains(pc33Detail, "bg-danger") || !strings.Contains(pc33Detail, "broken") {
			t.Error("pc/detail/pc-33.html: broken status not found")
		}

		// Software list: data JSON
		swList := readHTML("software", "list.html")
		if !strings.Contains(swList, "Visual Studio Code") {
			t.Error("software/list.html: Visual Studio Code not found")
		}
		if !strings.Contains(swList, "Python") {
			t.Error("software/list.html: Python not found")
		}

		// Device list: empty data (no devices seed)
		devList := readHTML("devices", "list.html")
		if !strings.Contains(devList, "Daftar Perangkat") {
			t.Error("devices/list.html: title not found")
		}
		// Should not render errors for empty data
		if strings.Contains(devList, "error") || strings.Contains(devList, "Error") {
			t.Error("devices/list.html: rendered with error for empty data")
		}

		// Schedule list: data JSON
		schList := readHTML("schedules", "list.html")
		if !strings.Contains(schList, "Pemrograman") {
			t.Error("schedules/list.html: Pemrograman not found")
		}
		if !strings.Contains(schList, "Dosen A") {
			t.Error("schedules/list.html: Dosen A not found")
		}
	})

	// ── 5. DATA JSON FILES ──
	t.Run("data_json_files", func(t *testing.T) {
		pcData, err := os.ReadFile(filepath.Join(labDir, "data", "pc.json"))
		if err != nil {
			t.Fatalf("read pc.json: %v", err)
		}
		if !strings.Contains(string(pcData), "pc-1") {
			t.Error("data/pc.json: pc-1 not found")
		}
		if !strings.Contains(string(pcData), "pc-cadangan-1") {
			t.Error("data/pc.json: pc-cadangan-1 not found")
		}
		if !strings.Contains(string(pcData), "pc-dosen") {
			t.Error("data/pc.json: pc-dosen not found")
		}

		devData, err := os.ReadFile(filepath.Join(labDir, "data", "devices.json"))
		if err != nil {
			t.Fatalf("read devices.json: %v", err)
		}
		if string(devData) != "[]\n" && string(devData) != "[]" {
			t.Errorf("data/devices.json: expected empty array, got %s", string(devData))
		}

		swData, err := os.ReadFile(filepath.Join(labDir, "data", "software.json"))
		if err != nil {
			t.Fatalf("read software.json: %v", err)
		}
		if !strings.Contains(string(swData), "Visual Studio Code") {
			t.Error("data/software.json: Visual Studio Code not found")
		}

		schData, err := os.ReadFile(filepath.Join(labDir, "data", "schedules.json"))
		if err != nil {
			t.Fatalf("read schedules.json: %v", err)
		}
		if !strings.Contains(string(schData), "Pemrograman") {
			t.Error("data/schedules.json: Pemrograman not found")
		}
		if !strings.Contains(string(schData), "Dosen A") {
			t.Error("data/schedules.json: Dosen A not found")
		}
	})

	// ── 6. PER-LAB STATIC FILES ──
	t.Run("per_lab_static", func(t *testing.T) {
		// Per-lab static should have all essential files
		essentialStatic := []string{
			"css/style.css",
			"js/public.js",
			"vendor/bootstrap/css/bootstrap.min.css",
			"vendor/bootstrap/js/bootstrap.bundle.min.js",
			"vendor/bootstrap-icons/bootstrap-icons.min.css",
		}
		for _, rel := range essentialStatic {
			parts := strings.Split(rel, "/")
			checkFile(t, append([]string{labDir, "static"}, parts...)...)
		}

		// Shared static should also exist (for lab selector)
		checkFile(t, filepath.Join(outDir, "static"), "css", "style.css")
	})

	// ── 7. LAB SELECTOR ──
	t.Run("lab_selector", func(t *testing.T) {
		// GenerateLabSelector is tested separately via TestLabSelector,
		// but verify the dist/index.html can be produced
		cfg := config.PublicBuildConfig{
			TemplateDir: filepath.Join(projectRoot, "web", "templates", "public"),
			StaticDir:   filepath.Join(projectRoot, "web", "static"),
			OutDir:      outDir,
			Enabled:     true,
			Interval:    30,
		}
		labs := []config.LabConfig{
			{ID: "TEST-1", Title: labTitle, URLPath: labName},
		}
		if err := services.GenerateLabSelector(labs, cfg); err != nil {
			t.Fatalf("GenerateLabSelector: %v", err)
		}
		selectorPath := filepath.Join(outDir, "index.html")
		if _, err := os.Stat(selectorPath); os.IsNotExist(err) {
			t.Fatal("dist/index.html not generated")
		}
		data, err := os.ReadFile(selectorPath)
		if err != nil {
			t.Fatalf("read index.html: %v", err)
		}
		html := string(data)
		if !strings.Contains(html, labTitle) {
			t.Error("lab selector: lab title not found")
		}
		if !strings.Contains(html, labName) {
			t.Error("lab selector: lab URLPath not found")
		}
		if !strings.Contains(html, "/"+labName+"/dashboard.html") {
			t.Error("lab selector: link to lab dashboard not found")
		}
	})
}

func checkFile(t *testing.T, parts ...string) {
	t.Helper()
	path := filepath.Join(parts...)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file not found: %s", path)
	}
}
