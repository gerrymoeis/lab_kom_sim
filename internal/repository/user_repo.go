package repository

import (
	"database/sql"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
)

type UserRepository struct {
	globalDB   *database.DB
	labURLPath string
}

func NewUserRepository(globalDB *database.DB, labURLPath string) *UserRepository {
	return &UserRepository{globalDB: globalDB, labURLPath: labURLPath}
}

func (r *UserRepository) List() ([]models.GlobalUser, error) {
	rows, err := r.globalDB.Query(`
		SELECT gu.id, gu.username, gu.full_name, lp.role,
		       gu.is_protected, gu.is_super_admin, gu.created_at
		FROM global_users gu
		JOIN lab_permissions lp ON lp.user_id = gu.id
		WHERE lp.lab_url_path = ?
		ORDER BY gu.created_at DESC
	`, r.labURLPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.GlobalUser
	for rows.Next() {
		var u models.GlobalUser
		if err := rows.Scan(&u.ID, &u.Username, &u.FullName, &u.Role,
			&u.IsProtected, &u.IsSuperAdmin, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *UserRepository) ListPaginated(searchTerm, role, sortBy, sortOrder string, page, pageSize int, usernameFilter string) ([]models.GlobalUser, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	baseFrom := `FROM global_users gu JOIN lab_permissions lp ON lp.user_id = gu.id`
	baseWhere := ` WHERE lp.lab_url_path = ?`
	var args []any
	args = append(args, r.labURLPath)

	if searchTerm != "" {
		baseWhere += ` AND (gu.username LIKE ? OR gu.full_name LIKE ?)`
		s := "%" + searchTerm + "%"
		args = append(args, s, s)
	}
	if role != "" {
		baseWhere += ` AND lp.role = ?`
		args = append(args, role)
	}
	if usernameFilter != "" {
		baseWhere += ` AND gu.username = ?`
		args = append(args, usernameFilter)
	}

	var total int
	countQ := `SELECT COUNT(*) ` + baseFrom + baseWhere
	if err := r.globalDB.QueryRow(countQ, args...).Scan(&total); err != nil {
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
	             gu.is_protected, gu.is_super_admin, gu.created_at
	       ` + baseFrom + baseWhere + ` ORDER BY ` + sortCol + ` ` + sortOrder + ` LIMIT ? OFFSET ?`
	queryArgs := append(args, pageSize, offset)

	rows, err := r.globalDB.Query(q, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []models.GlobalUser
	for rows.Next() {
		var u models.GlobalUser
		if err := rows.Scan(&u.ID, &u.Username, &u.FullName, &u.Role,
			&u.IsProtected, &u.IsSuperAdmin, &u.CreatedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, nil
}

func (r *UserRepository) GetByID(id int) (*models.GlobalUser, error) {
	var u models.GlobalUser
	err := r.globalDB.QueryRow(`
		SELECT gu.id, gu.username, gu.full_name, lp.role,
		       gu.is_protected, gu.is_super_admin, gu.created_at, gu.updated_at
		FROM global_users gu
		JOIN lab_permissions lp ON lp.user_id = gu.id
		WHERE gu.id = ? AND lp.lab_url_path = ?
	`, id, r.labURLPath).Scan(&u.ID, &u.Username, &u.FullName, &u.Role,
		&u.IsProtected, &u.IsSuperAdmin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByUsername(username string) (*models.GlobalUser, error) {
	var u models.GlobalUser
	err := r.globalDB.QueryRow(`
		SELECT gu.id, gu.username, gu.full_name, lp.role,
		       gu.is_protected, gu.is_super_admin, gu.created_at, gu.updated_at
		FROM global_users gu
		JOIN lab_permissions lp ON lp.user_id = gu.id
		WHERE gu.username = ? AND lp.lab_url_path = ?
	`, username, r.labURLPath).Scan(&u.ID, &u.Username, &u.FullName, &u.Role,
		&u.IsProtected, &u.IsSuperAdmin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetPasswordHash(id int) (string, error) {
	var hash string
	err := r.globalDB.QueryRow(`SELECT password FROM global_users WHERE id = ?`, id).Scan(&hash)
	return hash, err
}

func (r *UserRepository) ExistsUsername(username string, excludeID int) (bool, error) {
	var count int
	err := r.globalDB.QueryRow(`SELECT COUNT(*) FROM global_users WHERE username = ? AND id != ?`, username, excludeID).Scan(&count)
	return count > 0, err
}

func (r *UserRepository) Create(username, passwordHash, fullName, role string) (sql.Result, error) {
	return r.globalDB.Exec(`INSERT INTO global_users (username, password, full_name, password_is_default) VALUES (?, ?, ?, 0)`,
		username, passwordHash, fullName)
}

func (r *UserRepository) UpdateUser(id int, username, fullName, role string) error {
	_, err := r.globalDB.Exec(`UPDATE global_users SET username = ?, full_name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		username, fullName, id)
	return err
}

func (r *UserRepository) UpdateProfile(id int, username, fullName string) error {
	_, err := r.globalDB.Exec(`UPDATE global_users SET username = ?, full_name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		username, fullName, id)
	return err
}

func (r *UserRepository) UpdatePassword(id int, hash string) error {
	_, err := r.globalDB.Exec(`UPDATE global_users SET password = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, hash, id)
	return err
}

func (r *UserRepository) Delete(id int) error {
	_, err := r.globalDB.Exec(`DELETE FROM lab_permissions WHERE user_id = ? AND lab_url_path = ?`, id, r.labURLPath)
	if err != nil {
		return err
	}
	var remaining int
	r.globalDB.QueryRow(`SELECT COUNT(*) FROM lab_permissions WHERE user_id = ?`, id).Scan(&remaining)
	if remaining == 0 {
		_, err = r.globalDB.Exec(`DELETE FROM global_users WHERE id = ?`, id)
	}
	return err
}
