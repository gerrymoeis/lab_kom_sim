package config

import (
	"os"
	"path/filepath"
	"testing"
)

// saveEnv saves current values of env vars for later restoration
func saveEnv(keys ...string) map[string]string {
	saved := make(map[string]string, len(keys))
	for _, k := range keys {
		saved[k] = os.Getenv(k)
	}
	return saved
}

// restoreEnv restores env vars to saved values
func restoreEnv(saved map[string]string) {
	for k, v := range saved {
		if v == "" {
			os.Unsetenv(k)
		} else {
			os.Setenv(k, v)
		}
	}
}

// setEnv sets multiple env vars at once
func setEnv(vars map[string]string) {
	for k, v := range vars {
		os.Setenv(k, v)
	}
}

func TestParseLabsV2_MultiLab(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("LABS_1_ID")
		os.Unsetenv("LABS_1_DB")
		os.Unsetenv("LABS_1_TITLE")
		os.Unsetenv("LABS_1_URL")
		os.Unsetenv("LABS_2_ID")
		os.Unsetenv("LABS_2_DB")
		os.Unsetenv("LABS_2_TITLE")
		os.Unsetenv("LABS_2_URL")
	})

	os.Setenv("LABS_1_ID", "MI-1")
	os.Setenv("LABS_1_DB", "data/lab_mi_1.db")
	os.Setenv("LABS_1_TITLE", "Lab Kom MI 1")
	os.Setenv("LABS_1_URL", "lab-kom-mi")
	os.Setenv("LABS_2_ID", "VOKASI-1")
	os.Setenv("LABS_2_DB", "data/lab_vokasi_1.db")
	os.Setenv("LABS_2_TITLE", "Lab Kom Vokasi")
	os.Setenv("LABS_2_URL", "vokasi")

	labs, found := parseLabsV2("uploads", "inventaris_lab.db")
	if !found {
		t.Fatal("expected parseLabsV2 to find LABS_* vars")
	}
	if len(labs) != 2 {
		t.Fatalf("expected 2 labs, got %d", len(labs))
	}

	lab1 := labs[0]
	if lab1.ID != "MI-1" {
		t.Errorf("lab[0].ID: expected MI-1, got %s", lab1.ID)
	}
	if lab1.DBPath != "data/lab_mi_1.db" {
		t.Errorf("lab[0].DBPath: expected data/lab_mi_1.db, got %s", lab1.DBPath)
	}
	if lab1.URLPath != "lab-kom-mi" {
		t.Errorf("lab[0].URLPath: expected lab-kom-mi, got %s", lab1.URLPath)
	}
	if lab1.Title != "Lab Kom MI 1" {
		t.Errorf("lab[0].Title: expected Lab Kom MI 1, got %s", lab1.Title)
	}
	expectedUploadDir1 := filepath.Join("uploads", "lab-kom-mi")
	if lab1.UploadDir != expectedUploadDir1 {
		t.Errorf("lab[0].UploadDir: expected %s, got %s", expectedUploadDir1, lab1.UploadDir)
	}
	if lab1.EnvIndex != 1 {
		t.Errorf("lab[0].EnvIndex: expected 1, got %d", lab1.EnvIndex)
	}
	if len(lab1.Layout.ColsPerRow) == 0 {
		t.Error("lab[0].Layout.ColsPerRow is empty")
	}

	lab2 := labs[1]
	if lab2.ID != "VOKASI-1" {
		t.Errorf("lab[1].ID: expected VOKASI-1, got %s", lab2.ID)
	}
	if lab2.DBPath != "data/lab_vokasi_1.db" {
		t.Errorf("lab[1].DBPath: expected data/lab_vokasi_1.db, got %s", lab2.DBPath)
	}
	if lab2.URLPath != "vokasi" {
		t.Errorf("lab[1].URLPath: expected vokasi, got %s", lab2.URLPath)
	}
	if lab2.Title != "Lab Kom Vokasi" {
		t.Errorf("lab[1].Title: expected Lab Kom Vokasi, got %s", lab2.Title)
	}
	expectedUploadDir2 := filepath.Join("uploads", "vokasi")
	if lab2.UploadDir != expectedUploadDir2 {
		t.Errorf("lab[1].UploadDir: expected %s, got %s", expectedUploadDir2, lab2.UploadDir)
	}
	if lab2.EnvIndex != 2 {
		t.Errorf("lab[1].EnvIndex: expected 2, got %d", lab2.EnvIndex)
	}
}

