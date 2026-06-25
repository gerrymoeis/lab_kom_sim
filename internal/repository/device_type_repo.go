package repository

import (
	"database/sql"
	"fmt"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/search"
)

type DeviceTypeRepository struct {
	db     DBTX
	search *search.Builder
}

func NewDeviceTypeRepository(db *database.DB) *DeviceTypeRepository {
	return &DeviceTypeRepository{db: db, search: search.New(db)}
}

func (r *DeviceTypeRepository) List(category, search string) ([]models.DeviceType, error) {
	return r.listWithQuery(category, search, "", "", 0, 0)
}

func (r *DeviceTypeRepository) ListPaginated(category, search, sortBy string, page, pageSize int) ([]models.DeviceType, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM device_types dt JOIN categories c ON c.id = dt.category_id WHERE 1=1`
	var args []any
	if category != "" {
		countQuery += ` AND c.name = ?`
		args = append(args, category)
	}
	if search != "" {
		sClause, sArgs := r.search.Where("device_type", search)
		countQuery += sClause
		args = append(args, sArgs...)
	}
	r.db.QueryRow(countQuery, args...).Scan(&total)

	dts, err := r.listWithQuery(category, search, sortBy, ` LIMIT ? OFFSET ?`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	return dts, total, nil
}

func (r *DeviceTypeRepository) listWithQuery(category, search, sortBy string, suffix string, limit, offset int) ([]models.DeviceType, error) {
	query := `SELECT dt.id, dt.category_id, c.name, dt.name, dt.brand, dt.model, dt.label_prefix,
		dt.usage_type, dt.default_location, COALESCE(dt.photo,''), dt.created_at, dt.updated_at
		FROM device_types dt JOIN categories c ON c.id = dt.category_id WHERE 1=1`
	var args []any
	if category != "" {
		query += ` AND c.name = ?`
		args = append(args, category)
	}
	if search != "" {
		sClause, sArgs := r.search.Where("device_type", search)
		query += sClause
		args = append(args, sArgs...)
	}
	switch sortBy {
	case "name":
		query += ` ORDER BY dt.name`
	default:
		query += ` ORDER BY c.name, dt.name`
	}
	query += suffix
	if suffix != "" {
		args = append(args, limit, offset)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dts []models.DeviceType
	for rows.Next() {
		var dt models.DeviceType
		var brand, model, loc, photo sql.NullString
		if err := rows.Scan(&dt.ID, &dt.CategoryID, &dt.CategoryName, &dt.Name,
			&brand, &model, &dt.LabelPrefix, &dt.UsageType, &loc, &photo, &dt.CreatedAt, &dt.UpdatedAt); err != nil {
			return nil, err
		}
		dt.Brand = valStr(brand)
		dt.Model = valStr(model)
		dt.DefaultLocation = valStr(loc)
		dt.Photo = valStr(photo)
		dts = append(dts, dt)
	}
	return dts, nil
}

func (r *DeviceTypeRepository) GetAllSimple() ([]models.DeviceType, error) {
	rows, err := r.db.Query(`SELECT dt.id, dt.category_id, c.name, dt.name,
		dt.label_prefix, dt.usage_type, dt.default_location, COALESCE(dt.photo,'')
		FROM device_types dt JOIN categories c ON c.id = dt.category_id ORDER BY c.name, dt.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dts []models.DeviceType
	for rows.Next() {
		var dt models.DeviceType
		var loc, photo sql.NullString
		if rows.Scan(&dt.ID, &dt.CategoryID, &dt.CategoryName, &dt.Name,
			&dt.LabelPrefix, &dt.UsageType, &loc, &photo) != nil {
			continue
		}
		dt.DefaultLocation = valStr(loc)
		dt.Photo = valStr(photo)
		dts = append(dts, dt)
	}
	return dts, nil
}

func (r *DeviceTypeRepository) GetBySlug(slug string) (*models.DeviceType, error) {
	return r.getByField("slug", slug)
}

