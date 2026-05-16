package database

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func SeedDefaultUser(db *DB) error {
	admins := []struct {
		Username string
		Password string
		FullName string
	}{
		{"admin", getEnvDefault("ADMIN_PASSWORD", "admin123"), "Administrator"},
		{"rekan", getEnvDefault("REKAN_PASSWORD", "rekan123"), "Rekan Administrator"},
	}

	for _, a := range admins {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", a.Username).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check user %s: %w", a.Username, err)
		}
		if count > 0 {
			continue
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(a.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password for %s: %w", a.Username, err)
		}

		_, err = db.Exec(`INSERT INTO users (username, password, full_name, role, session_token) VALUES (?, ?, ?, 'admin', NULL)`,
			a.Username, string(hashedPassword), a.FullName)
		if err != nil {
			return fmt.Errorf("failed to create user %s: %w", a.Username, err)
		}

		fmt.Printf("✅ Default user created: %s\n", a.Username)
	}

	return nil
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
