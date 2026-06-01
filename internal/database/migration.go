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

	// Drop old indexes no longer needed
	for _, idx := range []string{
		"idx_device_types_category", "idx_device_types_item_type",
		"idx_device_loans_status", "idx_devices_item_type",
		"idx_devices_loanable_qty", "idx_devices_consumable_qty",
	} {
		db.Exec("DROP INDEX IF EXISTS " + idx)
	}

	// Detect old device schema and drop tables for recreation
	if hasOld, _ := d.columnExists(db, "device_types", "notes_template"); hasOld {
		log.Println("Detected old device schema — dropping old tables for migration")
		db.Exec("DROP TABLE IF EXISTS device_installations")
		db.Exec("DROP TABLE IF EXISTS loan_extensions")
		db.Exec("DROP TABLE IF EXISTS device_usages")
		db.Exec("DROP TABLE IF EXISTS device_loans")
		db.Exec("DROP TABLE IF EXISTS devices")
		db.Exec("DROP TABLE IF EXISTS device_types")
		db.Exec("DROP TABLE IF EXISTS categories")
	}

	// Recreate activity_logs with updated entity_type CHECK if still old schema
	if hasOldAL, _ := d.columnExists(db, "activity_logs", "id"); hasOldAL {
		var entityCheck string
		err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='activity_logs'`).Scan(&entityCheck)
		if err == nil && !strings.Contains(entityCheck, "'category'") {
			actLogSQL := `CREATE TABLE IF NOT EXISTS activity_logs_v2 (
				id {{PK}},
				user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE SET NULL,
				username TEXT NOT NULL,
				user_role TEXT NOT NULL,
				action TEXT NOT NULL CHECK(action IN ('create', 'update', 'delete', 'upload', 'login', 'logout', 'view', 'export')),
				entity_type TEXT NOT NULL CHECK(entity_type IN ('pc', 'device', 'software', 'logbook', 'user', 'auth', 'device_loan', 'device_usage', 'schedule', 'device_type', 'category', 'device_installation', 'loan_extension')),
				entity_id INTEGER,
				description TEXT NOT NULL,
				old_values TEXT,
				new_values TEXT,
				created_at {{TS}} NOT NULL DEFAULT CURRENT_TIMESTAMP,
				ip_address TEXT,
				user_agent TEXT,
				status TEXT DEFAULT 'success' CHECK(status IN ('success', 'failed', 'error')),
				error_message TEXT
			)`
			actLogSQL = strings.ReplaceAll(actLogSQL, "{{PK}}", d.pkType)
			actLogSQL = strings.ReplaceAll(actLogSQL, "{{TS}}", d.tsType)
			if _, err := db.Exec(actLogSQL); err == nil {
				db.Exec("INSERT INTO activity_logs_v2 SELECT * FROM activity_logs")
				db.Exec("DROP TABLE activity_logs")
				db.Exec("ALTER TABLE activity_logs_v2 RENAME TO activity_logs")
				log.Println("Migrated activity_logs entity_type CHECK constraint")
			}
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
			{{ROW}} INTEGER NOT NULL DEFAULT 0 CHECK({{ROW}} >= 0 AND {{ROW}} <= 6),
			{{COL}} INTEGER NOT NULL DEFAULT 0 CHECK({{COL}} >= 0 AND {{COL}} <= 8),
			status TEXT NOT NULL DEFAULT 'normal' CHECK(status IN ('normal', 'warning', 'broken')),
			processor TEXT,
			ram TEXT,
			storage TEXT,
			purchase_date DATE,
			notes TEXT,
			last_checked {{TS}},
			asset_id TEXT,
			serial_number TEXT,
			operating_system TEXT,
			pc_type TEXT NOT NULL DEFAULT 'PC All-in-one',
			brand_model TEXT NOT NULL DEFAULT 'Axioo Mypc One Pro K7-24 (16N9)',
			accessories TEXT NOT NULL DEFAULT 'Keyboard & Mouse Axioo (Wired Set)',
			photo_serial TEXT,
			photo_front TEXT,
			label TEXT NOT NULL DEFAULT '' UNIQUE,
			placement TEXT NOT NULL DEFAULT 'dipakai' CHECK(placement IN ('dipakai', 'cadangan')),
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			updated_at {{TS}} DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS categories (
			id {{PK}},
			name TEXT NOT NULL UNIQUE,
			default_prefix TEXT NOT NULL UNIQUE,
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS device_types (
			id {{PK}},
			category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
			name TEXT NOT NULL,
			brand TEXT,
			model TEXT,
			asset_code_prefix TEXT NOT NULL UNIQUE,
			usage_type TEXT NOT NULL CHECK(usage_type IN ('loanable', 'consumable', 'installable')),
			default_location TEXT,
			photo TEXT,
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			updated_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(category_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS devices (
			id {{PK}},
			device_type_id INTEGER NOT NULL REFERENCES device_types(id) ON DELETE RESTRICT,
			asset_code TEXT NOT NULL UNIQUE,
			serial_number TEXT,
			condition TEXT NOT NULL DEFAULT 'normal' CHECK(condition IN ('normal', 'warning', 'rusak')),
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
			borrower_type TEXT NOT NULL CHECK(borrower_type IN ('dosen', 'mahasiswa', 'staff', 'lainnya')),
			loan_date DATE NOT NULL,
			return_date DATE NOT NULL,
			actual_return_date DATE,
			purpose TEXT,
			notes TEXT,
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			updated_at {{TS}} DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS device_usages (
			id {{PK}},
			device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			user_name TEXT NOT NULL,
			user_type TEXT NOT NULL CHECK(user_type IN ('dosen', 'mahasiswa', 'staff', 'lainnya')),
			usage_date DATE NOT NULL,
			is_available TEXT NOT NULL DEFAULT 'yes' CHECK(is_available IN ('yes', 'no')),
			purpose TEXT,
			notes TEXT,
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS loan_extensions (
			id {{PK}},
			loan_id INTEGER NOT NULL REFERENCES device_loans(id) ON DELETE CASCADE,
			previous_return_date DATE NOT NULL,
			new_return_date DATE NOT NULL,
			extended_at {{TS}} DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS device_installations (
			id {{PK}},
			device_id INTEGER NOT NULL UNIQUE REFERENCES devices(id) ON DELETE CASCADE,
			location_installed TEXT NOT NULL,
			installation_start_date DATE,
			installation_finish_date DATE,
			photo TEXT,
			notes TEXT,
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			updated_at {{TS}} DEFAULT CURRENT_TIMESTAMP
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
		`CREATE TABLE IF NOT EXISTS activity_logs (
			id {{PK}},
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE SET NULL,
			username TEXT NOT NULL,
			user_role TEXT NOT NULL,
			action TEXT NOT NULL CHECK(action IN ('create', 'update', 'delete', 'upload', 'login', 'logout', 'view', 'export')),
			entity_type TEXT NOT NULL CHECK(entity_type IN ('pc', 'device', 'software', 'logbook', 'user', 'auth', 'device_loan', 'device_usage', 'schedule', 'device_type', 'category', 'device_installation', 'loan_extension')),
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
		`CREATE INDEX IF NOT EXISTS idx_device_types_category_id ON device_types(category_id)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_condition ON devices(condition)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_device_type_id ON devices(device_type_id)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_asset_code ON devices(asset_code)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_serial ON devices(serial_number)`,
		`CREATE INDEX IF NOT EXISTS idx_device_loans_device_id ON device_loans(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_device_loans_loan_date ON device_loans(loan_date)`,
		`CREATE INDEX IF NOT EXISTS idx_device_loans_return_date ON device_loans(return_date)`,
		`CREATE INDEX IF NOT EXISTS idx_device_loans_borrower ON device_loans(borrower_name)`,
		`CREATE INDEX IF NOT EXISTS idx_device_loans_borrower_type ON device_loans(borrower_type)`,
		`CREATE INDEX IF NOT EXISTS idx_device_usages_device_id ON device_usages(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_device_usages_usage_date ON device_usages(usage_date)`,
		`CREATE INDEX IF NOT EXISTS idx_device_usages_user_name ON device_usages(user_name)`,
		`CREATE INDEX IF NOT EXISTS idx_device_usages_user_type ON device_usages(user_type)`,
		`CREATE INDEX IF NOT EXISTS idx_device_usages_available ON device_usages(is_available)`,
		`CREATE INDEX IF NOT EXISTS idx_device_usages_device_avail ON device_usages(device_id, is_available)`,
		`CREATE INDEX IF NOT EXISTS idx_device_loans_return_actual ON device_loans(actual_return_date)`,
		`CREATE INDEX IF NOT EXISTS idx_device_loans_device_return ON device_loans(device_id, actual_return_date)`,
		`CREATE INDEX IF NOT EXISTS idx_device_installations_device_id ON device_installations(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_device_installations_location ON device_installations(location_installed)`,
		`CREATE INDEX IF NOT EXISTS idx_loan_extensions_loan_id ON loan_extensions(loan_id)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_asset_id ON pcs(asset_id)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_pc_type ON pcs(pc_type)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_brand_model ON pcs(brand_model)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_serial_number ON pcs(serial_number)`,
		`CREATE INDEX IF NOT EXISTS idx_pcs_grid ON pcs("row", "column")`,
		`CREATE INDEX IF NOT EXISTS idx_software_catalog_category ON software_catalog(category)`,
		`CREATE INDEX IF NOT EXISTS idx_software_catalog_cat_name ON software_catalog(category, name)`,
		`CREATE INDEX IF NOT EXISTS idx_schedules_day ON course_schedules(day)`,
		`CREATE INDEX IF NOT EXISTS idx_schedules_day_time ON course_schedules(day, time_start)`,
	}

	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// ALTER TABLE for columns that might be missing on existing databases
	pcsExtra := map[string]string{
		"asset_id": "TEXT", "serial_number": "TEXT",
		"operating_system": "TEXT",
		"pc_type": "TEXT NOT NULL DEFAULT 'PC All-in-one'",
		"brand_model": "TEXT NOT NULL DEFAULT 'Axioo Mypc One Pro K7-24 (16N9)'",
		"accessories": "TEXT NOT NULL DEFAULT 'Keyboard & Mouse Axioo (Wired Set)'",
		"label": "TEXT DEFAULT ''",
		"placement": "TEXT NOT NULL DEFAULT 'dipakai' CHECK(placement IN ('dipakai', 'cadangan'))",
		"photo_serial": "TEXT", "photo_front": "TEXT",
	}
	devicesExtra := map[string]string{
		"device_type_id": "INTEGER", "asset_code": "TEXT", "serial_number": "TEXT",
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

	// PC schema migration: status (inactive removed), placement added, device_type→pc_type
	if colExists, _ := d.columnExists(db, "pcs", "status"); colExists {
		// Step 1: Add placement column if missing
		if placedExists, _ := d.columnExists(db, "pcs", "placement"); !placedExists {
			db.Exec(`ALTER TABLE pcs ADD COLUMN placement TEXT NOT NULL DEFAULT 'dipakai' CHECK(placement IN ('dipakai', 'cadangan'))`)
		}
		// Step 2: Migrate old inactive PCs → cadangan
		db.Exec(`UPDATE pcs SET placement='cadangan', status='normal' WHERE status='inactive'`)
		db.Exec(`UPDATE pcs SET placement='dipakai' WHERE placement IS NULL OR placement=''`)

		// Step 3: Rename device_type → pc_type (if column exists and not already renamed)
		if dtExists, _ := d.columnExists(db, "pcs", "device_type"); dtExists {
			if ptExists, _ := d.columnExists(db, "pcs", "pc_type"); !ptExists {
				db.Exec(`ALTER TABLE pcs RENAME COLUMN device_type TO pc_type`)
				db.Exec(`DROP INDEX IF EXISTS idx_pcs_device_type`)
			}
		}
		// Step 4: Drop deprecated columns (best effort — may fail on older SQLite)
		for _, depCol := range []string{"physical_condition", "brand", "model", "action_notes"} {
			if cExists, _ := d.columnExists(db, "pcs", depCol); cExists {
				if _, err := db.Exec(fmt.Sprintf("ALTER TABLE pcs DROP COLUMN %s", depCol)); err != nil {
					log.Printf("WARN: could not drop pcs.%s (%v), skipping", depCol, err)
				}
			}
		}

		// Step 5: Migrate pc_number → lowercase label
		if numExists, _ := d.columnExists(db, "pcs", "pc_number"); numExists {
			db.Exec(`UPDATE pcs SET label = 'pc-' || CAST(pc_number AS TEXT) WHERE label IS NULL OR label = '' OR label GLOB '[0-9]*'`)
			db.Exec(`UPDATE pcs SET label = 'pc-dosen' WHERE label = 'PC-Dosen'`)
			db.Exec(`UPDATE pcs SET label = 'pc-laboran' WHERE label = 'PC-Laboran'`)
			db.Exec(`UPDATE pcs SET label = 'pc-cctv' WHERE label = 'PC-CCTV'`)
			if _, err := db.Exec("ALTER TABLE pcs DROP COLUMN pc_number"); err != nil {
				log.Printf("WARN: could not drop pcs.pc_number (%v), skipping", err)
			}
			db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_pcs_label ON pcs(label)")
		}
	}

	// Step 6: Remove row CHECK upper bound (was `<= 5` or `<= 6` → now unlimited)
	if !isPostgres {
		var pcsSQL string
		err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='pcs'`).Scan(&pcsSQL)
		if err == nil && (strings.Contains(pcsSQL, "<= 5") || strings.Contains(pcsSQL, "<= 6")) {
			db.Exec("PRAGMA foreign_keys=OFF")
			pcsV2 := `CREATE TABLE pcs_v2 (
				id {{PK}},
				{{ROW}} INTEGER NOT NULL DEFAULT 0 CHECK({{ROW}} >= 0),
				{{COL}} INTEGER NOT NULL DEFAULT 0 CHECK({{COL}} >= 0 AND {{COL}} <= 8),
				status TEXT NOT NULL DEFAULT 'normal' CHECK(status IN ('normal', 'warning', 'broken')),
				processor TEXT, ram TEXT, storage TEXT, purchase_date DATE, notes TEXT,
				last_checked {{TS}}, asset_id TEXT, serial_number TEXT, operating_system TEXT,
				pc_type TEXT NOT NULL DEFAULT 'PC All-in-one',
				brand_model TEXT NOT NULL DEFAULT 'Axioo Mypc One Pro K7-24 (16N9)',
				accessories TEXT NOT NULL DEFAULT 'Keyboard & Mouse Axioo (Wired Set)',
				photo_serial TEXT, photo_front TEXT,
				label TEXT NOT NULL DEFAULT '' UNIQUE,
				placement TEXT NOT NULL DEFAULT 'dipakai' CHECK(placement IN ('dipakai', 'cadangan')),
				created_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
				updated_at {{TS}} DEFAULT CURRENT_TIMESTAMP
			)`
			pcsV2 = strings.ReplaceAll(pcsV2, "{{PK}}", d.pkType)
			pcsV2 = strings.ReplaceAll(pcsV2, "{{ROW}}", d.qRow)
			pcsV2 = strings.ReplaceAll(pcsV2, "{{COL}}", d.qCol)
			pcsV2 = strings.ReplaceAll(pcsV2, "{{TS}}", d.tsType)
			if _, err := db.Exec(pcsV2); err == nil {
				db.Exec(`INSERT INTO pcs_v2 SELECT * FROM pcs`)
				db.Exec(`DROP TABLE pcs`)
				db.Exec(`ALTER TABLE pcs_v2 RENAME TO pcs`)
				for _, idx := range []string{
					"idx_pcs_status ON pcs(status)",
					"idx_pcs_asset_id ON pcs(asset_id)",
					"idx_pcs_pc_type ON pcs(pc_type)",
					"idx_pcs_brand_model ON pcs(brand_model)",
					"idx_pcs_serial_number ON pcs(serial_number)",
					"idx_pcs_grid ON pcs(\"row\", \"column\")",
				} {
					db.Exec("CREATE INDEX IF NOT EXISTS " + idx)
				}
				log.Println("Removed pcs row CHECK upper bound (was <=5 or <=6, now unlimited)")
			}
			db.Exec("PRAGMA foreign_keys=ON")
		}
	}

	// Drop old unique index if exists
	db.Exec(`DROP INDEX IF EXISTS idx_logbook_unique`)

	// Cleanup NIM: ensure existing data follows 11-digit rule
	db.Exec(`UPDATE logbook_entries SET nim = '' WHERE length(nim) != 11`)

	// Cleanup duplicates
	if res, err := db.Exec(`DELETE FROM logbook_entries WHERE id NOT IN (SELECT MIN(id) FROM logbook_entries GROUP BY date, LOWER(TRIM(student_name)), time_in)`); err == nil {
		if n, _ := res.RowsAffected(); n > 0 { fmt.Printf("Cleaned up %d duplicate logbook entries\n", n) }
	}

	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_logbook_unique ON logbook_entries(date, LOWER(TRIM(student_name)), time_in)`); err != nil {
		return fmt.Errorf("failed to create unique index: %w", err)
	}

	// Add usage_type override column to devices (nullable, device-level override of device_type.usage_type)
	if _, err := db.Exec(`ALTER TABLE devices ADD COLUMN usage_type TEXT CHECK(usage_type IN ('loanable', 'consumable', 'installable'))`); err != nil {
		if !isPostgres && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("failed to add usage_type to devices: %w", err)
		}
	}

	// Slug column for software_catalog only (name has spaces/special chars, needs slugify)
	if exists, err := d.columnExists(db, "software_catalog", "slug"); err != nil {
		return fmt.Errorf("failed to check software_catalog.slug: %w", err)
	} else if !exists {
		if _, err := db.Exec(`ALTER TABLE software_catalog ADD COLUMN slug TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("failed to add software_catalog.slug: %w", err)
		}
		log.Println("Added slug column to software_catalog")
	}

	// Populate slug for existing software_catalog entries (if slug is empty)
	var emptySlugCount int
	db.QueryRow(`SELECT COUNT(*) FROM software_catalog WHERE slug = '' OR slug IS NULL`).Scan(&emptySlugCount)
	if emptySlugCount > 0 {
		rows, err := db.Query(`SELECT id, name FROM software_catalog WHERE slug = '' OR slug IS NULL`)
		if err != nil {
			return fmt.Errorf("failed to query software for slug population: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id int
			var name string
			if err := rows.Scan(&id, &name); err != nil {
				continue
			}
			// Generate slug using same logic as util.Slugify (inline to avoid import cycle)
			slug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "-"))
			slug = strings.Map(func(r rune) rune {
				if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
					return r
				}
				return -1
			}, slug)
			// Remove consecutive dashes and trim
			for strings.Contains(slug, "--") {
				slug = strings.ReplaceAll(slug, "--", "-")
			}
			slug = strings.Trim(slug, "-")

			if _, err := db.Exec(`UPDATE software_catalog SET slug = ? WHERE id = ?`, slug, id); err != nil {
				log.Printf("WARN: failed to populate slug for software id=%d: %v", id, err)
			}
		}
		log.Printf("Populated slug for %d existing software entries", emptySlugCount)
	}

	// Create unique index on slug after population
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_software_catalog_slug ON software_catalog(slug)`); err != nil {
		return fmt.Errorf("failed to create unique index on software_catalog.slug: %w", err)
	}

	if !isPostgres {
		if _, err := db.Exec("ANALYZE"); err != nil {
			return fmt.Errorf("failed to run ANALYZE: %w", err)
		}
	}

	return SeedSchedules(db)
}
