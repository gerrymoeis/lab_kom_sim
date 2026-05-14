package database

import (
	"fmt"
)

func runPostgresMigrations(db *DB) error {
	columnExists := func(tableName, columnName string) (bool, error) {
		var exists bool
		err := db.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM information_schema.columns
				WHERE table_name = $1 AND column_name = $2
			)
		`, tableName, columnName).Scan(&exists)
		return exists, err
	}

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			full_name TEXT NOT NULL,
			role TEXT NOT NULL CHECK(role IN ('admin', 'dosen')),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS pcs (
			id SERIAL PRIMARY KEY,
			pc_number INTEGER UNIQUE NOT NULL CHECK(pc_number >= 1 AND pc_number <= 40),
			"row" INTEGER NOT NULL CHECK("row" >= 1 AND "row" <= 5),
			"column" INTEGER NOT NULL CHECK("column" >= 1 AND "column" <= 8),
			status TEXT NOT NULL DEFAULT 'normal' CHECK(status IN ('normal', 'warning', 'broken', 'inactive')),
			processor TEXT,
			ram TEXT,
			storage TEXT,
			purchase_date DATE,
			notes TEXT,
			last_checked TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS device_types (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			category TEXT NOT NULL,
			brand TEXT,
			model TEXT,
			item_type TEXT NOT NULL DEFAULT 'individual' CHECK(item_type IN ('individual', 'consumable')),
			is_loanable BOOLEAN NOT NULL DEFAULT TRUE,
			is_consumable BOOLEAN NOT NULL DEFAULT FALSE,
			asset_code_prefix TEXT,
			default_location TEXT,
			notes_template TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS devices (
			id SERIAL PRIMARY KEY,
			device_type_id INTEGER NOT NULL REFERENCES device_types(id) ON DELETE RESTRICT,
			asset_code TEXT UNIQUE,
			name TEXT NOT NULL,
			brand TEXT,
			model TEXT,
			serial_number TEXT,
			item_type TEXT NOT NULL DEFAULT 'individual' CHECK(item_type IN ('individual', 'consumable')),
			is_loanable BOOLEAN NOT NULL DEFAULT TRUE,
			is_consumable BOOLEAN NOT NULL DEFAULT FALSE,
			quantity_total INTEGER DEFAULT 1,
			quantity_available INTEGER DEFAULT 1,
			condition TEXT NOT NULL DEFAULT 'baik' CHECK(condition IN ('baik', 'rusak', 'maintenance')),
			location TEXT,
			purchase_date DATE,
			notes TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS device_loans (
			id SERIAL PRIMARY KEY,
			device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			borrower_name TEXT NOT NULL,
			borrower_type TEXT CHECK(borrower_type IN ('dosen', 'mahasiswa', 'staff', 'lainnya')),
			loan_date DATE NOT NULL,
			expected_return_date DATE,
			actual_return_date DATE,
			quantity INTEGER NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'returned', 'overdue')),
			purpose TEXT,
			notes TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS device_usages (
			id SERIAL PRIMARY KEY,
			device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			user_name TEXT NOT NULL,
			user_type TEXT CHECK(user_type IN ('dosen', 'mahasiswa', 'staff', 'lainnya')),
			usage_date DATE NOT NULL,
			quantity INTEGER NOT NULL DEFAULT 1,
			purpose TEXT,
			notes TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS software_catalog (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			category TEXT NOT NULL DEFAULT 'other' CHECK(category IN ('required', 'other')),
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS pc_software (
			pc_id INTEGER NOT NULL REFERENCES pcs(id) ON DELETE CASCADE,
			software_id INTEGER NOT NULL REFERENCES software_catalog(id) ON DELETE CASCADE,
			installed BOOLEAN NOT NULL DEFAULT TRUE,
			version TEXT,
			notes TEXT,
			PRIMARY KEY (pc_id, software_id)
		)`,
		`CREATE TABLE IF NOT EXISTS logbook_entries (
			id SERIAL PRIMARY KEY,
			date DATE NOT NULL,
			student_name TEXT NOT NULL,
			nim TEXT NOT NULL,
			time_in TEXT NOT NULL,
			time_out TEXT,
			purpose TEXT,
			source_file TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS maintenance_logs (
			id SERIAL PRIMARY KEY,
			pc_id INTEGER NOT NULL REFERENCES pcs(id) ON DELETE CASCADE,
			date DATE NOT NULL,
			type TEXT NOT NULL CHECK(type IN ('repair', 'upgrade', 'cleaning', 'check')),
			description TEXT NOT NULL,
			technician TEXT,
			cost REAL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS activity_logs (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE SET NULL,
			username TEXT NOT NULL,
			user_role TEXT NOT NULL,
			action TEXT NOT NULL CHECK(action IN ('create', 'update', 'delete', 'upload', 'login', 'logout', 'view', 'export')),
			entity_type TEXT NOT NULL CHECK(entity_type IN ('pc', 'device', 'software', 'logbook', 'user', 'auth', 'device_loan', 'device_usage')),
			entity_id INTEGER,
			description TEXT NOT NULL,
			old_values TEXT,
			new_values TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			ip_address TEXT,
			user_agent TEXT,
			status TEXT DEFAULT 'success' CHECK(status IN ('success', 'failed', 'error')),
			error_message TEXT
		)`,
		// Indexes
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

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("postgres migration failed: %w\nSQL: %s", err, m)
		}
	}

	// ALTER TABLE for PC columns (matching SQLite path)
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

	for colName, colDef := range assetColumns {
		exists, err := columnExists("pcs", colName)
		if err != nil {
			return fmt.Errorf("failed to check pcs column %s: %w", colName, err)
		}
		if !exists {
			sql := fmt.Sprintf("ALTER TABLE pcs ADD COLUMN IF NOT EXISTS %s %s", colName, colDef)
			if _, err := db.Exec(sql); err != nil {
				return fmt.Errorf("failed to add pcs column %s: %w", colName, err)
			}
		}
	}

	// ALTER TABLE for devices columns
	deviceColumns := map[string]string{
		"device_type_id":     "INTEGER",
		"asset_code":         "TEXT",
		"model":              "TEXT",
		"serial_number":      "TEXT",
		"item_type":          "TEXT NOT NULL DEFAULT 'individual'",
		"is_loanable":        "BOOLEAN NOT NULL DEFAULT TRUE",
		"is_consumable":      "BOOLEAN NOT NULL DEFAULT FALSE",
		"quantity_total":     "INTEGER DEFAULT 1",
		"quantity_available": "INTEGER DEFAULT 1",
	}

	for colName, colDef := range deviceColumns {
		exists, err := columnExists("devices", colName)
		if err != nil {
			return fmt.Errorf("failed to check devices column %s: %w", colName, err)
		}
		if !exists {
			sql := fmt.Sprintf("ALTER TABLE devices ADD COLUMN IF NOT EXISTS %s %s", colName, colDef)
			if _, err := db.Exec(sql); err != nil {
				return fmt.Errorf("failed to add devices column %s: %w", colName, err)
			}
		}
	}

	// Additional indexes (matching SQLite path)
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

	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// Drop old unique index if exists then recreate
	_, err := db.Exec(`DROP INDEX IF EXISTS idx_logbook_unique`)
	if err != nil {
		fmt.Printf("Warning: Failed to drop old index: %v\n", err)
	}

	// Cleanup duplicates
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
		if n, _ := result.RowsAffected(); n > 0 {
			fmt.Printf("Cleaned up %d duplicate logbook entries\n", n)
		}
	}

	// Create functional unique index
	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_logbook_unique ON logbook_entries(date, LOWER(TRIM(student_name)), time_in)`)
	if err != nil {
		return fmt.Errorf("failed to create unique index: %w", err)
	}

	// Update activity_logs entity_type CHECK constraint to include device_loan and device_usage
	_, err = db.Exec(`ALTER TABLE activity_logs DROP CONSTRAINT IF EXISTS activity_logs_entity_type_check`)
	if err != nil {
		fmt.Printf("Warning: Failed to drop activity_logs constraint: %v\n", err)
	}
	_, err = db.Exec(`ALTER TABLE activity_logs ADD CONSTRAINT activity_logs_entity_type_check 
		CHECK(entity_type IN ('pc', 'device', 'software', 'logbook', 'user', 'auth', 'device_loan', 'device_usage'))`)
	if err != nil {
		fmt.Printf("Warning: Failed to add activity_logs constraint: %v\n", err)
	}

	return seedDeviceTypesIfEmpty(db, "TRUE", "FALSE")
}
