package repository

import (
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/search"
)

type DeviceUsageRepository struct {
	db     DBTX
	search *search.Builder
}

func NewDeviceUsageRepository(db *database.DB) *DeviceUsageRepository {
	return &DeviceUsageRepository{db: db, search: search.New(db)}
}

func (r *DeviceUsageRepository) WithTx(tx *database.Tx) *DeviceUsageRepository {
	return &DeviceUsageRepository{db: tx, search: r.search}
}

type DeviceUsageFilters struct {
	DeviceID    string
	Search      string
	IsAvailable string
	Category    string
	SortBy      string
	SortOrder   string
}

func (r *DeviceUsageRepository) List(filters DeviceUsageFilters) ([]DeviceUsageRow, error) {
	return r.listWithQuery(filters, "")
}

func (r *DeviceUsageRepository) ListPaginated(filters DeviceUsageFilters, page, pageSize int) ([]DeviceUsageRow, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}

	usageClause, usageArgs := r.buildUsageClause(filters)

	var total int
	r.db.QueryRow(`SELECT COUNT(*) FROM device_usages u
		JOIN devices d ON d.id = u.device_id
		JOIN device_types dt ON dt.id = d.device_type_id
		JOIN categories c ON c.id = dt.category_id WHERE 1=1`+usageClause, usageArgs...).Scan(&total)

	sortBy := "u.usage_date"
	switch filters.SortBy {
	case "user_name":
		sortBy = "u.user_name"
	case "usage_date":
		sortBy = "u.usage_date"
	case "is_available":
		sortBy = "u.is_available"
	case "purpose":
		sortBy = "u.purpose"
	case "category":
		sortBy = "c.name"
	}

	query := `SELECT u.id, u.device_id, d.label, dt.name, c.name,
		c.label_prefix, dt.label_prefix,
		u.user_name, u.user_type, u.usage_date, u.is_available, COALESCE(u.purpose,''), COALESCE(u.notes,'')
		FROM device_usages u
		JOIN devices d ON d.id = u.device_id
		JOIN device_types dt ON dt.id = d.device_type_id
		JOIN categories c ON c.id = dt.category_id WHERE 1=1` + usageClause
	orderDir := "DESC"
	if filters.SortOrder == "ASC" {
		orderDir = "ASC"
	}
	query += ` ORDER BY ` + sortBy + ` ` + orderDir + ` LIMIT ? OFFSET ?`

	allArgs := append(usageArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.Query(query, allArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var usages []DeviceUsageRow
	for rows.Next() {
		var u DeviceUsageRow
		if err := rows.Scan(&u.ID, &u.DeviceID, &u.DeviceLabel, &u.DeviceTypeName, &u.CategoryName,
			&u.CategoryPrefix, &u.DeviceTypePrefix,
			&u.UserName, &u.UserType, &u.UsageDate, &u.IsAvailable, &u.Purpose, &u.Notes); err != nil {
			return nil, 0, err
		}
		usages = append(usages, u)
	}
	return usages, total, nil
}

func (r *DeviceUsageRepository) buildUsageClause(filters DeviceUsageFilters) (string, []any) {
	var clause string
	var args []any
	if filters.DeviceID != "" {
		clause += ` AND u.device_id = ?`
		args = append(args, filters.DeviceID)
	}
	if filters.IsAvailable != "" {
		clause += ` AND u.is_available = ?`
		args = append(args, filters.IsAvailable)
	}
	if filters.Category != "" {
		clause += ` AND c.name = ?`
		args = append(args, filters.Category)
	}
	if filters.Search != "" {
		sClause, sArgs := r.search.Where("device_usage", filters.Search)
		clause += sClause
		args = append(args, sArgs...)
	}
	return clause, args
}

func (r *DeviceUsageRepository) ExportAll() ([]DeviceUsageRow, error) {
	return r.listWithQuery(DeviceUsageFilters{}, "")
}

func (r *DeviceUsageRepository) listWithQuery(filters DeviceUsageFilters, suffix string) ([]DeviceUsageRow, error) {
	usageClause, usageArgs := r.buildUsageClause(filters)
	query := `SELECT u.id, u.device_id, d.label, dt.name, c.name,
		c.label_prefix, dt.label_prefix,
		u.user_name, u.user_type, u.usage_date, u.is_available, COALESCE(u.purpose,''), COALESCE(u.notes,'')
		FROM device_usages u
		JOIN devices d ON d.id = u.device_id
		JOIN device_types dt ON dt.id = d.device_type_id
		JOIN categories c ON c.id = dt.category_id WHERE 1=1` + usageClause

	sortBy := "u.usage_date"
	switch filters.SortBy {
	case "user_name":
		sortBy = "u.user_name"
	case "usage_date":
		sortBy = "u.usage_date"
	case "is_available":
		sortBy = "u.is_available"
	case "purpose":
		sortBy = "u.purpose"
	}
	query += ` ORDER BY ` + sortBy + ` DESC` + suffix

	rows, err := r.db.Query(query, usageArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usages []DeviceUsageRow
	for rows.Next() {
		var u DeviceUsageRow
		if err := rows.Scan(&u.ID, &u.DeviceID, &u.DeviceLabel, &u.DeviceTypeName, &u.CategoryName,
			&u.CategoryPrefix, &u.DeviceTypePrefix,
			&u.UserName, &u.UserType, &u.UsageDate, &u.IsAvailable, &u.Purpose, &u.Notes); err != nil {
			return nil, err
		}
		usages = append(usages, u)
	}
	return usages, nil
}

type DeviceUsageRow struct {
	models.DeviceUsage
	DeviceTypeName   string
	CategoryName     string
	CategoryPrefix   string
	DeviceTypePrefix string
}

func (r *DeviceUsageRepository) GetByID(id int) (*DeviceUsageRow, error) {
	var u DeviceUsageRow
	err := r.db.QueryRow(`SELECT u.id, u.device_id, d.label, dt.name, c.name,
		c.label_prefix, dt.label_prefix,
		u.user_name, u.user_type, u.usage_date, u.is_available, COALESCE(u.purpose,''), COALESCE(u.notes,'')
		FROM device_usages u
		JOIN devices d ON d.id = u.device_id
		JOIN device_types dt ON dt.id = d.device_type_id
		JOIN categories c ON c.id = dt.category_id WHERE u.id = ?`, id).
		Scan(&u.ID, &u.DeviceID, &u.DeviceLabel, &u.DeviceTypeName, &u.CategoryName,
			&u.CategoryPrefix, &u.DeviceTypePrefix,
			&u.UserName, &u.UserType, &u.UsageDate, &u.IsAvailable, &u.Purpose, &u.Notes)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *DeviceUsageRepository) GetConsumableDevices() ([]models.Device, error) {
	rows, err := r.db.Query(`SELECT d.id, d.label, COALESCE(d.serial_number,''), d.condition
		FROM devices d
		JOIN device_types dt ON dt.id = d.device_type_id
		WHERE dt.usage_type = 'consumable'
		AND d.condition = 'normal'
		AND (d.id NOT IN (SELECT device_id FROM device_usages) OR
			(SELECT is_available FROM device_usages WHERE device_id = d.id ORDER BY usage_date DESC, id DESC LIMIT 1) = 'yes')
		ORDER BY d.label`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var d models.Device
		if rows.Scan(&d.ID, &d.Label, &d.SerialNumber, &d.Condition) == nil {
			devices = append(devices, d)
		}
	}
	return devices, nil
}

func (r *DeviceUsageRepository) Create(deviceID int, userName, userType string, usageDate time.Time, isAvailable, purpose string) (int64, error) {
	result, err := r.db.Exec(`INSERT INTO device_usages (device_id, user_name, user_type, usage_date, is_available, purpose)
		VALUES (?, ?, ?, ?, ?, ?)`, deviceID, userName, userType, usageDate, isAvailable, purpose)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *DeviceUsageRepository) Update(id int, userName, userType string, usageDate time.Time, isAvailable, purpose, notes string) error {
	_, err := r.db.Exec(`UPDATE device_usages SET user_name=?, user_type=?, usage_date=?, is_available=?, purpose=?, notes=? WHERE id=?`,
		userName, userType, usageDate, isAvailable, purpose, notes, id)
	return err
}

func (r *DeviceUsageRepository) UpdateAvailability(id int, isAvailable string) error {
	_, err := r.db.Exec("UPDATE device_usages SET is_available = ? WHERE id = ?", isAvailable, id)
	return err
}

func (r *DeviceUsageRepository) Delete(id int) error {
	_, err := r.db.Exec("DELETE FROM device_usages WHERE id = ?", id)
	return err
}
