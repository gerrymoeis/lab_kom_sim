package database

import (
	"encoding/json"
	"fmt"

	"inventaris-lab-kom/internal/config"

	"golang.org/x/crypto/bcrypt"
)

func SeedGlobalUsers(db *DB, labs []config.LabConfig) error {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM global_users").Scan(&count)
	if count > 0 {
		return nil
	}

	// 1. Super admin — akses semua lab + system features
	adminPass := env("ADMIN_PASSWORD", "admin123")
	hashedAdmin, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}
	res, err := db.Exec(`INSERT INTO global_users (username, password, full_name, is_super_admin, is_protected, password_is_default)
		VALUES ('admin', ?, 'Administrator', 1, 1, 1)`, string(hashedAdmin))
	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}
	adminID, _ := res.LastInsertId()

	// Super admin mendapat permission ke semua lab
	for _, lab := range labs {
		db.Exec(`INSERT OR IGNORE INTO lab_permissions (user_id, lab_url_path, role)
			VALUES (?, ?, 'admin')`, adminID, lab.URLPath)
	}

	// 2. Akun utama per lab — username = urlPath, password = urlPath + "123"
	for _, lab := range labs {
		mainPass := env("MAIN_ACCOUNT_PASSWORD", lab.URLPath+"123")
		hashedMain, err := bcrypt.GenerateFromPassword([]byte(mainPass), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash main account password for %s: %w", lab.URLPath, err)
		}

		var exists int
		db.QueryRow(`SELECT COUNT(*) FROM global_users WHERE username = ?`, lab.URLPath).Scan(&exists)
		if exists > 0 {
			continue
		}
		res, err := db.Exec(`INSERT INTO global_users (username, password, full_name, password_is_default)
			VALUES (?, ?, ?, 1)`, lab.URLPath, string(hashedMain), "Akun Utama "+lab.Title)
		if err != nil {
			return fmt.Errorf("failed to create main account %s: %w", lab.URLPath, err)
		}
		userID, _ := res.LastInsertId()
		db.Exec(`INSERT INTO lab_permissions (user_id, lab_url_path, role, is_main_account)
			VALUES (?, ?, 'admin', 1)`, userID, lab.URLPath)
	}

	// 3. Seed default grid layouts
	for _, lab := range labs {
		var exists int
		db.QueryRow(`SELECT COUNT(*) FROM grid_layouts WHERE lab_url_path = ?`, lab.URLPath).Scan(&exists)
		if exists > 0 {
			continue
		}
		layout := config.GetGridLayout(lab.URLPath)
		colsJSON, _ := json.Marshal(layout.ColsPerRow)
		hasGap := 0
		if layout.HasGap {
			hasGap = 1
		}
		db.Exec(`INSERT INTO grid_layouts (lab_url_path, cols_per_row, has_gap, gap_pos)
			VALUES (?, ?, ?, ?)`, lab.URLPath, string(colsJSON), hasGap, layout.GapPos)
	}

	return nil
}
