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
	// Helper function to check if column exists
	columnExists := func(tableName, columnName string) (bool, error) {
		query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
		rows, err := db.Query(query)
		if err != nil {
			return false, err
		}
		defer rows.Close()

		for rows.Next() {
			var cid int
			var name string
			var dataType string
			var notNull int
			var defaultValue sql.NullString
			var pk int

			if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
				return false, err
			}

			if name == columnName {
				return true, nil
			}
		}
		return false, nil
	}

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
			purpose TEXT,
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

		// Activity logs table (audit trail)
		`CREATE TABLE IF NOT EXISTS activity_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			username TEXT NOT NULL,
			user_role TEXT NOT NULL,
			action TEXT NOT NULL CHECK(action IN ('create', 'update', 'delete', 'upload', 'login', 'logout', 'view', 'export')),
			entity_type TEXT NOT NULL CHECK(entity_type IN ('pc', 'device', 'software', 'logbook', 'user', 'auth')),
			entity_id INTEGER,
			description TEXT NOT NULL,
			old_values TEXT,
			new_values TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			ip_address TEXT,
			user_agent TEXT,
			status TEXT DEFAULT 'success' CHECK(status IN ('success', 'failed', 'error')),
			error_message TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
		)`,

		// Create indexes for better performance
		`CREATE INDEX IF NOT EXISTS idx_pcs_status ON pcs(status)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_number ON pcs(pc_number)`,
		`CREATE INDEX IF NOT EXISTS idx_software_pc_id ON software(pc_id)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_date ON logbook_entries(date)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_nim ON logbook_entries(nim)`,
		`CREATE INDEX IF NOT EXISTS idx_maintenance_pc_id ON maintenance_logs(pc_id)`,
		`CREATE INDEX IF NOT EXISTS idx_maintenance_date ON maintenance_logs(date)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_logs_user_id ON activity_logs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_logs_action ON activity_logs(action)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_logs_entity ON activity_logs(entity_type, entity_id)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_logs_created_at ON activity_logs(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_logs_username ON activity_logs(username)`,
	}

	// Run basic migrations
	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	// Add new columns for asset management (check if exists first)
	assetColumns := map[string]string{
		"asset_id":           "TEXT",
		"serial_number":      "TEXT",
		"brand":              "TEXT",
		"model":              "TEXT",
		"operating_system":   "TEXT",
		"physical_condition": "TEXT DEFAULT 'baik' CHECK(physical_condition IN ('baik', 'cukup', 'rusak'))",
		// New columns for PC refinement
		"device_type":        "TEXT NOT NULL DEFAULT 'PC All-in-one'",
		"brand_model":        "TEXT NOT NULL DEFAULT 'Axioo Mypc One Pro K7-24 (16N9)'",
		"accessories":        "TEXT NOT NULL DEFAULT 'Keyboard & Mouse Axioo (Wired Set)'",
		"action_notes":       "TEXT",
		"photo_serial":       "TEXT",
		"photo_front":        "TEXT",
	}

	for columnName, columnDef := range assetColumns {
		exists, err := columnExists("pcs", columnName)
		if err != nil {
			return fmt.Errorf("failed to check column %s: %w", columnName, err)
		}

		if !exists {
			alterSQL := fmt.Sprintf("ALTER TABLE pcs ADD COLUMN %s %s", columnName, columnDef)
			if _, err := db.Exec(alterSQL); err != nil {
				return fmt.Errorf("failed to add column %s: %w", columnName, err)
			}
		}
	}

	// Create indexes if not exists
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_pcs_asset_id ON pcs(asset_id)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_device_type ON pcs(device_type)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_brand_model ON pcs(brand_model)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_serial_number ON pcs(serial_number)`,
		// Logbook indexes untuk performance (search, sort, filter)
		`CREATE INDEX IF NOT EXISTS idx_logbook_student_name ON logbook_entries(student_name)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_nim ON logbook_entries(nim)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_purpose ON logbook_entries(purpose)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_time_in ON logbook_entries(time_in)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_composite_search ON logbook_entries(student_name, nim, date)`,
	}

	for _, indexSQL := range indexes {
		if _, err := db.Exec(indexSQL); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// Handle unique constraint untuk logbook dengan cleanup duplicates
	var uniqueIndexExists bool
	err := db.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM sqlite_master 
		WHERE type='index' AND name='idx_logbook_unique'
	`).Scan(&uniqueIndexExists)

	if err == nil && uniqueIndexExists {
		// Drop old index (NIM-based)
		_, err = db.Exec(`DROP INDEX IF EXISTS idx_logbook_unique`)
		if err != nil {
			fmt.Printf("Warning: Failed to drop old index: %v\n", err)
		}
	}

	// Cleanup duplicates before creating new unique index
	// Duplicates defined as: same date + same student_name (case-insensitive) + same time_in
	// This catches cases where OCR misreads NIM but name/date/time are same
	result, err := db.Exec(`
		DELETE FROM logbook_entries
		WHERE id NOT IN (
			SELECT MIN(id)
			FROM logbook_entries
			GROUP BY date, LOWER(TRIM(student_name)), time_in
		)
	`)
	
	if err != nil {
		fmt.Printf("Warning: Failed to cleanup duplicates: %v\n", err)
	} else {
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			fmt.Printf("Cleaned up %d duplicate logbook entries\n", rowsAffected)
		}
	}

	// Create new unique index based on name + date + time (not NIM)
	// This prevents duplicates even if OCR misreads NIM digits
	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_logbook_unique ON logbook_entries(date, LOWER(TRIM(student_name)), time_in)`)
	if err != nil {
		return fmt.Errorf("failed to create unique index: %w", err)
	}

	return nil
}
