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
	res, err := db.Exec(`INSERT INTO global_users (username, password, full_name, is_super_admin, is_protected)
		VALUES ('admin', ?, 'Administrator', 1, 1)`, string(hashedAdmin))
	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}
	adminID, _ := res.LastInsertId()

	// Super admin mendapat permission ke semua lab
	for _, lab := range labs {
		db.Exec(`INSERT OR IGNORE INTO lab_permissions (user_id, lab_url_path, role)
			VALUES (?, ?, 'admin')`, adminID, lab.URLPath)
	}

	// 2. Lab admin per lab — hanya akses lab nya sendiri
	labPass := env("LAB_ADMIN_PASSWORD", "labadmin123")
	hashedLab, err := bcrypt.GenerateFromPassword([]byte(labPass), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash lab admin password: %w", err)
	}

	for _, lab := range labs {
		labAdminUser := lab.ID + "-admin"
		var exists int
		db.QueryRow(`SELECT COUNT(*) FROM global_users WHERE username = ?`, labAdminUser).Scan(&exists)
		if exists > 0 {
			continue
		}
		res, err := db.Exec(`INSERT INTO global_users (username, password, full_name)
			VALUES (?, ?, ?)`, labAdminUser, string(hashedLab), "Admin "+lab.Title)
		if err != nil {
			return fmt.Errorf("failed to create lab admin %s: %w", labAdminUser, err)
		}
		userID, _ := res.LastInsertId()
		db.Exec(`INSERT INTO lab_permissions (user_id, lab_url_path, role)
			VALUES (?, ?, 'admin')`, userID, lab.URLPath)
	}

	// 3. Seed lab_configs from env
	for _, lab := range labs {
		var exists int
		db.QueryRow(`SELECT COUNT(*) FROM lab_configs WHERE lab_id = ?`, lab.ID).Scan(&exists)
		if exists == 0 {
			db.Exec(`INSERT INTO lab_configs (lab_id, title, url_path, db_path)
				VALUES (?, ?, ?, ?)`, lab.ID, lab.Title, lab.URLPath, lab.DBPath)
		}
	}

	// 4. Seed default grid layouts
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
