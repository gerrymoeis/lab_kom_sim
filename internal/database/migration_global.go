package database

import (
	"database/sql"
	"fmt"
)

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
			role TEXT NOT NULL DEFAULT 'admin' CHECK(role IN ('admin')),
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

	}

	for _, t := range tables {
		if _, err := db.Exec(t); err != nil {
			return err
		}
	}

	// Add is_main_account to lab_permissions
	if !colExistsGlobal(db, "lab_permissions", "is_main_account") {
		if _, err := db.Exec(`ALTER TABLE lab_permissions ADD COLUMN is_main_account INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("failed to add is_main_account to lab_permissions: %w", err)
		}
	}

	// Add password_is_default to global_users
	if !colExistsGlobal(db, "global_users", "password_is_default") {
		if _, err := db.Exec(`ALTER TABLE global_users ADD COLUMN password_is_default INTEGER NOT NULL DEFAULT 1`); err != nil {
			return fmt.Errorf("failed to add password_is_default to global_users: %w", err)
		}
	}

	return nil
}

func colExistsGlobal(db *DB, table, col string) bool {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			continue
		}
		if name == col {
			return true
		}
	}
	return false
}
