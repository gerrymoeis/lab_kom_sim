package repository

import (
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
)

type DeviceUsageRepository struct {
	db DBTX
}

func NewDeviceUsageRepository(db *database.DB) *DeviceUsageRepository {
	return &DeviceUsageRepository{db: db}
}

func (r *DeviceUsageRepository) WithTx(tx *database.Tx) *DeviceUsageRepository {
	return &DeviceUsageRepository{db: tx}
}

type DeviceUsageFilters struct {
	DeviceID string
	Search   string
	SortBy   string
}

func (r *DeviceUsageRepository) List(filters DeviceUsageFilters) ([]DeviceUsageRow, error) {
	return r.listWithQuery(filters, "")
}

func (r *DeviceUsageRepository) ListPaginated(filters DeviceUsageFilters, page, pageSize int) ([]DeviceUsageRow, int, error) {
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 20 }

	usageClause, usageArgs := r.buildUsageClause(filters)

	var total int
	r.db.QueryRow(`SELECT COUNT(*) FROM device_usages u JOIN devices d ON u.device_id = d.id WHERE 1=1`+usageClause, usageArgs...).Scan(&total)

	sortBy := "u.usage_date"
	if filters.SortBy == "user_name" {
		sortBy = "u.user_name"
	}

	query := `SELECT u.id, u.device_id, d.name, d.asset_code, u.user_name, u.user_type, u.usage_date,
		u.quantity, u.is_available, u.purpose FROM device_usages u JOIN devices d ON u.device_id = d.id WHERE 1=1` + usageClause
	query += ` ORDER BY ` + sortBy + ` DESC LIMIT ? OFFSET ?`

	allArgs := append(usageArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.Query(query, allArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var usages []DeviceUsageRow
	for rows.Next() {
		var u DeviceUsageRow
		if err := rows.Scan(&u.ID, &u.DeviceID, &u.DeviceName, &u.DeviceAssetCode,
			&u.UserName, &u.UserType, &u.UsageDate, &u.Quantity, &u.IsAvailable, &u.Purpose); err != nil {
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
	if filters.Search != "" {
		clause += ` AND (u.user_name LIKE ? OR d.name LIKE ?)`
		s := "%" + filters.Search + "%"
		args = append(args, s, s)
	}
	return clause, args
}

func (r *DeviceUsageRepository) listWithQuery(filters DeviceUsageFilters, suffix string) ([]DeviceUsageRow, error) {
	usageClause, usageArgs := r.buildUsageClause(filters)
	query := `SELECT u.id, u.device_id, d.name, d.asset_code, u.user_name, u.user_type, u.usage_date,
		u.quantity, u.is_available, u.purpose FROM device_usages u JOIN devices d ON u.device_id = d.id WHERE 1=1` + usageClause

	sortBy := "u.usage_date"
	if filters.SortBy == "user_name" {
		sortBy = "u.user_name"
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
		if err := rows.Scan(&u.ID, &u.DeviceID, &u.DeviceName, &u.DeviceAssetCode,
			&u.UserName, &u.UserType, &u.UsageDate, &u.Quantity, &u.IsAvailable, &u.Purpose); err != nil {
			return nil, err
		}
		usages = append(usages, u)
	}
	return usages, nil
}

type DeviceUsageRow struct {
	models.DeviceUsage
	DeviceName      string
	DeviceAssetCode string
}

func (r *DeviceUsageRepository) GetByID(id int) (*DeviceUsageRow, error) {
	var u DeviceUsageRow
	err := r.db.QueryRow(`SELECT u.id, u.device_id, d.name, d.asset_code, u.user_name, u.user_type,
		u.usage_date, u.quantity, u.is_available, u.purpose, COALESCE(u.notes,'')
		FROM device_usages u JOIN devices d ON u.device_id = d.id WHERE u.id = ?`, id).
		Scan(&u.ID, &u.DeviceID, &u.DeviceName, &u.DeviceAssetCode,
			&u.UserName, &u.UserType, &u.UsageDate, &u.Quantity, &u.IsAvailable, &u.Purpose, &u.Notes)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *DeviceUsageRepository) GetConsumableDevices() ([]models.Device, error) {
	rows, err := r.db.Query(`SELECT id, asset_code, name, item_type, quantity_available, is_consumable FROM devices WHERE is_consumable = TRUE AND quantity_available > 0 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var d models.Device
		if rows.Scan(&d.ID, &d.AssetCode, &d.Name, &d.ItemType, &d.QuantityAvailable, &d.IsConsumable) == nil {
			devices = append(devices, d)
		}
	}
	return devices, nil
}

func (r *DeviceUsageRepository) Create(deviceID int, userName, userType string, usageDate time.Time, quantity int, isAvailable, purpose string) (int64, error) {
	result, err := r.db.Exec(`INSERT INTO device_usages (device_id, user_name, user_type, usage_date, quantity, is_available, purpose) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		deviceID, userName, userType, usageDate, quantity, isAvailable, purpose)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *DeviceUsageRepository) Update(id int, userName, userType string, usageDate time.Time, quantity int, isAvailable, purpose, notes string) error {
	_, err := r.db.Exec(`UPDATE device_usages SET user_name=?, user_type=?, usage_date=?, quantity=?, is_available=?, purpose=?, notes=? WHERE id=?`,
		userName, userType, usageDate, quantity, isAvailable, purpose, notes, id)
	return err
}

func (r *DeviceUsageRepository) UpdateAvailability(id int, isAvailable string) error {
	_, err := r.db.Exec(`UPDATE device_usages SET is_available = ? WHERE id = ?`, isAvailable, id)
	return err
}

func (r *DeviceUsageRepository) GetDeviceAndQuantity(id int) (deviceID, oldQty int, oldAvail string, err error) {
	err = r.db.QueryRow(`SELECT device_id, quantity, is_available FROM device_usages WHERE id = ?`, id).Scan(&deviceID, &oldQty, &oldAvail)
	return
}

func (r *DeviceUsageRepository) Delete(id int) error {
	_, err := r.db.Exec(`DELETE FROM device_usages WHERE id = ?`, id)
	return err
}
