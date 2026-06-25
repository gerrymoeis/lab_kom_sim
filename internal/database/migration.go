package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"inventaris-lab-kom/internal/util"
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
				user_id INTEGER NOT NULL ,
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
		`CREATE TABLE IF NOT EXISTS pcs (
			id {{PK}},
			{{ROW}} INTEGER NOT NULL DEFAULT 0 CHECK({{ROW}} >= 0),
			{{COL}} INTEGER NOT NULL DEFAULT 0 CHECK({{COL}} >= 0),
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
		`CREATE TABLE IF NOT EXISTS sticker_templates (
			id {{PK}},
			name TEXT NOT NULL,
			sticker_type TEXT NOT NULL CHECK(sticker_type IN ('pc', 'device')),
			font_size_cm REAL NOT NULL DEFAULT 1.0,
			padding_h_cm REAL NOT NULL DEFAULT 0.5,
			padding_v_cm REAL NOT NULL DEFAULT 0.8,
			created_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			updated_at {{TS}} DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(name, sticker_type)
		)`,
		`CREATE TABLE IF NOT EXISTS activity_logs (
			id {{PK}},
			user_id INTEGER NOT NULL ,
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
				{{COL}} INTEGER NOT NULL DEFAULT 0 CHECK({{COL}} >= 0),
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
			// Generate slug using util.Slugify (single source of truth)
			slug := util.Slugify(name)

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

	// Fix activity_logs.user_id NOT NULL — conflicts with ON DELETE SET NULL.
	// When a user is deleted, SQLite tries to SET NULL on activity_logs.user_id,
	// but NOT NULL prevents it → FK violation → DELETE fails silently.
	if !isPostgres {
		var alSQL string
		if rErr := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='activity_logs'`).Scan(&alSQL); rErr == nil && strings.Contains(alSQL, "user_id INTEGER NOT NULL") {
			actLogSQL := `CREATE TABLE IF NOT EXISTS activity_logs_v2 (
				id {{PK}},
				user_id INTEGER ,
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
				log.Println("Migrated activity_logs.user_id — removed NOT NULL for ON DELETE SET NULL compatibility")
			}
		}
	}

	if err := normalizeExistingData(db); err != nil {
		return fmt.Errorf("failed to normalize existing data: %w", err)
	}

	if !isPostgres {
		if _, err := db.Exec("ANALYZE"); err != nil {
			return fmt.Errorf("failed to run ANALYZE: %w", err)
		}
	}

	return nil
}

func toTitleCase(s string) string {
	return util.ToTitleCase(s)
}

func toTitleCaseWithAbbr(s string) string {
	return util.ToTitleCaseWithAbbr(s)
}

func sanitizeText(s string) string {
	return util.SanitizeText(s)
}

func toUpperTrim(s string) string {
	return util.ToUpperTrim(s)
}

func generateSlug(s string) string {
	return util.Slugify(s)
}

func normalizeExistingData(db *DB) error {
	log.Println("Normalizing existing data...")
	if err := normalizeCategories(db); err != nil {
		return err
	}
	if err := normalizeDeviceTypes(db); err != nil {
		return err
	}
	if err := normalizeDevices(db); err != nil {
		return err
	}
	if err := normalizeDeviceLoans(db); err != nil {
		return err
	}
	if err := normalizeDeviceUsages(db); err != nil {
		return err
	}
	if err := normalizeDeviceInstallations(db); err != nil {
		return err
	}
	if err := normalizeSchedules(db); err != nil {
		return err
	}
	if err := normalizeSoftwareCatalogs(db); err != nil {
		return err
	}
	if err := normalizePCs(db); err != nil {
		return err
	}
	if err := normalizeLogbookEntries(db); err != nil {
		return err
	}
	return nil
}

func normalizeCategories(db *DB) error {
	rows, err := db.Query("SELECT id, name, default_prefix FROM categories")
	if err != nil {
		return err
	}
	defer rows.Close()
	var count int
	for rows.Next() {
		var id int
		var name, prefix string
		if rows.Scan(&id, &name, &prefix) != nil {
			continue
		}
		newName := toTitleCase(name)
		newPrefix := toUpperTrim(prefix)
		if newName != name || newPrefix != prefix {
			db.Exec("UPDATE categories SET name = ?, default_prefix = ? WHERE id = ?", newName, newPrefix, id)
			count++
		}
	}
	if count > 0 {
		log.Printf("  Normalized %d categories", count)
	}
	return nil
}

func normalizeDeviceTypes(db *DB) error {
	rows, err := db.Query("SELECT id, name, COALESCE(brand,''), COALESCE(model,''), asset_code_prefix, COALESCE(default_location,'') FROM device_types")
	if err != nil {
		return err
	}
	defer rows.Close()
	var count int
	for rows.Next() {
		var id int
		var name, brand, model, prefix, loc string
		if rows.Scan(&id, &name, &brand, &model, &prefix, &loc) != nil {
			continue
		}
		newName := toTitleCase(name)
		newBrand := toTitleCase(brand)
		newModel := toTitleCase(model)
		newPrefix := toUpperTrim(prefix)
		newLoc := toTitleCase(loc)
		if newName != name || newBrand != brand || newModel != model || newPrefix != prefix || newLoc != loc {
			db.Exec("UPDATE device_types SET name = ?, brand = ?, model = ?, asset_code_prefix = ?, default_location = ? WHERE id = ?",
				newName, newBrand, newModel, newPrefix, newLoc, id)
			count++
		}
	}
	if count > 0 {
		log.Printf("  Normalized %d device_types", count)
	}
	return nil
}

