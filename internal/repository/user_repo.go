package repository

import (
	"database/sql"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/search"
)

type UserRepository struct {
	db     DBTX
	search *search.Builder
}

func NewUserRepository(db *database.DB) *UserRepository {
	return &UserRepository{db: db, search: search.New(db)}
}

func (r *UserRepository) List() ([]models.User, error) {
	rows, err := r.db.Query(`SELECT id, username, full_name, role, is_protected, is_super_admin, created_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.FullName, &u.Role, &u.IsProtected, &u.IsSuperAdmin, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *UserRepository) ListPaginated(search, role, sortBy, sortOrder string, page, pageSize int, usernameFilter string) ([]models.User, int, error) {
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 20 }

	buildCount := func() (string, []any) {
		q := `SELECT COUNT(*) FROM users WHERE 1=1`
		var a []any
		if search != "" {
			sClause, sArgs := r.search.Where("user", search)
			q += sClause; a = append(a, sArgs...)
		}
		if role != "" {
			q += ` AND role = ?`; a = append(a, role)
		}
		if usernameFilter != "" {
			q += ` AND username = ?`; a = append(a, usernameFilter)
		}
		return q, a
	}

	countQ, countArgs := buildCount()
	var total int
	if err := r.db.QueryRow(countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	q := `SELECT id, username, full_name, role, is_protected, is_super_admin, created_at FROM users WHERE 1=1`
	var qa []any
	if search != "" {
		sClause, sArgs := r.search.Where("user", search)
		q += sClause; qa = append(qa, sArgs...)
	}
	if role != "" {
		q += ` AND role = ?`; qa = append(qa, role)
	}
	if usernameFilter != "" {
		q += ` AND username = ?`; qa = append(qa, usernameFilter)
	}

	validSort := map[string]bool{"username": true, "full_name": true, "role": true, "created_at": true}
	if !validSort[sortBy] { sortBy = "created_at" }
	if sortOrder != "ASC" { sortOrder = "DESC" }
	q += ` ORDER BY ` + sortBy + ` ` + sortOrder + ` LIMIT ? OFFSET ?`
	qa = append(qa, pageSize, offset)

	rows, err := r.db.Query(q, qa...)
	if err != nil { return nil, 0, err }
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.FullName, &u.Role, &u.IsProtected, &u.IsSuperAdmin, &u.CreatedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, nil
}

func (r *UserRepository) GetByID(id int) (*models.User, error) {
	var u models.User
	err := r.db.QueryRow(`SELECT id, username, full_name, role, is_protected, is_super_admin, created_at, updated_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &u.FullName, &u.Role, &u.IsProtected, &u.IsSuperAdmin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

var userFullCols = []string{"id", "username", "password", "full_name", "role", "is_protected", "is_super_admin", "created_at", "updated_at"}

func (r *UserRepository) GetByUsername(username string) (*models.User, error) {
	return getByField[models.User](r.db, "users", userFullCols, "username", username)
}

func (r *UserRepository) GetPasswordHash(id int) (string, error) {
	var hash string
	err := r.db.QueryRow(`SELECT password FROM users WHERE id = ?`, id).Scan(&hash)
	return hash, err
}

func (r *UserRepository) GetSessionToken(id int) (string, error) {
	var token string
	err := r.db.QueryRow(`SELECT session_token FROM users WHERE id = ?`, id).Scan(&token)
	return token, err
}

func (r *UserRepository) ExistsUsername(username string, excludeID int) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ? AND id != ?`, username, excludeID).Scan(&count)
	return count > 0, err
}

func (r *UserRepository) Create(username, passwordHash, fullName, role string) (sql.Result, error) {
	return r.db.Exec(`INSERT INTO users (username, password, full_name, role) VALUES (?, ?, ?, ?)`,
		username, passwordHash, fullName, role)
}

func (r *UserRepository) UpdateUser(id int, username, fullName, role string) error {
	_, err := r.db.Exec(`UPDATE users SET username = ?, full_name = ?, role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		username, fullName, role, id)
	return err
}

func (r *UserRepository) UpdateProfile(id int, username, fullName string) error {
	_, err := r.db.Exec(`UPDATE users SET username = ?, full_name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		username, fullName, id)
	return err
}

func (r *UserRepository) UpdatePassword(id int, hash string) error {
	_, err := r.db.Exec(`UPDATE users SET password = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, hash, id)
	return err
}

func (r *UserRepository) UpdateSessionToken(id int, token string) error {
	_, err := r.db.Exec(`UPDATE users SET session_token = ? WHERE id = ?`, token, id)
	return err
}

func (r *UserRepository) ClearSessionToken(id int) error {
	_, err := r.db.Exec(`UPDATE users SET session_token = NULL WHERE id = ?`, id)
	return err
}

func (r *UserRepository) Delete(id int) error {
	_, err := r.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}
