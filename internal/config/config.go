package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds application configuration
type Config struct {
	Environment      string
	Host             string
	Port             string
	Labs             []LabConfig
	MultiLabMode     bool   // true if LABS= or LABS_<N>_* is used (multi-lab), false if only DATABASE_PATH (single-lab)
	DatabaseURL      string
	SessionSecret    string
	CookieSecure     bool
	UploadPath       string
	GlobalDBPath     string
	EnvPath          string
	GeminiAPIKey     string
	OpenRouterAPIKey string
	Android          bool
	WriteMode        string
	Timezone         string
	DefaultPageSize   int
	LogRetentionDays  int
	LogCleanupInterval int
	Backup            BackupConfig
	PublicBuild       PublicBuildConfig
}

// BackupConfig holds SQLite auto-backup configuration
type BackupConfig struct {
	Enabled   bool
	Interval  int
	Dir       []string
	Retention int
	MinDiskMB int64
	Compress  bool
}

// PublicBuildConfig holds SSG auto-build configuration
type PublicBuildConfig struct {
	Enabled      bool
	Interval     int
	OutDir       string
	TemplateDir  string
	StaticDir    string
	RepoDir      string
	Branch       string
}

// Load loads configuration from environment variables with defaults
func Load() *Config {
	// Load .env file if exists (so ENV_PATH can be defined inside it)
	envPath := ".env"
	if err := godotenv.Load(envPath); err != nil {
		log.Println("Warning: .env file not found, using environment variables or defaults")
	}

	// EnvPath is configurable via ENV_PATH env var (set in /opt/simlab/.env on production via systemd)
	// Fallback to ".env" and resolve to absolute path for reliability
	envPath = getEnv("ENV_PATH", ".env")
	if absEnvPath, err := filepath.Abs(envPath); err == nil {
		envPath = absEnvPath
	}

	dbPath := getEnv("DATABASE_PATH", "inventaris_lab.db")
	uploadPath := getEnv("UPLOAD_PATH", "uploads")
	var labs []LabConfig
	multiLabMode := false

	// Try new format first (LABS_<N>_*)
	v2Labs, found := parseLabsV2(uploadPath, dbPath)
	if found {
		labs = v2Labs
		multiLabMode = true
		log.Printf("Loaded %d lab(s) from LABS_<N>_* format", len(labs))
	} else {
		// Fallback: old format (LABS=...)
		labsEnv := getEnv("LABS", "")
		if labsEnv != "" {
			labs = parseLabs(labsEnv, uploadPath, dbPath)
			multiLabMode = true
			log.Println("Warning: LABS= (old format) is deprecated. Use LABS_<N>_* format instead.")
		} else {
			labs = []LabConfig{defaultLab(dbPath, uploadPath)}
		}
	}

	if len(labs) == 0 {
		log.Println("Warning: no labs configured, using default")
		labs = []LabConfig{defaultLab(dbPath, uploadPath)}
	}

	return &Config{
		Environment:   getEnv("ENVIRONMENT", "development"),
		Host:          getEnv("HOST", "0.0.0.0"),
		Port:          getEnv("PORT", "8080"),
		Labs:          labs,
		MultiLabMode:  multiLabMode,
		EnvPath:       envPath,
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		SessionSecret: getEnv("SESSION_SECRET", "change-this-secret-in-production"),
		CookieSecure:  getEnv("COOKIE_SECURE", "false") == "true",
		UploadPath:    uploadPath,
		GlobalDBPath:  getEnv("GLOBAL_DB_PATH", "data/global.db"),
		GeminiAPIKey:     getEnv("GEMINI_API_KEY", ""),
		OpenRouterAPIKey: getEnv("OPENROUTER_API_KEY", ""),
		Android:          getEnv("ANDROID", "false") == "true",
		WriteMode:        getEnv("WRITE_MODE", "sync"),
		Timezone:         getEnv("TIMEZONE", "Asia/Jakarta"),
		DefaultPageSize:    getEnvInt("DEFAULT_PAGE_SIZE", 25),
		LogRetentionDays:   getEnvInt("LOG_RETENTION_DAYS", 90),
		LogCleanupInterval: getEnvInt("LOG_CLEANUP_INTERVAL", 24),
		PublicBuild: PublicBuildConfig{
			Enabled:     getEnv("PUBLIC_BUILD_ENABLED", "false") == "true",
			Interval:    getEnvInt("PUBLIC_BUILD_INTERVAL", 30),
			OutDir:      getEnv("PUBLIC_BUILD_OUT", "dist"),
			TemplateDir: getEnv("PUBLIC_BUILD_TEMPLATE_DIR", "web/templates/public"),
			StaticDir:   getEnv("PUBLIC_BUILD_STATIC_DIR", "web/static"),
			RepoDir:     getEnv("PUBLIC_BUILD_REPO_DIR", ""),
			Branch:      getEnv("PUBLIC_BUILD_BRANCH", "main"),
		},
		Backup: BackupConfig{
			Enabled:   getEnv("BACKUP_ENABLED", "true") == "true",
			Interval:  getEnvInt("BACKUP_INTERVAL", 30),
			Dir:       parseDirs(getEnv("BACKUP_DIR", "./backups")),
			Retention: getEnvInt("BACKUP_RETENTION", 20),
			MinDiskMB: int64(getEnvInt("BACKUP_MIN_DISK_MB", 500)),
			Compress:  getEnv("BACKUP_COMPRESS", "true") == "true",
		},
	}
}