func normalizeDevices(db *DB) error {
	rows, err := db.Query("SELECT id, COALESCE(location,''), COALESCE(notes,''), COALESCE(serial_number,'') FROM devices")
	if err != nil {
		return err
	}
	defer rows.Close()
	var count int
	for rows.Next() {
		var id int
		var loc, notes, serial string
		if rows.Scan(&id, &loc, &notes, &serial) != nil {
			continue
		}
		newLoc := toTitleCase(loc)
		newNotes := sanitizeText(notes)
		newSerial := sanitizeText(serial)
		if newLoc != loc || newNotes != notes || newSerial != serial {
			db.Exec("UPDATE devices SET location = ?, notes = ?, serial_number = ? WHERE id = ?",
				newLoc, newNotes, newSerial, id)
			count++
		}
	}
	if count > 0 {
		log.Printf("  Normalized %d devices", count)
	}
	return nil
}

func normalizeDeviceLoans(db *DB) error {
	rows, err := db.Query("SELECT id, COALESCE(borrower_name,''), COALESCE(purpose,''), COALESCE(notes,'') FROM device_loans")
	if err != nil {
		return err
	}
	defer rows.Close()
	var count int
	for rows.Next() {
		var id int
		var name, purpose, notes string
		if rows.Scan(&id, &name, &purpose, &notes) != nil {
			continue
		}
		newName := toTitleCaseWithAbbr(name)
		newPurpose := sanitizeText(purpose)
		newNotes := sanitizeText(notes)
		if newName != name || newPurpose != purpose || newNotes != notes {
			db.Exec("UPDATE device_loans SET borrower_name = ?, purpose = ?, notes = ? WHERE id = ?",
				newName, newPurpose, newNotes, id)
			count++
		}
	}
	if count > 0 {
		log.Printf("  Normalized %d device_loans", count)
	}
	return nil
}

func normalizeDeviceUsages(db *DB) error {
	rows, err := db.Query("SELECT id, COALESCE(user_name,''), COALESCE(purpose,''), COALESCE(notes,'') FROM device_usages")
	if err != nil {
		return err
	}
	defer rows.Close()
	var count int
	for rows.Next() {
		var id int
		var name, purpose, notes string
		if rows.Scan(&id, &name, &purpose, &notes) != nil {
			continue
		}
		newName := toTitleCaseWithAbbr(name)
		newPurpose := sanitizeText(purpose)
		newNotes := sanitizeText(notes)
		if newName != name || newPurpose != purpose || newNotes != notes {
			db.Exec("UPDATE device_usages SET user_name = ?, purpose = ?, notes = ? WHERE id = ?",
				newName, newPurpose, newNotes, id)
			count++
		}
	}
	if count > 0 {
		log.Printf("  Normalized %d device_usages", count)
	}
	return nil
}

func normalizeDeviceInstallations(db *DB) error {
	rows, err := db.Query("SELECT id, COALESCE(location_installed,''), COALESCE(notes,'') FROM device_installations")
	if err != nil {
		return err
	}
	defer rows.Close()
	var count int
	for rows.Next() {
		var id int
		var loc, notes string
		if rows.Scan(&id, &loc, &notes) != nil {
			continue
		}
		newLoc := toTitleCase(loc)
		newNotes := sanitizeText(notes)
		if newLoc != loc || newNotes != notes {
			db.Exec("UPDATE device_installations SET location_installed = ?, notes = ? WHERE id = ?",
				newLoc, newNotes, id)
			count++
		}
	}
	if count > 0 {
		log.Printf("  Normalized %d device_installations", count)
	}
	return nil
}

func normalizeSchedules(db *DB) error {
	rows, err := db.Query("SELECT id, COALESCE(course_name,''), COALESCE(lecturer,''), COALESCE(class,''), COALESCE(notes,'') FROM course_schedules")
	if err != nil {
		return err
	}
	defer rows.Close()
	var count int
	for rows.Next() {
		var id int
		var course, lecturer, class, notes string
		if rows.Scan(&id, &course, &lecturer, &class, &notes) != nil {
			continue
		}
		newCourse := toTitleCase(course)
		newLecturer := toTitleCaseWithAbbr(lecturer)
		newClass := strings.ToUpper(strings.TrimSpace(class))
		newNotes := sanitizeText(notes)
		if newCourse != course || newLecturer != lecturer || newClass != class || newNotes != notes {
			db.Exec("UPDATE course_schedules SET course_name = ?, lecturer = ?, class = ?, notes = ? WHERE id = ?",
				newCourse, newLecturer, newClass, newNotes, id)
			count++
		}
	}
	if count > 0 {
		log.Printf("  Normalized %d course_schedules", count)
	}
	return nil
}

