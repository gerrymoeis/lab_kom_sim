package repository

import (
	"database/sql"
	"fmt"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
)

var globalUserCols = []string{
	"id", "username", "password", "full_name",
	"'admin' AS role",
	"is_super_admin", "is_protected", "is_global_admin", "session_token",
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

func (r *GlobalUserRepository) Create(username, hashedPassword, fullName string, isSuperAdmin, isGlobalAdmin bool) (sql.Result, error) {
	sa := 0
	if isSuperAdmin {
		sa = 1
	}
	ga := 0
	if isGlobalAdmin {
		ga = 1
	}
	return r.db.Exec(`INSERT INTO global_users (username, password, full_name, is_super_admin, is_global_admin, password_is_default) VALUES (?, ?, ?, ?, ?, 0)`,
		username, hashedPassword, fullName, sa, ga)
}

func (r *GlobalUserRepository) Update(id int, username, fullName string, isSuperAdmin, isGlobalAdmin bool) error {
	sa := 0
	if isSuperAdmin {
		sa = 1
	}
	ga := 0
	if isGlobalAdmin {
		ga = 1
	}
	_, err := r.db.Exec(`UPDATE global_users SET username = ?, full_name = ?, is_super_admin = ?, is_global_admin = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		username, fullName, sa, ga, id)
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

func (r *GlobalUserRepository) ListByLabPaginated(labURLPath, searchTerm, role, sortBy, sortOrder string, page, pageSize int) ([]models.GlobalUser, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	baseFrom := `FROM global_users gu JOIN lab_permissions lp ON lp.user_id = gu.id`
	baseWhere := ` WHERE lp.lab_url_path = ?`
	var args []any
	args = append(args, labURLPath)

	if searchTerm != "" {
		baseWhere += ` AND (gu.username LIKE ? OR gu.full_name LIKE ?)`
		s := "%" + searchTerm + "%"
		args = append(args, s, s)
	}
	if role != "" {
		baseWhere += ` AND lp.role = ?`
		args = append(args, role)
	}

	var total int
	countQ := `SELECT COUNT(*) ` + baseFrom + baseWhere
	if err := r.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	validSort := map[string]bool{"username": true, "full_name": true, "role": true, "created_at": true}
	if !validSort[sortBy] {
		sortBy = "created_at"
	}
	if sortOrder != "ASC" {
		sortOrder = "DESC"
	}

	sortCol := sortBy
	switch sortBy {
	case "username":
		sortCol = "gu.username"
	case "full_name":
		sortCol = "gu.full_name"
	case "role":
		sortCol = "lp.role"
	case "created_at":
		sortCol = "gu.created_at"
	}

	q := `SELECT gu.id, gu.username, gu.full_name, lp.role,
	             gu.is_protected, gu.is_super_admin, gu.is_global_admin, gu.created_at
	       ` + baseFrom + baseWhere + ` ORDER BY ` + sortCol + ` ` + sortOrder + ` LIMIT ? OFFSET ?`
	queryArgs := append(args, pageSize, offset)

	rows, err := r.db.Query(q, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []models.GlobalUser
	for rows.Next() {
		var u models.GlobalUser
		if err := rows.Scan(&u.ID, &u.Username, &u.FullName, &u.Role,
			&u.IsProtected, &u.IsSuperAdmin, &u.IsGlobalAdmin, &u.CreatedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, nil
}

func (r *GlobalUserRepository) GetByUsernameAndLab(username, labURLPath string) (*models.GlobalUser, error) {
	var u models.GlobalUser
	err := r.db.QueryRow(`
		SELECT gu.id, gu.username, gu.full_name, lp.role,
		       gu.is_protected, gu.is_super_admin, gu.is_global_admin, gu.created_at, gu.updated_at
		FROM global_users gu
		JOIN lab_permissions lp ON lp.user_id = gu.id
		WHERE gu.username = ? AND lp.lab_url_path = ?
	`, username, labURLPath).Scan(&u.ID, &u.Username, &u.FullName, &u.Role,
		&u.IsProtected, &u.IsSuperAdmin, &u.IsGlobalAdmin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *GlobalUserRepository) GetUsernamesByLab(labURLPath string) ([]string, error) {
	rows, err := r.db.Query(`
		SELECT gu.username FROM global_users gu
		JOIN lab_permissions lp ON lp.user_id = gu.id
		WHERE lp.lab_url_path = ?
		ORDER BY gu.username
	`, labURLPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var usernames []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		usernames = append(usernames, u)
	}
	return usernames, rows.Err()
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