func TestParseLabsV2_DefaultTitleAndURL(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("LABS_1_ID")
		os.Unsetenv("LABS_1_DB")
	})

	// Only set ID and DB — Title and URL should use defaults
	os.Setenv("LABS_1_ID", "MI-1")
	os.Setenv("LABS_1_DB", "data/lab_mi_1.db")

	labs, found := parseLabsV2("uploads", "inventaris_lab.db")
	if !found {
		t.Fatal("expected parseLabsV2 to find LABS_* vars")
	}
	if len(labs) != 1 {
		t.Fatalf("expected 1 lab, got %d", len(labs))
	}

	if labs[0].Title != "MI 1" {
		t.Errorf("expected default title 'MI 1', got %s", labs[0].Title)
	}
	if labs[0].URLPath != "mi-1" {
		t.Errorf("expected default URL 'mi-1', got %s", labs[0].URLPath)
	}
}

func TestParseLabsV2_NoVars(t *testing.T) {
	labs, found := parseLabsV2("uploads", "inventaris_lab.db")
	if found {
		t.Error("expected found=false when no LABS_* vars set")
	}
	if labs != nil {
		t.Errorf("expected nil labs, got %v", labs)
	}
}

func TestLoad_SessionSecret(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("SESSION_SECRET")
	})

	os.Setenv("SESSION_SECRET", "test-session-secret-123")
	cfg := Load()
	if cfg.SessionSecret != "test-session-secret-123" {
		t.Errorf("expected SessionSecret 'test-session-secret-123', got %s", cfg.SessionSecret)
	}
}

func TestLoad_SessionSecretDefault(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("SESSION_SECRET")
	})

	os.Unsetenv("SESSION_SECRET")
	cfg := Load()
	if cfg.SessionSecret != "change-this-secret-in-production" {
		t.Errorf("expected default SessionSecret, got %s", cfg.SessionSecret)
	}
}

func TestLoad_EnvPath(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("ENV_PATH")
		os.Unsetenv("LABS_1_ID")
		os.Unsetenv("LABS_1_DB")
	})

	os.Setenv("ENV_PATH", "/opt/simlab/.env.production")
	os.Setenv("LABS_1_ID", "MI-1")
	os.Setenv("LABS_1_DB", "data/lab.db")

	cfg := Load()
	expectedPath, _ := filepath.Abs("/opt/simlab/.env.production")
	if cfg.EnvPath != expectedPath {
		t.Errorf("expected EnvPath %s, got %s", expectedPath, cfg.EnvPath)
	}
}

func TestLoad_EnvPathDefault(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("ENV_PATH")
		os.Unsetenv("LABS_1_ID")
		os.Unsetenv("LABS_1_DB")
	})

	os.Unsetenv("ENV_PATH")
	os.Setenv("LABS_1_ID", "MI-1")
	os.Setenv("LABS_1_DB", "data/lab.db")

	cfg := Load()
	if cfg.EnvPath == "" {
		t.Error("EnvPath should not be empty")
	}
	// Should be an absolute path ending with ".env"
	if filepath.Base(cfg.EnvPath) != ".env" {
		t.Errorf("expected EnvPath to end with '.env', got %s", cfg.EnvPath)
	}
}

func TestLoad_DatabaseURL(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("LABS_1_ID")
		os.Unsetenv("LABS_1_DB")
	})

	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/simlab")
	os.Setenv("LABS_1_ID", "MI-1")
	os.Setenv("LABS_1_DB", "data/lab.db")

	cfg := Load()
	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/simlab" {
		t.Errorf("expected DatabaseURL 'postgres://...', got %s", cfg.DatabaseURL)
	}
}

func TestLoad_DatabaseURLDefault(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("LABS_1_ID")
		os.Unsetenv("LABS_1_DB")
	})

	os.Unsetenv("DATABASE_URL")
	os.Setenv("LABS_1_ID", "MI-1")
	os.Setenv("LABS_1_DB", "data/lab.db")

	cfg := Load()
	if cfg.DatabaseURL != "" {
		t.Errorf("expected empty DatabaseURL, got %s", cfg.DatabaseURL)
	}
}

