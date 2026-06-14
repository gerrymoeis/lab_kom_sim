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

var categoryCols = []string{"id", "name", "default_prefix", "created_at"}

func (r *CategoryRepository) GetByID(id int) (*models.Category, error) {
	return getOne[models.Category](r.db, "categories", categoryCols, "id = ?", id)
}

func (r *CategoryRepository) GetByName(name string) (*models.Category, error) {
	return getByField[models.Category](r.db, "categories", categoryCols, "name", name)
}

func (r *CategoryRepository) GetByPrefixSlug(slug string) (*models.Category, error) {
	return getOne[models.Category](r.db, "categories", categoryCols, "LOWER(default_prefix) = LOWER(?)", slug)
}

func (r *CategoryRepository) ListByUsageType(usageType string) ([]models.Category, error) {
	rows, err := r.db.Query(`SELECT DISTINCT c.id, c.name, c.default_prefix, c.created_at
		FROM categories c
		JOIN device_types dt ON dt.category_id = c.id
		WHERE dt.usage_type = ?
		ORDER BY c.name`, usageType)
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

func (r *CategoryRepository) Create(name, prefix string) (sql.Result, error) {
	return r.db.Exec("INSERT INTO categories (name, default_prefix) VALUES (?, ?)", name, prefix)
}

func (r *CategoryRepository) Update(id int, name, prefix string) error {
	_, err := r.db.Exec("UPDATE categories SET name=?, default_prefix=? WHERE id=?", name, prefix, id)
	return err
}

func (r *CategoryRepository) Delete(id int) error {
	if _, err := r.db.Exec("DELETE FROM devices WHERE device_type_id IN (SELECT id FROM device_types WHERE category_id = ?)", id); err != nil {
		return err
	}
	if _, err := r.db.Exec("DELETE FROM device_types WHERE category_id = ?", id); err != nil {
		return err
	}
	_, err := r.db.Exec("DELETE FROM categories WHERE id = ?", id)
	return err
}