// parseLabs splits LABS env into LabConfig slice
// Format: LAB-ID:dbPath[:Title[:urlPath]],...
// Example: MI-1:data/lab_mi_1.db:Lab Kom MI 1:lab-kom-mi,VOKASI-1:data/lab_vokasi_1.db:Lab Kom Vokasi:vokasi
func parseLabs(raw, uploadPath, fallbackDBPath string) []LabConfig {
	parts := strings.Split(raw, ",")
	labs := make([]LabConfig, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		segments := strings.SplitN(p, ":", 4)
		if len(segments) < 2 || segments[0] == "" || segments[1] == "" {
			log.Printf("Warning: skipping invalid LABS entry: %q", p)
			continue
		}
		labID := segments[0]
		dbPath := segments[1]
		title := labTitleFromName(labID)
		if len(segments) > 2 && segments[2] != "" {
			title = segments[2]
		}
		urlPath := strings.ToLower(labID)
		if len(segments) > 3 && segments[3] != "" {
			urlPath = segments[3]
		}
		uploadDir := filepath.Join(uploadPath, urlPath)
		labs = append(labs, LabConfig{
			ID:        labID,
			Title:     title,
			DBPath:    dbPath,
			URLPath:   urlPath,
			UploadDir: uploadDir,
			Layout:    GetGridLayout(urlPath),
		})
	}
	if len(labs) == 0 {
		log.Println("Warning: LABS env set but no valid entries, using default lab")
		return []LabConfig{defaultLab(fallbackDBPath, uploadPath)}
	}
	return labs
}

// parseLabsV2 reads LABS_<N>_* env vars into LabConfig slice
// Format: LABS_<N>_ID=<id>, LABS_<N>_DB=<path>, LABS_<N>_TITLE=<title>, LABS_<N>_URL=<slug>
// Returns labs slice and whether any V2 format vars were found
func parseLabsV2(uploadPath, fallbackDBPath string) ([]LabConfig, bool) {
	type labDef struct {
		ID, DBPath, Title, URLPath string
		Index                      int
	}
	labMap := make(map[int]*labDef)

	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "LABS_") {
			continue
		}
		eqIdx := strings.Index(env, "=")
		if eqIdx < 0 {
			continue
		}
		key := env[:eqIdx]
		value := env[eqIdx+1:]

		if !strings.HasSuffix(key, "_ID") && !strings.HasSuffix(key, "_DB") &&
			!strings.HasSuffix(key, "_TITLE") && !strings.HasSuffix(key, "_URL") {
			continue
		}
		// Extract N from "LABS_<N>_SUFFIX"
		trimmed := strings.TrimPrefix(key, "LABS_")
		lastUnderscore := strings.LastIndex(trimmed, "_")
		if lastUnderscore < 0 {
			continue
		}
		nStr := trimmed[:lastUnderscore]
		n, err := strconv.Atoi(nStr)
		if err != nil || n <= 0 {
			continue
		}
		suffix := trimmed[lastUnderscore+1:]

		if labMap[n] == nil {
			labMap[n] = &labDef{Index: n}
		}
		switch suffix {
		case "ID":
			labMap[n].ID = value
		case "DB":
			labMap[n].DBPath = value
		case "TITLE":
			labMap[n].Title = value
		case "URL":
			labMap[n].URLPath = value
		}
	}

	if len(labMap) == 0 {
		return nil, false
	}

	labs := make([]LabConfig, 0, len(labMap))
	for _, def := range labMap {
		if def.ID == "" || def.DBPath == "" {
			log.Printf("Warning: LABS_%d missing ID or DB, skipping", def.Index)
			continue
		}
		title := def.Title
		if title == "" {
			title = labTitleFromName(def.ID)
		}
		urlPath := def.URLPath
		if urlPath == "" {
			urlPath = strings.ToLower(def.ID)
		}
		uploadDir := filepath.Join(uploadPath, urlPath)
		labs = append(labs, LabConfig{
			ID:        def.ID,
			Title:     title,
			DBPath:    def.DBPath,
			URLPath:   urlPath,
			UploadDir: uploadDir,
			Layout:    GetGridLayout(urlPath),
			EnvIndex:  def.Index,
		})
	}
	return labs, true
}