func normalizeSoftwareCatalogs(db *DB) error {
	rows, err := db.Query("SELECT id, COALESCE(name,''), COALESCE(description,''), slug FROM software_catalog")
	if err != nil {
		return err
	}
	defer rows.Close()
	var updated int
	for rows.Next() {
		var id int
		var name, desc, oldSlug string
		if rows.Scan(&id, &name, &desc, &oldSlug) != nil {
			continue
		}
		newName := toTitleCase(name)
		newDesc := sanitizeText(desc)
		newSlug := util.Slugify(newName)
		if newName == name && newDesc == desc && newSlug == oldSlug {
			continue
		}

		_, err := db.Exec("UPDATE software_catalog SET name = ?, description = ?, slug = ? WHERE id = ?",
			newName, newDesc, newSlug, id)
		if err == nil {
			updated++
			continue
		}

		errStr := err.Error()

		if strings.Contains(errStr, "software_catalog.name") {
			var targetID int
			if err2 := db.QueryRow("SELECT id FROM software_catalog WHERE name = ? AND id != ?", newName, id).Scan(&targetID); err2 == nil {
				mergeSoftwareRows(db, id, targetID)
				updated++
				continue
			}
		}

		if strings.Contains(errStr, "software_catalog.slug") {
			suffixSlug := newSlug + "-" + fmt.Sprintf("%d", id)
			if _, err2 := db.Exec("UPDATE software_catalog SET name = ?, description = ?, slug = ? WHERE id = ?",
				newName, newDesc, suffixSlug, id); err2 == nil {
				updated++
				continue
			}
		}

		log.Printf("WARN: failed to normalize software id=%d: %v", id, err)
	}
	if updated > 0 {
		log.Printf("  Normalized %d software_catalogs", updated)
	}
	return nil
}

func mergeSoftwareRows(db *DB, fromID, intoID int) {
	rows, err := db.Query("SELECT pc_id FROM pc_software WHERE software_id = ? AND installed = TRUE", fromID)
	if err == nil {
		for rows.Next() {
			var pcID int
			if rows.Scan(&pcID) == nil {
				db.Exec("INSERT OR IGNORE INTO pc_software (pc_id, software_id, installed) VALUES (?, ?, TRUE)", pcID, intoID)
			}
		}
		rows.Close()
	}
	db.Exec("DELETE FROM software_catalog WHERE id = ?", fromID)
	log.Printf("  Merged software id=%d into id=%d", fromID, intoID)
}

func normalizePCs(db *DB) error {
	rows, err := db.Query(`SELECT id, COALESCE(processor,''), COALESCE(ram,''), COALESCE(storage,''),
		COALESCE(serial_number,''), COALESCE(operating_system,''), COALESCE(pc_type,''),
		COALESCE(brand_model,''), COALESCE(accessories,''), COALESCE(notes,'') FROM pcs`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var count int
	for rows.Next() {
		var id int
		var cpu, ram, storage, serial, os, pct, brand, acc, notes string
		if rows.Scan(&id, &cpu, &ram, &storage, &serial, &os, &pct, &brand, &acc, &notes) != nil {
			continue
		}
		newCPU := sanitizeText(cpu)
		newRAM := sanitizeText(ram)
		newStorage := sanitizeText(storage)
		newSerial := sanitizeText(serial)
		newOS := sanitizeText(os)
		newPCT := sanitizeText(pct)
		newBrand := sanitizeText(brand)
		newAcc := sanitizeText(acc)
		newNotes := sanitizeText(notes)
		if newCPU != cpu || newRAM != ram || newStorage != storage || newSerial != serial ||
			newOS != os || newPCT != pct || newBrand != brand || newAcc != acc || newNotes != notes {
			db.Exec("UPDATE pcs SET processor = ?, ram = ?, storage = ?, serial_number = ?, operating_system = ?, pc_type = ?, brand_model = ?, accessories = ?, notes = ? WHERE id = ?",
				newCPU, newRAM, newStorage, newSerial, newOS, newPCT, newBrand, newAcc, newNotes, id)
			count++
		}
	}
	if count > 0 {
		log.Printf("  Normalized %d pcs", count)
	}
	return nil
}

func normalizeLogbookEntries(db *DB) error {
	rows, err := db.Query("SELECT id, COALESCE(student_name,''), COALESCE(nim,''), COALESCE(purpose,'') FROM logbook_entries")
	if err != nil {
		return err
	}
	defer rows.Close()
	var count int
	for rows.Next() {
		var id int
		var name, nim, purpose string
		if rows.Scan(&id, &name, &nim, &purpose) != nil {
			continue
		}
		newName := toTitleCaseWithAbbr(name)
		newNIM := strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(nim, " ", "")))
		newPurpose := toTitleCase(purpose)
		if newName != name || newNIM != nim || newPurpose != purpose {
			db.Exec("UPDATE logbook_entries SET student_name = ?, nim = ?, purpose = ? WHERE id = ?",
				newName, newNIM, newPurpose, id)
			count++
		}
	}
	if count > 0 {
		log.Printf("  Normalized %d logbook_entries", count)
	}
	return nil
}