func TestParseLabsV2_InvalidMissingID(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("LABS_1_DB")
		os.Unsetenv("LABS_2_ID")
		os.Unsetenv("LABS_2_DB")
	})

	// Lab 1 has DB but no ID → should be skipped
	os.Setenv("LABS_1_DB", "data/lab_mi_1.db")
	// Lab 2 is valid
	os.Setenv("LABS_2_ID", "VOKASI-1")
	os.Setenv("LABS_2_DB", "data/lab_vokasi_1.db")

	labs, found := parseLabsV2("uploads", "inventaris_lab.db")
	if !found {
		t.Fatal("expected parseLabsV2 to find LABS_* vars")
	}
	if len(labs) != 1 {
		t.Fatalf("expected 1 valid lab (skipping LABS_1 missing ID), got %d", len(labs))
	}
	if labs[0].ID != "VOKASI-1" {
		t.Errorf("expected remaining lab ID VOKASI-1, got %s", labs[0].ID)
	}
}

func TestParseLabsV2_InvalidMissingDB(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("LABS_1_ID")
		os.Unsetenv("LABS_2_ID")
		os.Unsetenv("LABS_2_DB")
	})

	// Lab 1 has ID but no DB → should be skipped
	os.Setenv("LABS_1_ID", "MI-1")
	// Lab 2 is valid
	os.Setenv("LABS_2_ID", "VOKASI-1")
	os.Setenv("LABS_2_DB", "data/lab_vokasi_1.db")

	labs, found := parseLabsV2("uploads", "inventaris_lab.db")
	if !found {
		t.Fatal("expected parseLabsV2 to find LABS_* vars")
	}
	if len(labs) != 1 {
		t.Fatalf("expected 1 valid lab (skipping LABS_1 missing DB), got %d", len(labs))
	}
	if labs[0].ID != "VOKASI-1" {
		t.Errorf("expected remaining lab ID VOKASI-1, got %s", labs[0].ID)
	}
}

func TestParseLabsV2_InvalidNonNumericN(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("LABS_ABC_ID")
		os.Unsetenv("LABS_ABC_DB")
		os.Unsetenv("LABS_1_ID")
		os.Unsetenv("LABS_1_DB")
	})

	// Lab with non-numeric N → should be skipped
	os.Setenv("LABS_ABC_ID", "MI-1")
	os.Setenv("LABS_ABC_DB", "data/lab.db")
	// Valid lab
	os.Setenv("LABS_1_ID", "VOKASI-1")
	os.Setenv("LABS_1_DB", "data/lab_vokasi.db")

	labs, found := parseLabsV2("uploads", "inventaris_lab.db")
	if !found {
		t.Fatal("expected parseLabsV2 to find LABS_* vars")
	}
	if len(labs) != 1 {
		t.Fatalf("expected 1 valid lab (skipping non-numeric N), got %d", len(labs))
	}
}

func TestParseLabsV2_InvalidNZero(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("LABS_0_ID")
		os.Unsetenv("LABS_0_DB")
		os.Unsetenv("LABS_1_ID")
		os.Unsetenv("LABS_1_DB")
	})

	// Lab with N=0 → should be skipped (n <= 0)
	os.Setenv("LABS_0_ID", "MI-1")
	os.Setenv("LABS_0_DB", "data/lab.db")
	// Valid lab
	os.Setenv("LABS_1_ID", "VOKASI-1")
	os.Setenv("LABS_1_DB", "data/lab_vokasi.db")

	labs, found := parseLabsV2("uploads", "inventaris_lab.db")
	if !found {
		t.Fatal("expected parseLabsV2 to find LABS_* vars")
	}
	if len(labs) != 1 {
		t.Fatalf("expected 1 valid lab (skipping N=0), got %d", len(labs))
	}
}

func TestParseLabsV2_UnknownSuffix(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("LABS_1_ID")
		os.Unsetenv("LABS_1_DB")
		os.Unsetenv("LABS_1_XYZ")
	})

	// LABS_1_XYZ is not a recognized suffix → should be ignored, but valid ID+DB still work
	os.Setenv("LABS_1_ID", "MI-1")
	os.Setenv("LABS_1_DB", "data/lab.db")
	os.Setenv("LABS_1_XYZ", "something")

	labs, found := parseLabsV2("uploads", "inventaris_lab.db")
	if !found {
		t.Fatal("expected parseLabsV2 to find LABS_* vars")
	}
	if len(labs) != 1 {
		t.Fatalf("expected 1 lab, got %d", len(labs))
	}
	if labs[0].ID != "MI-1" {
		t.Errorf("expected lab ID MI-1, got %s", labs[0].ID)
	}
}

