package repository

import (
	"database/sql"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
)

type UserRepository struct {
	db DBTX
}

func NewUserRepository(db *database.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) List() ([]models.User, error) {
	rows, err := r.db.Query(`SELECT id, username, full_name, role, created_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.FullName, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *UserRepository) ListPaginated(page, pageSize int) ([]models.User, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	rows, err := r.db.Query(`SELECT id, username, full_name, role, created_at FROM users ORDER BY created_at DESC LIMIT ? OFFSET ?`, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.FullName, &u.Role, &u.CreatedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, nil
}

func (r *UserRepository) GetByID(id int) (*models.User, error) {
	var u models.User
	err := r.db.QueryRow(`SELECT id, username, full_name, role, created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &u.FullName, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByUsername(username string) (*models.User, error) {
	var u models.User
	err := r.db.QueryRow(`SELECT id, password, full_name, role FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Password, &u.FullName, &u.Role)
	if err != nil {
		return nil, err
	}
	return &u, nil
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
