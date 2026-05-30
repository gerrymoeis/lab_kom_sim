package repository

import (
	"database/sql"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
)

type CategoryRepository struct {
	db DBTX
}

func NewCategoryRepository(db *database.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) List() ([]models.Category, error) {
	rows, err := r.db.Query("SELECT id, name, default_prefix, created_at FROM categories ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cats []models.Category
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.DefaultPrefix, &c.CreatedAt); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, nil
}

func (r *CategoryRepository) GetByID(id int) (*models.Category, error) {
	var c models.Category
	err := r.db.QueryRow("SELECT id, name, default_prefix, created_at FROM categories WHERE id = ?", id).
		Scan(&c.ID, &c.Name, &c.DefaultPrefix, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CategoryRepository) GetByName(name string) (*models.Category, error) {
	var c models.Category
	err := r.db.QueryRow("SELECT id, name, default_prefix, created_at FROM categories WHERE name = ?", name).
		Scan(&c.ID, &c.Name, &c.DefaultPrefix, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CategoryRepository) Create(name, prefix string) (sql.Result, error) {
	return r.db.Exec("INSERT INTO categories (name, default_prefix) VALUES (?, ?)", name, prefix)
}

func (r *CategoryRepository) Update(id int, name, prefix string) error {
	_, err := r.db.Exec("UPDATE categories SET name=?, default_prefix=? WHERE id=?", name, prefix, id)
	return err
}

func (r *CategoryRepository) Delete(id int) error {
	_, err := r.db.Exec("DELETE FROM categories WHERE id = ?", id)
	return err
}