func TestLoad_MultiLabMode(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("LABS_1_ID")
		os.Unsetenv("LABS_1_DB")
		os.Unsetenv("LABS_1_TITLE")
		os.Unsetenv("LABS_1_URL")
	})

	os.Setenv("LABS_1_ID", "MI-1")
	os.Setenv("LABS_1_DB", "data/lab.db")
	os.Setenv("LABS_1_TITLE", "Lab Kom MI")
	os.Setenv("LABS_1_URL", "lab-kom-mi")

	cfg := Load()
	if !cfg.MultiLabMode {
		t.Error("expected MultiLabMode=true when LABS_<N>_* vars set")
	}
	if len(cfg.Labs) != 1 {
		t.Fatalf("expected 1 lab, got %d", len(cfg.Labs))
	}
	if cfg.Labs[0].ID != "MI-1" {
		t.Errorf("expected lab ID MI-1, got %s", cfg.Labs[0].ID)
	}
}

func TestLoad_DefaultLab(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("DATABASE_PATH")
	})

	// No LABS_* vars, no LABS= vars → should create default lab
	os.Setenv("DATABASE_PATH", "test_inventaris.db")

	cfg := Load()
	if cfg.MultiLabMode {
		t.Error("expected MultiLabMode=false for default single lab")
	}
	if len(cfg.Labs) != 1 {
		t.Fatalf("expected 1 default lab, got %d", len(cfg.Labs))
	}
	if cfg.Labs[0].DBPath != "test_inventaris.db" {
		t.Errorf("expected default lab DBPath 'test_inventaris.db', got %s", cfg.Labs[0].DBPath)
	}
}

func TestGetEnvInt(t *testing.T) {
	t.Cleanup(func() {
		os.Unsetenv("TEST_INT")
	})

	tests := []struct {
		name     string
		value    string
		expected int
	}{
		{"valid_int", "42", 42},
		{"zero", "0", 0},
		{"negative", "-5", -5},
		{"invalid", "abc", 10},
		{"empty", "", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value == "" {
				os.Unsetenv("TEST_INT")
			} else {
				os.Setenv("TEST_INT", tt.value)
			}
			result := getEnvInt("TEST_INT", 10)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestLabTitleFromName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"mi-1", "Mi 1"},
		{"VOKASI-1", "VOKASI 1"},
		{"lab_kom_mi", "Lab Kom Mi"},
		{"", "Laboratorium Komputer"},
		{"single", "Single"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := labTitleFromName(tt.input)
			if result != tt.expected {
				t.Errorf("labTitleFromName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLabNameFromPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"data/lab_mi_1.db", "lab_mi_1"},
		{"lab.db", "lab"},
		{"/path/to/custom.db", "custom"},
		{"", "lab"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := labNameFromPath(tt.input)
			if result != tt.expected {
				t.Errorf("labNameFromPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseDirs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"single", "./backups", []string{"./backups"}},
		{"multiple", "./backups,/tmp/backup", []string{"./backups", "/tmp/backup"}},
		{"with_quotes", `"./backups","./extra"`, []string{"./backups", "./extra"}},
		{"empty", "", []string{"./backups"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDirs(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d dirs, got %d: %v", len(tt.expected), len(result), result)
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("dir[%d]: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestDefaultLab(t *testing.T) {
	lab := defaultLab("data/test_lab.db", "uploads")
	if lab.ID != "test_lab" {
		t.Errorf("expected ID 'test_lab', got %s", lab.ID)
	}
	if lab.DBPath != "data/test_lab.db" {
		t.Errorf("expected DBPath 'data/test_lab.db', got %s", lab.DBPath)
	}
	if lab.URLPath != "test_lab" {
		t.Errorf("expected URLPath 'test_lab', got %s", lab.URLPath)
	}
	expectedUploadDir := filepath.Join("uploads", "test_lab")
	if lab.UploadDir != expectedUploadDir {
		t.Errorf("expected UploadDir %s, got %s", expectedUploadDir, lab.UploadDir)
	}
	if lab.EnvIndex != 0 {
		t.Errorf("expected EnvIndex 0 for default lab, got %d", lab.EnvIndex)
	}
}
