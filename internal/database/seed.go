package database

import (
	"database/sql"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// SeedDefaultUser creates default admin user if not exists
func SeedDefaultUser(db *sql.DB) error {
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

// SeedSampleData creates sample data for testing (optional)
func SeedSampleData(db *sql.DB) error {
	// Check if PCs already exist
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pcs").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check PCs: %w", err)
	}

	if count > 0 {
		return nil // Data already exists
	}

	// Insert 40 PCs in 5 rows x 8 columns
	for row := 1; row <= 5; row++ {
		for col := 1; col <= 8; col++ {
			pcNumber := (row-1)*8 + col
			status := "normal"
			
			// Add some variety in status for demo
			if pcNumber%10 == 0 {
				status = "warning"
			} else if pcNumber%15 == 0 {
				status = "broken"
			}

			_, err := db.Exec(`
				INSERT INTO pcs (pc_number, row, column, status, processor, ram, storage, notes)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`, pcNumber, row, col, status, "Intel Core i5", "8GB", "256GB SSD", fmt.Sprintf("PC Lab %d", pcNumber))

			if err != nil {
				return fmt.Errorf("failed to insert PC %d: %w", pcNumber, err)
			}
		}
	}

	// Insert sample devices
	devices := []struct {
		name     string
		category string
		brand    string
		location string
	}{
		{"Printer HP LaserJet", "printer", "HP", "Ruang Lab"},
		{"Router TP-Link", "router", "TP-Link", "Ruang Lab"},
		{"Speaker Logitech", "speaker", "Logitech", "Ruang Lab"},
		{"PC Cadangan 1", "pc_cadangan", "Custom", "Gudang"},
		{"Komputer Labor", "komputer_labor", "Dell", "Meja Labor"},
	}

	for _, device := range devices {
		_, err := db.Exec(`
			INSERT INTO devices (name, category, brand, condition, location)
			VALUES (?, ?, ?, ?, ?)
		`, device.name, device.category, device.brand, "baik", device.location)

		if err != nil {
			return fmt.Errorf("failed to insert device %s: %w", device.name, err)
		}
	}

	fmt.Println("✅ Sample data created (40 PCs and 5 devices)")
	return nil
}
