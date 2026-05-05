package database

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// InitDB initializes database connection
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return db, nil
}

// RunMigrations runs database migrations
func RunMigrations(db *sql.DB) error {
	migrations := []string{
		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			full_name TEXT NOT NULL,
			role TEXT NOT NULL CHECK(role IN ('admin', 'dosen')),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// PCs table
		`CREATE TABLE IF NOT EXISTS pcs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			pc_number INTEGER UNIQUE NOT NULL CHECK(pc_number >= 1 AND pc_number <= 40),
			row INTEGER NOT NULL CHECK(row >= 1 AND row <= 5),
			column INTEGER NOT NULL CHECK(column >= 1 AND column <= 8),
			status TEXT NOT NULL DEFAULT 'normal' CHECK(status IN ('normal', 'warning', 'broken', 'inactive')),
			processor TEXT,
			ram TEXT,
			storage TEXT,
			purchase_date DATE,
			notes TEXT,
			last_checked DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Add new columns for asset management (if not exists)
		`ALTER TABLE pcs ADD COLUMN asset_id TEXT`,
		`ALTER TABLE pcs ADD COLUMN serial_number TEXT`,
		`ALTER TABLE pcs ADD COLUMN brand TEXT`,
		`ALTER TABLE pcs ADD COLUMN model TEXT`,
		`ALTER TABLE pcs ADD COLUMN operating_system TEXT`,
		`ALTER TABLE pcs ADD COLUMN physical_condition TEXT DEFAULT 'baik' CHECK(physical_condition IN ('baik', 'cukup', 'rusak'))`,

		// Devices table
		`CREATE TABLE IF NOT EXISTS devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			category TEXT NOT NULL CHECK(category IN ('printer', 'router', 'speaker', 'pc_cadangan', 'komputer_labor', 'komputer_dosen', 'lainnya')),
			brand TEXT,
			condition TEXT NOT NULL DEFAULT 'baik' CHECK(condition IN ('baik', 'rusak', 'maintenance')),
			location TEXT,
			purchase_date DATE,
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Software table
		`CREATE TABLE IF NOT EXISTS software (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			pc_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			version TEXT,
			license TEXT,
			install_date DATE,
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (pc_id) REFERENCES pcs(id) ON DELETE CASCADE
		)`,

		// Logbook entries table
		`CREATE TABLE IF NOT EXISTS logbook_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date DATE NOT NULL,
			student_name TEXT NOT NULL,
			nim TEXT NOT NULL,
			time_in TEXT NOT NULL,
			time_out TEXT,
			notes TEXT,
			source_file TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Maintenance logs table
		`CREATE TABLE IF NOT EXISTS maintenance_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			pc_id INTEGER NOT NULL,
			date DATE NOT NULL,
			type TEXT NOT NULL CHECK(type IN ('repair', 'upgrade', 'cleaning', 'check')),
			description TEXT NOT NULL,
			technician TEXT,
			cost REAL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (pc_id) REFERENCES pcs(id) ON DELETE CASCADE
		)`,

		// Create indexes for better performance
		`CREATE INDEX IF NOT EXISTS idx_pcs_status ON pcs(status)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_number ON pcs(pc_number)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_asset_id ON pcs(asset_id)`,
		`CREATE INDEX IF NOT EXISTS idx_software_pc_id ON software(pc_id)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_date ON logbook_entries(date)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_nim ON logbook_entries(nim)`,
		`CREATE INDEX IF NOT EXISTS idx_maintenance_pc_id ON maintenance_logs(pc_id)`,
		`CREATE INDEX IF NOT EXISTS idx_maintenance_date ON maintenance_logs(date)`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}
