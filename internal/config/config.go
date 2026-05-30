package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds application configuration
type Config struct {
	Environment      string
	Host             string
	Port             string
	DatabasePath     string
	DatabaseURL      string
	SessionSecret    string
	UploadPath       string
	GeminiAPIKey     string
	OpenRouterAPIKey string
	Android          bool
	WriteMode        string
	Timezone         string
	DefaultPageSize  int
	Backup           BackupConfig
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

// Load loads configuration from environment variables with defaults
func Load() *Config {
	// Load .env file if exists
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables or defaults")
	}

	return &Config{
		Environment:   getEnv("ENVIRONMENT", "development"),
		Host:          getEnv("HOST", "0.0.0.0"),
		Port:          getEnv("PORT", "8080"),
		DatabasePath:  getEnv("DATABASE_PATH", "inventaris_lab.db"),
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		SessionSecret: getEnv("SESSION_SECRET", "change-this-secret-in-production"),
		UploadPath:    getEnv("UPLOAD_PATH", "uploads"),
		GeminiAPIKey:     getEnv("GEMINI_API_KEY", ""),
		OpenRouterAPIKey: getEnv("OPENROUTER_API_KEY", ""),
		Android:          getEnv("ANDROID", "false") == "true",
		WriteMode:        getEnv("WRITE_MODE", "sync"),
		Timezone:         getEnv("TIMEZONE", "Asia/Jakarta"),
		DefaultPageSize:  getEnvInt("DEFAULT_PAGE_SIZE", 25),
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
