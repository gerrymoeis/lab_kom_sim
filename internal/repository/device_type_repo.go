package repository

import (
	"database/sql"

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
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 20 }

	var total int
	countQuery := `SELECT COUNT(*) FROM device_types WHERE 1=1`
	var args []any
	if category != "" {
		countQuery += ` AND category = ?`
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
	query := `SELECT id, name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template, created_at FROM device_types WHERE 1=1`
	var args []any
	if category != "" {
		query += ` AND category = ?`
		args = append(args, category)
	}
	if search != "" {
		sClause, sArgs := r.search.Where("device_type", search)
		query += sClause
		args = append(args, sArgs...)
	}
	switch sortBy {
	case "name":
		query += ` ORDER BY name`
	default:
		query += ` ORDER BY category, name`
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
		var brand, model, prefix, loc, notes sql.NullString
		if err := rows.Scan(&dt.ID, &dt.Name, &dt.Category, &brand, &model, &dt.ItemType,
			&dt.IsLoanable, &dt.IsConsumable, &prefix, &loc, &notes, &dt.CreatedAt); err != nil {
			return nil, err
		}
		dt.Brand = valStr(brand)
		dt.Model = valStr(model)
		dt.AssetCodePrefix = valStr(prefix)
		dt.DefaultLocation = valStr(loc)
		dt.NotesTemplate = valStr(notes)
		dts = append(dts, dt)
	}
	return dts, nil
}

func (r *DeviceTypeRepository) GetAllSimple() ([]models.DeviceType, error) {
	rows, err := r.db.Query(`SELECT id, name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location FROM device_types ORDER BY category, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dts []models.DeviceType
	for rows.Next() {
		var dt models.DeviceType
		var brand, model, prefix, loc sql.NullString
		if rows.Scan(&dt.ID, &dt.Name, &dt.Category, &brand, &model, &dt.ItemType, &dt.IsLoanable, &dt.IsConsumable, &prefix, &loc) != nil {
			continue
		}
		dt.Brand = valStr(brand)
		dt.Model = valStr(model)
		dt.AssetCodePrefix = valStr(prefix)
		dt.DefaultLocation = valStr(loc)
		dts = append(dts, dt)
	}
	return dts, nil
}

func (r *DeviceTypeRepository) GetByID(id int) (*models.DeviceType, error) {
	var dt models.DeviceType
	var brand, model, prefix, loc, notes sql.NullString
	err := r.db.QueryRow(`SELECT id, name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template, created_at FROM device_types WHERE id = ?`, id).
		Scan(&dt.ID, &dt.Name, &dt.Category, &brand, &model, &dt.ItemType, &dt.IsLoanable, &dt.IsConsumable, &prefix, &loc, &notes, &dt.CreatedAt)
	if err != nil {
		return nil, err
	}
	dt.Brand = valStr(brand)
	dt.Model = valStr(model)
	dt.AssetCodePrefix = valStr(prefix)
	dt.DefaultLocation = valStr(loc)
	dt.NotesTemplate = valStr(notes)
	return &dt, nil
}

func (r *DeviceTypeRepository) GetByIDSimple(id int) (*models.DeviceType, error) {
	var dt models.DeviceType
	var brand, model, prefix, loc, notes sql.NullString
	err := r.db.QueryRow(`SELECT id, name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template FROM device_types WHERE id = ?`, id).
		Scan(&dt.ID, &dt.Name, &dt.Category, &brand, &model, &dt.ItemType, &dt.IsLoanable, &dt.IsConsumable, &prefix, &loc, &notes)
	if err != nil {
		return nil, err
	}
	dt.Brand = valStr(brand)
	dt.Model = valStr(model)
	dt.AssetCodePrefix = valStr(prefix)
	dt.DefaultLocation = valStr(loc)
	dt.NotesTemplate = valStr(notes)
	return &dt, nil
}

func (r *DeviceTypeRepository) GetPrefix(id int) (string, error) {
	var prefix string
	err := r.db.QueryRow(`SELECT asset_code_prefix FROM device_types WHERE id = ?`, id).Scan(&prefix)
	return prefix, err
}

func (r *DeviceTypeRepository) GetName(id int) (string, error) {
	var name string
	err := r.db.QueryRow(`SELECT name FROM device_types WHERE id = ?`, id).Scan(&name)
	return name, err
}

func (r *DeviceTypeRepository) Create(name, category, brand, model, itemType string, isLoanable, isConsumable bool, prefix, location, notesTmpl string) (sql.Result, error) {
	return r.db.Exec(`INSERT INTO device_types (name, category, brand, model, item_type, is_loanable, is_consumable, asset_code_prefix, default_location, notes_template) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		name, category, brand, model, itemType, isLoanable, isConsumable, prefix, location, notesTmpl)
}

func (r *DeviceTypeRepository) Update(id int, name, category, brand, model, itemType string, isLoanable, isConsumable bool, prefix, location, notesTmpl string) error {
	_, err := r.db.Exec(`UPDATE device_types SET name=?, category=?, brand=?, model=?, item_type=?, is_loanable=?, is_consumable=?, asset_code_prefix=?, default_location=?, notes_template=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		name, category, brand, model, itemType, isLoanable, isConsumable, prefix, location, notesTmpl, id)
	return err
}

func (r *DeviceTypeRepository) Delete(id int) error {
	_, err := r.db.Exec(`DELETE FROM device_types WHERE id = ?`, id)
	return err
}

func valStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
