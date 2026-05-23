package config

import (
	"log"
	"os"

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
	WriteMode        string
	Timezone         string
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
		WriteMode:        getEnv("WRITE_MODE", "sync"),
		Timezone:         getEnv("TIMEZONE", "Local"),
	}
}

// getEnv gets environment variable with fallback to default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