// CommentOutLabEnv comments out 4 LABS_<N>_* lines in the .env file
func CommentOutLabEnv(envPath string, n int) error {
	data, err := os.ReadFile(envPath)
	if err != nil {
		return fmt.Errorf("gagal baca %s: %w", envPath, err)
	}

	lines := strings.Split(string(data), "\n")
	modified := false
	prefixes := []string{
		fmt.Sprintf("LABS_%d_ID=", n),
		fmt.Sprintf("LABS_%d_DB=", n),
		fmt.Sprintf("LABS_%d_TITLE=", n),
		fmt.Sprintf("LABS_%d_URL=", n),
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(trimmed, prefix) {
				lines[i] = "#" + line
				modified = true
				break
			}
		}
	}

	if !modified {
		return fmt.Errorf("tidak menemukan LABS_%d_* di %s", n, envPath)
	}

	return os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0644)
}

// AppendLabEnv appends 4 LABS_<N+1>_* lines to the .env file
// Scans all lines (including commented) to find the highest N
// Returns the new N index assigned to this lab
func AppendLabEnv(envPath string, lab LabConfig) (int, error) {
	data, err := os.ReadFile(envPath)
	if err != nil {
		return 0, fmt.Errorf("gagal baca %s: %w", envPath, err)
	}

	lines := strings.Split(string(data), "\n")

	// Find highest N from ALL lines (active + commented)
	maxN := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "#")
		if !strings.HasPrefix(trimmed, "LABS_") {
			continue
		}
		// Extract N from "LABS_<N>_SUFFIX=..."
		rest := strings.TrimPrefix(trimmed, "LABS_")
		underscoreIdx := strings.Index(rest, "_")
		if underscoreIdx < 0 {
			continue
		}
		n, err := strconv.Atoi(rest[:underscoreIdx])
		if err != nil || n <= 0 {
			continue
		}
		if n > maxN {
			maxN = n
		}
	}
	newN := maxN + 1

	// Ensure file ends with newline
	lastLine := ""
	if len(lines) > 0 {
		lastLine = lines[len(lines)-1]
	}
	if lastLine != "" {
		lines = append(lines, "")
	}

	newLines := []string{
		fmt.Sprintf("LABS_%d_ID=%s", newN, lab.ID),
		fmt.Sprintf("LABS_%d_DB=%s", newN, lab.DBPath),
		fmt.Sprintf("LABS_%d_TITLE=%s", newN, lab.Title),
		fmt.Sprintf("LABS_%d_URL=%s", newN, lab.URLPath),
	}
	lines = append(lines, newLines...)

	return newN, os.WriteFile(envPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func defaultLab(dbPath, uploadPath string) LabConfig {
	id := labNameFromPath(dbPath)
	title := labTitleFromName(id)
	urlPath := strings.ToLower(id)
	uploadDir := filepath.Join(uploadPath, urlPath)
	return LabConfig{
		ID:        id,
		Title:     title,
		DBPath:    dbPath,
		URLPath:   urlPath,
		UploadDir: uploadDir,
		Layout:    GetGridLayout(urlPath),
	}
}

func labNameFromPath(dbPath string) string {
	base := filepath.Base(dbPath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if base == "" {
		return "lab"
	}
	return base
}

func labTitleFromName(name string) string {
	t := strings.ReplaceAll(name, "_", " ")
	t = strings.ReplaceAll(t, "-", " ")
	t = strings.TrimSpace(t)
	if t == "" {
		return "Laboratorium Komputer"
	}
	words := strings.Fields(t)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// parseDirs splits comma-separated directory paths, trimming whitespace and quotes
func parseDirs(raw string) []string {
	parts := strings.Split(raw, ",")
	dirs := make([]string, 0, len(parts))
	for _, p := range parts {
		d := strings.TrimSpace(p)
		d = strings.Trim(d, `"'`)
		if d != "" {
			dirs = append(dirs, d)
		}
	}
	if len(dirs) == 0 {
		return []string{"./backups"}
	}
	return dirs
}

func (c *Config) LabLayout(labName string) GridLayout {
	return GetGridLayout(labName)
}

// getEnv gets environment variable with fallback to default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets environment variable as integer with fallback
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var i int
		if _, err := fmt.Sscanf(value, "%d", &i); err == nil {
			return i
		}
	}
	return defaultValue
}
