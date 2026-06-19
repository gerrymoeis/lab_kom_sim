package database

func RunGlobalMigrations(db *DB) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS global_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			full_name TEXT NOT NULL DEFAULT '',
			is_super_admin INTEGER NOT NULL DEFAULT 0,
			is_protected INTEGER NOT NULL DEFAULT 0,
			session_token TEXT DEFAULT '',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS lab_permissions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES global_users(id),
			lab_url_path TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'dosen' CHECK(role IN ('admin', 'dosen')),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, lab_url_path)
		)`,
		`CREATE TABLE IF NOT EXISTS grid_layouts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			lab_url_path TEXT NOT NULL UNIQUE,
			cols_per_row TEXT NOT NULL DEFAULT '[8,8,8,8,8]',
			has_gap INTEGER NOT NULL DEFAULT 0,
			gap_pos INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS lab_configs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			lab_id TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			url_path TEXT NOT NULL UNIQUE,
			db_path TEXT NOT NULL,
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, t := range tables {
		if _, err := db.Exec(t); err != nil {
			return err
		}
	}
	return nil
}
