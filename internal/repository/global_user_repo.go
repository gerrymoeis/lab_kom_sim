package repository

import (
	"database/sql"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
)

type GlobalUserRepository struct {
	db DBTX
}

func NewGlobalUserRepository(db *database.DB) *GlobalUserRepository {
	return &GlobalUserRepository{db: db}
}

func (r *GlobalUserRepository) GetByUsername(username string) (*models.GlobalUser, error) {
	return getOne[models.GlobalUser](r.db, "global_users", []string{
		"id", "username", "password", "full_name",
		"is_super_admin", "is_protected", "session_token",
		"created_at", "updated_at",
	}, "username = ?", username)
}

func (r *GlobalUserRepository) GetByID(id int) (*models.GlobalUser, error) {
	return getOne[models.GlobalUser](r.db, "global_users", []string{
		"id", "username", "password", "full_name",
		"is_super_admin", "is_protected", "session_token",
		"created_at", "updated_at",
	}, "id = ?", id)
}

func (r *GlobalUserRepository) Create(username, hashedPassword, fullName string, isSuperAdmin bool) (sql.Result, error) {
	sa := 0
	if isSuperAdmin {
		sa = 1
	}
	return r.db.Exec(`INSERT INTO global_users (username, password, full_name, is_super_admin) VALUES (?, ?, ?, ?)`,
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

func (r *GlobalUserRepository) ClearSessionToken(id int) error {
	_, err := r.db.Exec(`UPDATE global_users SET session_token = NULL WHERE id = ?`, id)
	return err
}

func (r *GlobalUserRepository) GetSessionToken(id int) (string, error) {
	var token string
	err := r.db.QueryRow(`SELECT session_token FROM global_users WHERE id = ?`, id).Scan(&token)
	return token, err
}

func (r *GlobalUserRepository) List() ([]models.GlobalUser, error) {
	return getAll[models.GlobalUser](r.db, "global_users", []string{
		"id", "username", "password", "full_name",
		"is_super_admin", "is_protected", "session_token",
		"created_at", "updated_at",
	}, "1=1 ORDER BY created_at DESC")
}

// --- Lab Permissions ---

func (r *GlobalUserRepository) GetPermissions(userID int) ([]models.LabPermission, error) {
	return getAll[models.LabPermission](r.db, "lab_permissions",
		[]string{"id", "user_id", "lab_url_path", "role", "created_at"},
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
