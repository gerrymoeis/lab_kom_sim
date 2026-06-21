package repository

import (
	"database/sql"
	"fmt"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
)

var globalUserCols = []string{
	"id", "username", "password", "full_name",
	"is_super_admin", "is_protected", "session_token",
	"password_is_default",
	"created_at", "updated_at",
}

var labPermissionCols = []string{
	"id", "user_id", "lab_url_path", "role",
	"is_main_account",
	"created_at",
}

type GlobalUserRepository struct {
	db DBTX
}

func NewGlobalUserRepository(db *database.DB) *GlobalUserRepository {
	return &GlobalUserRepository{db: db}
}

func (r *GlobalUserRepository) GetByUsername(username string) (*models.GlobalUser, error) {
	return getOne[models.GlobalUser](r.db, "global_users", globalUserCols, "username = ?", username)
}

func (r *GlobalUserRepository) GetByID(id int) (*models.GlobalUser, error) {
	return getOne[models.GlobalUser](r.db, "global_users", globalUserCols, "id = ?", id)
}

func (r *GlobalUserRepository) Create(username, hashedPassword, fullName string, isSuperAdmin bool) (sql.Result, error) {
	sa := 0
	if isSuperAdmin {
		sa = 1
	}
	return r.db.Exec(`INSERT INTO global_users (username, password, full_name, is_super_admin, password_is_default) VALUES (?, ?, ?, ?, 0)`,
		username, hashedPassword, fullName, sa)
}

func (r *GlobalUserRepository) Update(id int, username, fullName string, isSuperAdmin bool) error {
	sa := 0
	if isSuperAdmin {
		sa = 1
	}
	_, err := r.db.Exec(`UPDATE global_users SET username = ?, full_name = ?, is_super_admin = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		username, fullName, sa, id)
	return err
}

func (r *GlobalUserRepository) UpdatePassword(id int, hashedPassword string) error {
	_, err := r.db.Exec(`UPDATE global_users SET password = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, hashedPassword, id)
	return err
}

func (r *GlobalUserRepository) UpdateSessionToken(id int, token string) error {
	_, err := r.db.Exec(`UPDATE global_users SET session_token = ? WHERE id = ?`, token, id)
	return err
}

func (r *GlobalUserRepository) Delete(id int) error {
	_, err := r.db.Exec(`DELETE FROM global_users WHERE id = ?`, id)
	return err
}

func (r *GlobalUserRepository) ClearSessionToken(id int) error {
	_, err := r.db.Exec(`UPDATE global_users SET session_token = '' WHERE id = ?`, id)
	return err
}

func (r *GlobalUserRepository) GetSessionToken(id int) (string, error) {
	var token string
	err := r.db.QueryRow(`SELECT session_token FROM global_users WHERE id = ?`, id).Scan(&token)
	return token, err
}

func (r *GlobalUserRepository) List() ([]models.GlobalUser, error) {
	return getAll[models.GlobalUser](r.db, "global_users", globalUserCols, "1=1 ORDER BY created_at DESC")
}

// --- Lab Permissions ---

func (r *GlobalUserRepository) GetPermissions(userID int) ([]models.LabPermission, error) {
	return getAll[models.LabPermission](r.db, "lab_permissions", labPermissionCols,
		"user_id = ? ORDER BY lab_url_path", userID)
}

func (r *GlobalUserRepository) HasPermission(userID int, labURLPath string) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM lab_permissions WHERE user_id = ? AND lab_url_path = ?`, userID, labURLPath).Scan(&count)
	return count > 0, err
}

func (r *GlobalUserRepository) AddPermission(userID int, labURLPath, role string) error {
	_, err := r.db.Exec(`INSERT OR IGNORE INTO lab_permissions (user_id, lab_url_path, role) VALUES (?, ?, ?)`, userID, labURLPath, role)
	return err
}

func (r *GlobalUserRepository) RemovePermission(userID int, labURLPath string) error {
	_, err := r.db.Exec(`DELETE FROM lab_permissions WHERE user_id = ? AND lab_url_path = ?`, userID, labURLPath)
	return err
}

func (r *GlobalUserRepository) ClearPermissions(userID int) error {
	_, err := r.db.Exec(`DELETE FROM lab_permissions WHERE user_id = ?`, userID)
	return err
}

func (r *GlobalUserRepository) GetMainAccountForLab(labURLPath string) (*models.LabPermission, error) {
	return getOne[models.LabPermission](r.db, "lab_permissions", labPermissionCols,
		"lab_url_path = ? AND is_main_account = 1", labURLPath)
}

func (r *GlobalUserRepository) ClearDefaultPasswordFlag(userID int) error {
	_, err := r.db.Exec(`UPDATE global_users SET password_is_default = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, userID)
	return err
}

func (r *GlobalUserRepository) GetUsersWithDefaultPassword() ([]models.DefaultCredential, error) {
	rows, err := r.db.Query(`
		SELECT gu.username, gu.is_super_admin,
		       COALESCE(lp.lab_url_path, '') AS lab_url_path,
		       COALESCE(lp.is_main_account, 0) AS is_main_account
		FROM global_users gu
		LEFT JOIN lab_permissions lp ON lp.user_id = gu.id AND lp.is_main_account = 1
		WHERE gu.password_is_default = 1
		ORDER BY gu.is_super_admin DESC, gu.username
	`)
	if err != nil {
		return nil, fmt.Errorf("query default password users: %w", err)
	}
	defer rows.Close()

	var results []models.DefaultCredential
	for rows.Next() {
		var d models.DefaultCredential
		var labURLPath string
		var isMainAcct int
		if err := rows.Scan(&d.Username, &d.IsSuperAdmin, &labURLPath, &isMainAcct); err != nil {
			return nil, fmt.Errorf("scan default credential: %w", err)
		}
		d.IsMainAccount = isMainAcct == 1
		results = append(results, d)
	}
	return results, nil
}
