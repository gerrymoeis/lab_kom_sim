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

		// Device Types table (templates/presets)
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

		// Devices table (main table)
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

		// Device Loans table (peminjaman)
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

		// Device Usages table (pemakaian habis pakai)
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
		// Device management indexes
		`CREATE INDEX IF NOT EXISTS idx_device_types_category ON device_types(category)`,
		`CREATE INDEX IF NOT EXISTS idx_device_types_item_type ON device_types(item_type)`,
		// Device indexes (category removed - derived from device_types via JOIN)
		`CREATE INDEX IF NOT EXISTS idx_devices_condition ON devices(condition)`,
		`CREATE INDEX IF NOT EXISTS idx_device_loans_device_id ON device_loans(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_device_loans_status ON device_loans(status)`,
		`CREATE INDEX IF NOT EXISTS idx_device_loans_loan_date ON device_loans(loan_date)`,
		`CREATE INDEX IF NOT EXISTS idx_device_usages_device_id ON device_usages(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_device_usages_usage_date ON device_usages(usage_date)`,
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

	// Add new columns for devices table (device management system)
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
		// Device indexes (created after ALTER TABLE)
		`CREATE INDEX IF NOT EXISTS idx_devices_device_type_id ON devices(device_type_id)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_asset_code ON devices(asset_code)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_item_type ON devices(item_type)`,
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

	// Seed device_types if empty
	var deviceTypeCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM device_types`).Scan(&deviceTypeCount)
	if err == nil && deviceTypeCount == 0 {
		seedDeviceTypes := []string{
			// === PERIPHERAL ===
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('Pen Tab Wacom Intuos', 'peripheral', 'Wacom', 'Intuos', 'individual', 1, 0, 'PENTAB', 'Lemari Kaca', 'Untuk menggantikan mouse dan memungkinkan pengguna membuat karya seni digital, ilustrasi, dan desain dengan presisi tinggi.')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Mouse Axioo', 'peripheral', 'Axioo', NULL, 'individual', 1, 0, 'MOUSE', 'Lemari Kaca')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Keyboard Axioo', 'peripheral', 'Axioo', NULL, 'individual', 1, 0, 'KEYBOARD', 'Lemari Kaca')`,
			
			// === NETWORK ===
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('Switch Ruijie RG-ES116G 16 Port', 'network', 'Ruijie', 'RG-ES116G', 'individual', 1, 0, 'SWITCH-RJ16', 'Lemari Kaca', 'Switch gigabit non-PoE unmanaged dengan 16 port 10/100/1000Mbps untuk jaringan stabil.')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Switch Ruijie 48 Port', 'network', 'Ruijie', NULL, 'individual', 1, 0, 'SWITCH-RJ48', 'Lemari Kaca')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Switch Gigabit Linksys LGS108AP 8 Port', 'network', 'Linksys', 'LGS108AP', 'individual', 1, 0, 'SWITCH-LK', 'Lemari Kaca')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('Router Nirkabel MikroTik hAP lite', 'network', 'MikroTik', 'RB941-2nD', 'individual', 1, 0, 'ROUTER-MT', 'Lemari Kaca', 'Memiliki 4 port Fast Ethernet dan satu titik akses nirkabel 2.4 GHz dengan RouterOS.')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('Ubiquiti UniFi Access Point U6-LR', 'network', 'Ubiquiti', 'U6-LR', 'individual', 1, 0, 'UNIFI-AP', 'Lemari Kaca', 'Wireless access point untuk menyediakan konektivitas Wi-Fi.')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('PoE Adapter Ubiquiti U-POE-AF', 'network', 'Ubiquiti', 'U-POE-AF', 'individual', 1, 0, 'POE-UBNT', 'Lemari Kaca', 'PoE Injector untuk menyalurkan daya listrik melalui kabel UTP ke access point atau kamera CCTV.')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('Kabel UTP Belden CAT6', 'network', 'Belden', 'CAT6 NON PLENUM', 'consumable', 0, 1, 'CABLE-UTP', 'Lemari Kaca', 'Media transmisi data dalam jaringan komputer. Kemasan 305 meter (1000 kaki) per roll.')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('RJ45 Connectors', 'network', 'NYK', NULL, 'consumable', 0, 1, 'CONN-RJ45', 'Lemari Kaca', 'Konektor kabel ethernet untuk membuat kabel patch jaringan. 100 buah per kotak.')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('RJ45 Plug Boot Belden', 'network', 'Belden', 'AP700021', 'consumable', 0, 1, 'BOOT-RJ45', 'Lemari Kaca')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('Crimping RJ45', 'network', 'Ou Bao', NULL, 'individual', 1, 0, 'CRIMP-RJ45', 'Lemari Kaca', 'Kompatibel dengan konektor RJ45, RJ11, dan RJ12. Dilengkapi dengan pemotong dan pengupas kawat.')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('Penguji Kabel LAN', 'network', 'MAXLINE', 'NSS-468A', 'individual', 1, 0, 'TESTER-LAN', 'Lemari Kaca', 'Untuk memeriksa konektivitas RJ45 dan RJ11 dengan indikator LED.')`,
			
			// === POWER ===
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Adaptor PC Set Axioo', 'power', 'Axioo', NULL, 'individual', 1, 0, 'ADAPTOR-PC', 'Lemari Kaca')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('UPS APC Easy UPS', 'power', 'APC', 'Easy UPS', 'individual', 1, 0, 'UPS-APC', 'Lemari Kaca', 'Menyediakan daya cadangan dan melindungi perangkat dari lonjakan atau pemadaman listrik.')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Kabel Listrik SPEDER Cable', 'power', 'SPEDER', 'MONSTER', 'consumable', 0, 1, 'CABLE-PWR', 'Lemari Kaca')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Kabel Listrik BLITZ NYYHY', 'power', 'BLITZ', 'NYYHY 2x2.5mm', 'consumable', 0, 1, 'CABLE-NYY', 'Lemari Kaca')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Stop Kontak OB isi 4', 'power', 'UTICON', NULL, 'individual', 1, 0, 'STOPKONTAK', 'Lemari Kaca')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Steker Arde', 'power', 'MEVAL', NULL, 'individual', 1, 0, 'STEKER', 'Lemari Kaca')`,
			
			// === DISPLAY ===
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('Proyektor Hitachi', 'display', 'HITACHI', NULL, 'individual', 1, 0, 'PROJ-HTC', 'Tergantung di atap', 'Resolusi XGA (1024 x 768), kecerahan 2700 ANSI lumens, teknologi 3LCD. Konektivitas: HDMI, USB, VGA, Composite Video, Audio In/Out, RS-232C, RJ-45.')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Kabel HDMI 10 meter', 'display', 'HDTV Premium', NULL, 'individual', 1, 0, 'HDMI-10M', 'Lemari Kaca')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Kabel HDMI 20 meter', 'display', 'VENTION', NULL, 'individual', 1, 0, 'HDMI-20M', 'Lemari Kaca')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Kabel VGA', 'display', NULL, NULL, 'individual', 1, 0, 'CABLE-VGA', 'Lemari Kaca')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Wall Socket Module HDMI Websong', 'display', 'Websong', NULL, 'individual', 1, 0, 'SOCKET-HDMI', 'Dinding')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Remote Proyektor Hitachi', 'display', 'Hitachi', 'R017F', 'individual', 1, 0, 'REMOTE-PROJ', 'Lemari Kaca')`,
			
			// === PRINTER ===
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('Printer EPSON EcoTank L3250', 'printer', 'EPSON', 'EcoTank L3250', 'individual', 0, 0, 'PRINTER-EP', 'Ruang Lab', 'Printer Multifungsi (Print, Scan, Copy) dengan teknologi EcoTank. Konektivitas Wi-Fi dan Wi-Fi Direct. Kapasitas cetak: 4.500 halaman hitam/putih, 7.500 halaman warna.')`,
			
			// === CONSUMABLE ===
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('CD/DVD Windows Driver Set', 'consumable', 'Axioo', NULL, 'consumable', 1, 1, 'MEDIA-DVD', 'Lemari Kaca', 'DVD Windows Driver. Satu set dalam plastik.')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Isolasi Bening', 'consumable', NULL, NULL, 'consumable', 0, 1, 'ISOLASI-BEN', 'Lemari Kayu no.2')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Isolasi Hitam', 'consumable', NULL, NULL, 'consumable', 0, 1, 'ISOLASI-HTM', 'Lemari Kayu no.2')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Double Tip Kecil', 'consumable', NULL, NULL, 'consumable', 0, 1, 'DOUBLETIP', 'Lemari Kayu no.2')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('MicroSD Card SanDisk 512GB', 'consumable', 'SanDisk', 'Ultra 512GB UHS-I Class 10', 'consumable', 1, 1, 'SDCARD', 'Lemari Kaca', 'Media penyimpanan untuk kamera CCTV. Kecepatan baca hingga 100 MB/s.')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('Hard Disk Drive Seagate SkyHawk 6TB', 'consumable', 'Seagate', 'SkyHawk 6TB', 'consumable', 0, 1, 'HDD-SATA', 'Lemari Kaca', 'HDD internal SATA untuk surveillance recording 24/7 di sistem CCTV/DVR/NVR.')`,
			
			// === AUDIO (NEW CATEGORY) ===
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Speaker', 'audio', NULL, NULL, 'individual', 1, 0, 'SPEAKER', 'Lemari Kaca')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('Loudspeaker System JBL', 'audio', 'JBL', 'Pasión', 'individual', 1, 0, 'SPEAKER-JBL', 'Ruang Lab', 'Loudspeaker Pasif (membutuhkan amplifier eksternal) dirancang oleh HARMAN.')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Braket Speaker Dinding BMB', 'audio', 'BMB', NULL, 'individual', 1, 0, 'BRAKET-SPK', 'Ruang Lab')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('Mixing Amplifier HA AUDIO MA-2600', 'audio', 'HA AUDIO', 'MA-2600', 'individual', 1, 0, 'AMP-MIXER', 'Ruang Lab', 'Power Amplifier dan Mixing untuk Loudspeaker. Fitur Digital Korea Echo untuk Karaoke.')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('Mikrofon Nirkabel Champion 1', 'audio', 'Champion', 'Dual Channel UHF/PLL', 'individual', 1, 0, 'MIC-WIRELESS', 'Ruang Lab', 'Mikrofon Nirkabel Profesional dengan teknologi UHF/PLL Dual Channel.')`,
			
			// === TOOLS (NEW CATEGORY) ===
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('Hydraulic Crimping Tool YQK-240', 'tools', 'YQK', 'YQK-240', 'individual', 1, 0, 'CRIMP-HYD', 'Lemari Kaca', 'Alat Press Hidrolik untuk menghubungkan kabel dengan konektor berukuran besar.')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Gunting', 'tools', NULL, NULL, 'individual', 1, 0, 'GUNTING', 'Lemari Kaca')`,
			
			// === SERVER (NEW CATEGORY) ===
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('Server Komputer DELL', 'server', 'DELL Technologies', NULL, 'individual', 0, 0, 'SERVER-DELL', 'Ruang Server', 'Pusat komputasi dan penyimpanan data untuk Laboratorium Komputer. Tipe Rack-Mount.')`,
			
			// === SECURITY (NEW CATEGORY) ===
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('Kamera CCTV HIKVISION Smart Hybrid Light PT', 'security', 'HIKVISION', 'Smart Hybrid Light PT', 'individual', 0, 0, 'CCTV-CAM', 'Ruang Lab', 'PT Camera (Pan/Tilt) dengan teknologi Smart Hybrid Light untuk pengawasan area luas.')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) 
			 VALUES ('Digital Video Recorder HIKVISION AcuSense', 'security', 'HIKVISION', 'AcuSense TURBO HD PRO', 'individual', 0, 0, 'DVR-HIKV', 'Ruang Server', 'Perekam video dan hub sentral untuk kamera CCTV. Mendukung TURBO HD dan Hybrid dengan teknologi AI AcuSense.')`,
			
			// === STATIONERY (NEW CATEGORY) ===
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Buku Besar', 'stationery', 'Paperline', NULL, 'individual', 0, 0, 'BUKU', 'Meja Laboran')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Bolpoin', 'stationery', 'Snowman', NULL, 'consumable', 0, 1, 'BOLPOIN', 'Meja Laboran')`,
			
			`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location) 
			 VALUES ('Penggaris', 'stationery', 'Microtop', NULL, 'individual', 1, 0, 'PENGGARIS', 'Lemari Kayu no.2')`,
		}
		
		for _, seedSQL := range seedDeviceTypes {
			if _, err := db.Exec(seedSQL); err != nil {
				fmt.Printf("Warning: Failed to seed device_types: %v\n", err)
			}
		}
		fmt.Println("✅ Seeded device_types with 46 templates from real inventory data")
	}

	return nil
}
