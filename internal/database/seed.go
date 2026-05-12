package database

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// SeedDefaultUser creates default admin user if not exists
func SeedDefaultUser(db *DB) error {
	// Check if admin user exists
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", "admin").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check admin user: %w", err)
	}

	if count > 0 {
		return nil // Admin already exists
	}

	// Hash default password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Insert default admin user
	_, err = db.Exec(`
		INSERT INTO users (username, password, full_name, role)
		VALUES (?, ?, ?, ?)
	`, "admin", string(hashedPassword), "Administrator", "admin")

	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	fmt.Println("✅ Default admin user created (username: admin, password: admin123)")
	return nil
}
