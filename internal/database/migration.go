package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

type dialect struct {
	pkType, tsType, boolTrue, boolFalse, qRow, qCol string
	columnExists                                    func(db *DB, table, col string) (bool, error)
}

func runMigrations(db *DB, isPostgres bool) error {
	d := dialect{
		pkType: "INTEGER PRIMARY KEY AUTOINCREMENT",
		tsType: "DATETIME",
		boolTrue: "1", boolFalse: "0",
		qRow: "row", qCol: "column",
		columnExists: func(db *DB, table, col string) (bool, error) {
			rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
			if err != nil { return false, err }
			defer rows.Close()
			for rows.Next() {
				var cid int; var name, dtype string; var notNull int; var dflt sql.NullString; var pk int
				if rows.Scan(&cid, &name, &dtype, &notNull, &dflt, &pk) == nil && name == col { return true, nil }
			}
			return false, nil
		},
	}
	if isPostgres {
		d = dialect{
			pkType: "SERIAL PRIMARY KEY", tsType: "TIMESTAMP",
			boolTrue: "TRUE", boolFalse: "FALSE",
			qRow: `"row"`, qCol: `"column"`,
			columnExists: func(db *DB, table, col string) (bool, error) {
				var exists bool
				err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name=? AND column_name=?)`, table, col).Scan(&exists)
				return exists, err
			},
		}
	}

	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id {{PK}},
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			full_name TEXT NOT NULL,
			role TEXT NOT NULL CHECK(role IN ('admin', 'dosen')),
			session_token TEXT,
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			updated_at {{TS}} DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS pcs (
			id {{PK}},
			pc_number INTEGER UNIQUE NOT NULL CHECK(pc_number >= 1 AND pc_number <= 43),
			{{ROW}} INTEGER NOT NULL CHECK({{ROW}} >= 0 AND {{ROW}} <= 5),
			{{COL}} INTEGER NOT NULL CHECK({{COL}} >= 0 AND {{COL}} <= 8),
			status TEXT NOT NULL DEFAULT 'normal' CHECK(status IN ('normal', 'warning', 'broken', 'inactive')),
			processor TEXT,
			ram TEXT,
			storage TEXT,
			purchase_date DATE,
			notes TEXT,
			last_checked {{TS}},
			asset_id TEXT,
			serial_number TEXT,
			brand TEXT,
			model TEXT,
			operating_system TEXT,
			physical_condition TEXT DEFAULT 'baik' CHECK(physical_condition IN ('baik', 'cukup', 'rusak')),
			device_type TEXT NOT NULL DEFAULT 'PC All-in-one',
			brand_model TEXT NOT NULL DEFAULT 'Axioo Mypc One Pro K7-24 (16N9)',
			accessories TEXT NOT NULL DEFAULT 'Keyboard & Mouse Axioo (Wired Set)',
			action_notes TEXT,
			photo_serial TEXT,
			photo_front TEXT,
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			updated_at {{TS}} DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS device_types (
			id {{PK}},
			name TEXT NOT NULL UNIQUE,
			category TEXT NOT NULL,
			brand TEXT,
			model TEXT,
			item_type TEXT NOT NULL DEFAULT 'individual' CHECK(item_type IN ('individual', 'consumable')),
			is_loanable BOOLEAN NOT NULL DEFAULT {{TRUE}},
			is_consumable BOOLEAN NOT NULL DEFAULT {{FALSE}},
			asset_code_prefix TEXT,
			default_location TEXT,
			notes_template TEXT,
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			updated_at {{TS}} DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS devices (
			id {{PK}},
			device_type_id INTEGER NOT NULL REFERENCES device_types(id) ON DELETE RESTRICT,
			asset_code TEXT UNIQUE,
			name TEXT NOT NULL,
			brand TEXT,
			model TEXT,
			serial_number TEXT,
			item_type TEXT NOT NULL DEFAULT 'individual' CHECK(item_type IN ('individual', 'consumable')),
			is_loanable BOOLEAN NOT NULL DEFAULT {{TRUE}},
			is_consumable BOOLEAN NOT NULL DEFAULT {{FALSE}},
			quantity_total INTEGER DEFAULT 1,
			quantity_available INTEGER DEFAULT 1,
			condition TEXT NOT NULL DEFAULT 'baik' CHECK(condition IN ('baik', 'rusak', 'maintenance')),
			location TEXT,
			purchase_date DATE,
			notes TEXT,
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			updated_at {{TS}} DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS device_loans (
			id {{PK}},
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
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			updated_at {{TS}} DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS device_usages (
			id {{PK}},
			device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			user_name TEXT NOT NULL,
			user_type TEXT CHECK(user_type IN ('dosen', 'mahasiswa', 'staff', 'lainnya')),
			usage_date DATE NOT NULL,
			quantity INTEGER NOT NULL DEFAULT 1,
			is_available TEXT NOT NULL DEFAULT 'yes' CHECK(is_available IN ('yes', 'no')),
			purpose TEXT,
			notes TEXT,
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS software_catalog (
			id {{PK}},
			name TEXT NOT NULL UNIQUE,
			category TEXT NOT NULL DEFAULT 'other' CHECK(category IN ('required', 'other')),
			description TEXT,
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			updated_at {{TS}} DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS pc_software (
			pc_id INTEGER NOT NULL REFERENCES pcs(id) ON DELETE CASCADE,
			software_id INTEGER NOT NULL REFERENCES software_catalog(id) ON DELETE CASCADE,
			installed BOOLEAN NOT NULL DEFAULT {{TRUE}},
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			updated_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (pc_id, software_id)
		)`,
		`CREATE TABLE IF NOT EXISTS course_schedules (
			id {{PK}},
			course_name TEXT NOT NULL,
			lecturer TEXT NOT NULL,
			day TEXT NOT NULL,
			class TEXT NOT NULL,
			time_start TEXT NOT NULL,
			time_end TEXT NOT NULL,
			notes TEXT,
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			updated_at {{TS}} DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS logbook_entries (
			id {{PK}},
			date DATE NOT NULL,
			student_name TEXT NOT NULL,
			nim TEXT NOT NULL CHECK(length(nim) = 11),
			time_in TEXT NOT NULL,
			time_out TEXT,
			purpose TEXT,
			source_file TEXT,
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			updated_at {{TS}} DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS maintenance_logs (
			id {{PK}},
			pc_id INTEGER NOT NULL REFERENCES pcs(id) ON DELETE CASCADE,
			date DATE NOT NULL,
			type TEXT NOT NULL CHECK(type IN ('repair', 'upgrade', 'cleaning', 'check')),
			description TEXT NOT NULL,
			technician TEXT,
			cost REAL DEFAULT 0,
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS lost_items (
			id {{PK}},
			device_id INTEGER,
			item_name TEXT NOT NULL,
			item_description TEXT,
			reported_by TEXT NOT NULL,
			reported_date DATE NOT NULL DEFAULT (DATE('now')),
			last_seen_at {{TS}},
			location_last_seen TEXT,
			status TEXT NOT NULL DEFAULT 'hilang' CHECK(status IN ('hilang', 'ditemukan', 'dilaporkan')),
			owner_name TEXT,
			owner_class TEXT,
			owner_nim TEXT,
			returned_date DATE,
			photo TEXT,
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			updated_at {{TS}} DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS activity_logs (
			id {{PK}},
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE SET NULL,
			username TEXT NOT NULL,
			user_role TEXT NOT NULL,
			action TEXT NOT NULL CHECK(action IN ('create', 'update', 'delete', 'upload', 'login', 'logout', 'view', 'export')),
			entity_type TEXT NOT NULL CHECK(entity_type IN ('pc', 'device', 'software', 'logbook', 'user', 'auth', 'device_loan', 'device_usage', 'schedule', 'device_type', 'lost_item')),
			entity_id INTEGER,
			description TEXT NOT NULL,
			old_values TEXT,
			new_values TEXT,
			created_at {{TS}} NOT NULL DEFAULT CURRENT_TIMESTAMP,
			ip_address TEXT,
			user_agent TEXT,
			status TEXT DEFAULT 'success' CHECK(status IN ('success', 'failed', 'error')),
			error_message TEXT
		)`,
	}

	for _, t := range tables {
		t = strings.ReplaceAll(t, "{{PK}}", d.pkType)
		t = strings.ReplaceAll(t, "{{TS}}", d.tsType)
		t = strings.ReplaceAll(t, "{{TRUE}}", d.boolTrue)
		t = strings.ReplaceAll(t, "{{FALSE}}", d.boolFalse)
		t = strings.ReplaceAll(t, "{{ROW}}", d.qRow)
		t = strings.ReplaceAll(t, "{{COL}}", d.qCol)
		if _, err := db.Exec(t); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, t)
		}
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_pcs_status ON pcs(status)`,
		`CREATE INDEX IF NOT EXISTS idx_pc_software_pc_id ON pc_software(pc_id)`,
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
		`CREATE INDEX IF NOT EXISTS idx_pcs_asset_id ON pcs(asset_id)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_pc_type ON pcs(pc_type)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_brand_model ON pcs(brand_model)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_serial_number ON pcs(serial_number)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_student_name ON logbook_entries(student_name)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_purpose ON logbook_entries(purpose)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_time_in ON logbook_entries(time_in)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_composite_search ON logbook_entries(student_name, nim, date)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_device_type_id ON devices(device_type_id)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_asset_code ON devices(asset_code)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_item_type ON devices(item_type)`,
		`CREATE INDEX IF NOT EXISTS idx_software_catalog_category ON software_catalog(category)`,
		`CREATE INDEX IF NOT EXISTS idx_software_catalog_cat_name ON software_catalog(category, name)`,
		`CREATE INDEX IF NOT EXISTS idx_schedules_day ON course_schedules(day)`,
		`CREATE INDEX IF NOT EXISTS idx_schedules_day_time ON course_schedules(day, time_start)`,
		`CREATE INDEX IF NOT EXISTS idx_schedules_lecturer ON course_schedules(lecturer)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_loanable_qty ON devices(is_loanable, quantity_available)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_consumable_qty ON devices(is_consumable, quantity_available)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_serial ON devices(serial_number)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_date_time ON logbook_entries(date, time_in)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_logs_status ON activity_logs(status)`,
		`CREATE INDEX IF NOT EXISTS idx_pc_software_software_id ON pc_software(software_id)`,
		`CREATE INDEX IF NOT EXISTS idx_device_loans_borrower ON device_loans(borrower_name)`,
		`CREATE INDEX IF NOT EXISTS idx_device_usages_user_name ON device_usages(user_name)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_grid ON pcs("row", "column")`,
		`CREATE INDEX IF NOT EXISTS idx_lost_items_status ON lost_items(status)`,
		`CREATE INDEX IF NOT EXISTS idx_lost_items_reported_date ON lost_items(reported_date)`,
		`CREATE INDEX IF NOT EXISTS idx_lost_items_status_date ON lost_items(status, reported_date)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_date_time_id ON logbook_entries(date, time_in, id)`,
		`CREATE INDEX IF NOT EXISTS idx_logbook_search_id ON logbook_entries(student_name, nim, date, id)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_logs_created_id ON activity_logs(created_at, id)`,
	}

	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// ALTER TABLE for columns that might be missing on existing databases
	pcsExtra := map[string]string{
		"asset_id": "TEXT", "serial_number": "TEXT", "brand": "TEXT", "model": "TEXT",
		"operating_system": "TEXT",
		"physical_condition": "TEXT DEFAULT 'baik' CHECK(physical_condition IN ('baik', 'cukup', 'rusak'))",
		"device_type": "TEXT NOT NULL DEFAULT 'PC All-in-one'",
		"brand_model": "TEXT NOT NULL DEFAULT 'Axioo Mypc One Pro K7-24 (16N9)'",
		"accessories": "TEXT NOT NULL DEFAULT 'Keyboard & Mouse Axioo (Wired Set)'",
		"label": "TEXT DEFAULT ''",
		"action_notes": "TEXT", "photo_serial": "TEXT", "photo_front": "TEXT",
	}
	devicesExtra := map[string]string{
		"device_type_id": "INTEGER", "asset_code": "TEXT", "model": "TEXT",
		"serial_number": "TEXT", "item_type": "TEXT NOT NULL DEFAULT 'individual'",
		"is_loanable": "BOOLEAN NOT NULL DEFAULT " + d.boolTrue,
		"is_consumable": "BOOLEAN NOT NULL DEFAULT " + d.boolFalse,
		"quantity_total": "INTEGER DEFAULT 1", "quantity_available": "INTEGER DEFAULT 1",
	}

	for colName, colDef := range pcsExtra {
		exists, err := d.columnExists(db, "pcs", colName)
		if err != nil { return fmt.Errorf("failed to check pcs.%s: %w", colName, err) }
		if !exists {
			if _, err := db.Exec(fmt.Sprintf("ALTER TABLE pcs ADD COLUMN %s %s", colName, colDef)); err != nil {
				return fmt.Errorf("failed to add pcs.%s: %w", colName, err)
			}
		}
	}
	for colName, colDef := range devicesExtra {
		exists, err := d.columnExists(db, "devices", colName)
		if err != nil { return fmt.Errorf("failed to check devices.%s: %w", colName, err) }
		if !exists {
			if _, err := db.Exec(fmt.Sprintf("ALTER TABLE devices ADD COLUMN %s %s", colName, colDef)); err != nil {
				return fmt.Errorf("failed to add devices.%s: %w", colName, err)
			}
		}
	}

	// Drop old unique index if exists
	db.Exec(`DROP INDEX IF EXISTS idx_logbook_unique`)

	// Cleanup duplicates
	if res, err := db.Exec(`DELETE FROM logbook_entries WHERE id NOT IN (SELECT MIN(id) FROM logbook_entries GROUP BY date, LOWER(TRIM(student_name)), time_in)`); err == nil {
		if n, _ := res.RowsAffected(); n > 0 { fmt.Printf("Cleaned up %d duplicate logbook entries\n", n) }
	}

	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_logbook_unique ON logbook_entries(date, LOWER(TRIM(student_name)), time_in)`); err != nil {
		return fmt.Errorf("failed to create unique index: %w", err)
	}

	if !isPostgres {
		hasFTS := true
		if _, err := db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS logbook_fts USING fts5(
			student_name, nim, purpose,
			tokenize='trigram',
			detail='none',
			content='logbook_entries',
			content_rowid='id'
		)`); err != nil {
			log.Printf("WARN: fts5 not available (%v), logbook search will use LIKE fallback", err)
			hasFTS = false
		}

		if hasFTS {
			var ftsCount int

			for _, t := range []string{
				`CREATE TRIGGER IF NOT EXISTS logbook_fts_ai AFTER INSERT ON logbook_entries BEGIN
					INSERT INTO logbook_fts(rowid, student_name, nim, purpose) VALUES (new.id, new.student_name, new.nim, new.purpose); END`,
				`CREATE TRIGGER IF NOT EXISTS logbook_fts_ad AFTER DELETE ON logbook_entries BEGIN
					INSERT INTO logbook_fts(logbook_fts, rowid, student_name, nim, purpose) VALUES('delete', old.id, old.student_name, old.nim, old.purpose); END`,
				`CREATE TRIGGER IF NOT EXISTS logbook_fts_au AFTER UPDATE ON logbook_entries BEGIN
					INSERT INTO logbook_fts(logbook_fts, rowid, student_name, nim, purpose) VALUES('delete', old.id, old.student_name, old.nim, old.purpose);
					INSERT INTO logbook_fts(rowid, student_name, nim, purpose) VALUES (new.id, new.student_name, new.nim, new.purpose); END`,
			} {
				if _, err := db.Exec(t); err != nil {
					log.Printf("WARN: failed to create fts5 trigger: %v", err)
				}
			}

			db.QueryRow(`SELECT COUNT(*) FROM logbook_fts`).Scan(&ftsCount)
			if ftsCount == 0 {
				db.Exec(`INSERT INTO logbook_fts(rowid, student_name, nim, purpose) SELECT id, student_name, nim, purpose FROM logbook_entries`)
			}
		}

		if _, err := db.Exec("ANALYZE"); err != nil {
			return fmt.Errorf("failed to run ANALYZE: %w", err)
		}
	}

	return seedDeviceTypesIfEmpty(db, d.boolTrue, d.boolFalse)
}
