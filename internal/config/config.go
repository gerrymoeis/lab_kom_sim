package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds application configuration
type Config struct {
	Environment      string
	Host             string
	Port             string
	Labs             []LabConfig
	DatabaseURL      string
	SessionSecret    string
	CookieSecure     bool
	UploadPath       string
	GlobalDBPath     string
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
	// Load .env file if exists
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables or defaults")
	}

	dbPath := getEnv("DATABASE_PATH", "inventaris_lab.db")
	uploadPath := getEnv("UPLOAD_PATH", "uploads")
	labsEnv := getEnv("LABS", "")
	var labs []LabConfig
	if labsEnv != "" {
		labs = parseLabs(labsEnv, uploadPath, dbPath)
	} else {
		labs = []LabConfig{defaultLab(dbPath, uploadPath)}
	}

	return &Config{
		Environment:   getEnv("ENVIRONMENT", "development"),
		Host:          getEnv("HOST", "0.0.0.0"),
		Port:          getEnv("PORT", "8080"),
		Labs:          labs,
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
