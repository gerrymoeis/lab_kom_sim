package database

import (
	"database/sql"
	"fmt"
)

func runSQLiteMigrations(db *DB) error {
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
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			full_name TEXT NOT NULL,
			role TEXT NOT NULL CHECK(role IN ('admin', 'dosen')),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
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
		`CREATE TABLE IF NOT EXISTS device_types (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			category TEXT NOT NULL,
			brand TEXT,
			model TEXT,
			item_type TEXT NOT NULL DEFAULT 'individual' CHECK(item_type IN ('individual', 'consumable')),
			is_loanable BOOLEAN NOT NULL DEFAULT 1,
			is_consumable BOOLEAN NOT NULL DEFAULT 0,
			asset_code_prefix TEXT,
			default_location TEXT,
			notes_template TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_type_id INTEGER NOT NULL,
			asset_code TEXT UNIQUE,
			name TEXT NOT NULL,
			brand TEXT,
			model TEXT,
			serial_number TEXT,
			item_type TEXT NOT NULL DEFAULT 'individual' CHECK(item_type IN ('individual', 'consumable')),
			is_loanable BOOLEAN NOT NULL DEFAULT 1,
			is_consumable BOOLEAN NOT NULL DEFAULT 0,
			quantity_total INTEGER DEFAULT 1,
			quantity_available INTEGER DEFAULT 1,
			condition TEXT NOT NULL DEFAULT 'baik' CHECK(condition IN ('baik', 'rusak', 'maintenance')),
			location TEXT,
			purchase_date DATE,
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (device_type_id) REFERENCES device_types(id) ON DELETE RESTRICT
		)`,
		`CREATE TABLE IF NOT EXISTS device_loans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL,
			borrower_name TEXT NOT NULL,
			borrower_type TEXT CHECK(borrower_type IN ('dosen', 'mahasiswa', 'staff', 'lainnya')),
			loan_date DATE NOT NULL,
			expected_return_date DATE,
			actual_return_date DATE,
			quantity INTEGER NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'returned', 'overdue')),
			purpose TEXT,
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS device_usages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL,
			user_name TEXT NOT NULL,
			user_type TEXT CHECK(user_type IN ('dosen', 'mahasiswa', 'staff', 'lainnya')),
			usage_date DATE NOT NULL,
			quantity INTEGER NOT NULL DEFAULT 1,
			purpose TEXT,
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
		)`,
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
		`CREATE TABLE IF NOT EXISTS activity_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			username TEXT NOT NULL,
			user_role TEXT NOT NULL,
			action TEXT NOT NULL CHECK(action IN ('create', 'update', 'delete', 'upload', 'login', 'logout', 'view', 'export')),
			entity_type TEXT NOT NULL CHECK(entity_type IN ('pc', 'device', 'software', 'logbook', 'user', 'auth', 'device_loan', 'device_usage')),
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
		`CREATE INDEX IF NOT EXISTS idx_device_types_category ON device_types(category)`,
		`CREATE INDEX IF NOT EXISTS idx_device_types_item_type ON device_types(item_type)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_condition ON devices(condition)`,
		`CREATE INDEX IF NOT EXISTS idx_device_loans_device_id ON device_loans(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_device_loans_status ON device_loans(status)`,
		`CREATE INDEX IF NOT EXISTS idx_device_loans_loan_date ON device_loans(loan_date)`,
		`CREATE INDEX IF NOT EXISTS idx_device_usages_device_id ON device_usages(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_device_usages_usage_date ON device_usages(usage_date)`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	assetColumns := map[string]string{
		"asset_id":           "TEXT",
		"serial_number":      "TEXT",
		"brand":              "TEXT",
		"model":              "TEXT",
		"operating_system":   "TEXT",
		"physical_condition": "TEXT DEFAULT 'baik' CHECK(physical_condition IN ('baik', 'cukup', 'rusak'))",
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

	deviceColumns := map[string]string{
		"device_type_id":     "INTEGER",
		"asset_code":         "TEXT",
		"model":              "TEXT",
		"serial_number":      "TEXT",
		"item_type":          "TEXT NOT NULL DEFAULT 'individual'",
		"is_loanable":        "BOOLEAN NOT NULL DEFAULT 1",
		"is_consumable":      "BOOLEAN NOT NULL DEFAULT 0",
		"quantity_total":     "INTEGER DEFAULT 1",
		"quantity_available": "INTEGER DEFAULT 1",
	}

	for columnName, columnDef := range deviceColumns {
		exists, err := columnExists("devices", columnName)
		if err != nil {
			return fmt.Errorf("failed to check devices column %s: %w", columnName, err)
		}
		if !exists {
			alterSQL := fmt.Sprintf("ALTER TABLE devices ADD COLUMN %s %s", columnName, columnDef)
			if _, err := db.Exec(alterSQL); err != nil {
				return fmt.Errorf("failed to add devices column %s: %w", columnName, err)
			}
		}
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_pcs_asset_id ON pcs(asset_id)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_device_type ON pcs(device_type)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_brand_model ON pcs(brand_model)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_serial_number ON pcs(serial_number)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_student_name ON logbook_entries(student_name)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_nim ON logbook_entries(nim)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_purpose ON logbook_entries(purpose)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_time_in ON logbook_entries(time_in)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_composite_search ON logbook_entries(student_name, nim, date)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_device_type_id ON devices(device_type_id)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_asset_code ON devices(asset_code)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_item_type ON devices(item_type)`,
	}

	for _, indexSQL := range indexes {
		if _, err := db.Exec(indexSQL); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	var uniqueIndexExists bool
	err := db.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM sqlite_master 
		WHERE type='index' AND name='idx_logbook_unique'
	`).Scan(&uniqueIndexExists)

	if err == nil && uniqueIndexExists {
		_, err = db.Exec(`DROP INDEX IF EXISTS idx_logbook_unique`)
		if err != nil {
			fmt.Printf("Warning: Failed to drop old index: %v\n", err)
		}
	}

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

	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_logbook_unique ON logbook_entries(date, LOWER(TRIM(student_name)), time_in)`)
	if err != nil {
		return fmt.Errorf("failed to create unique index: %w", err)
	}

	// Add category column to software table
	exists, err := columnExists("software", "category")
	if err == nil && !exists {
		_, err = db.Exec(`ALTER TABLE software ADD COLUMN category TEXT NOT NULL DEFAULT 'other'`)
		if err != nil {
			fmt.Printf("Warning: Failed to add category to software: %v\n", err)
		}
	}

	return seedDeviceTypesIfEmpty(db, "1", "0")
}