func (r *DeviceTypeRepository) GetByLabelSlug(slug string) (*models.DeviceType, error) {
	return scanDeviceTypeRowWithPrefix(r.db, "LOWER(dt.label_prefix) = LOWER(?)", slug)
}

func (r *DeviceTypeRepository) getByField(field, value string) (*models.DeviceType, error) {
	return scanDeviceTypeRowWithPrefix(r.db, fmt.Sprintf("dt.%s = ?", field), value)
}

func (r *DeviceTypeRepository) GetByID(id int) (*models.DeviceType, error) {
	return scanDeviceTypeRowNoPrefix(r.db, "dt.id = ?", id)
}

func (r *DeviceTypeRepository) GetByIDSimple(id int) (*models.DeviceType, error) {
	var dt models.DeviceType
	var loc, photo sql.NullString
		err := r.db.QueryRow(`SELECT id, category_id, name, label_prefix, usage_type,
		COALESCE(default_location,''), COALESCE(photo,'') FROM device_types WHERE id = ?`, id).
		Scan(&dt.ID, &dt.CategoryID, &dt.Name, &dt.LabelPrefix, &dt.UsageType, &loc, &photo)
	if err != nil {
		return nil, err
	}
	dt.DefaultLocation = valStr(loc)
	dt.Photo = valStr(photo)
	return &dt, nil
}

func (r *DeviceTypeRepository) GetPrefix(id int) (string, error) {
	var prefix string
	err := r.db.QueryRow("SELECT label_prefix FROM device_types WHERE id = ?", id).Scan(&prefix)
	return prefix, err
}

func (r *DeviceTypeRepository) GetName(id int) (string, error) {
	var name string
	err := r.db.QueryRow("SELECT name FROM device_types WHERE id = ?", id).Scan(&name)
	return name, err
}

func (r *DeviceTypeRepository) Create(categoryID int, name, brand, model, prefix, usageType, location, photo string) (sql.Result, error) {
	return r.db.Exec(`INSERT INTO device_types (category_id, name, brand, model, label_prefix, usage_type, default_location, photo)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, categoryID, name, brand, model, prefix, usageType, location, photo)
}

func (r *DeviceTypeRepository) Update(id, categoryID int, name, brand, model, prefix, usageType, location, photo string) error {
	_, err := r.db.Exec(`UPDATE device_types SET category_id=?, name=?, brand=?, model=?,
		label_prefix=?, usage_type=?, default_location=?, photo=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		categoryID, name, brand, model, prefix, usageType, location, photo, id)
	return err
}

func (r *DeviceTypeRepository) Delete(id int) error {
	if _, err := r.db.Exec("DELETE FROM devices WHERE device_type_id = ?", id); err != nil {
		return err
	}
	_, err := r.db.Exec("DELETE FROM device_types WHERE id = ?", id)
	return err
}

func (r *DeviceTypeRepository) CountByCategoryID(categoryID int) (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM device_types WHERE category_id = ?", categoryID).Scan(&count)
	return count, err
}

func (r *DeviceTypeRepository) GetByCategoryID(categoryID int) ([]models.DeviceType, error) {
	rows, err := r.db.Query(`SELECT id, category_id, name, brand, model, label_prefix, usage_type,
		COALESCE(default_location,''), COALESCE(photo,'') FROM device_types WHERE category_id = ? ORDER BY name`, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dts []models.DeviceType
	for rows.Next() {
		var dt models.DeviceType
		var brand, model, loc, photo sql.NullString
		if rows.Scan(&dt.ID, &dt.CategoryID, &dt.Name, &brand, &model,
			&dt.LabelPrefix, &dt.UsageType, &loc, &photo) != nil {
			continue
		}
		dt.Brand = valStr(brand)
		dt.Model = valStr(model)
		dt.DefaultLocation = valStr(loc)
		dt.Photo = valStr(photo)
		dts = append(dts, dt)
	}
	return dts, nil
}
