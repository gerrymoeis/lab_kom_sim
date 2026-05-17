package repository

import (
	"database/sql"
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
)

type DeviceUsageRepository struct {
	db *database.DB
}

func NewDeviceUsageRepository(db *database.DB) *DeviceUsageRepository {
	return &DeviceUsageRepository{db: db}
}

type DeviceUsageFilters struct {
	DeviceID string
	Search   string
	SortBy   string
}

func (r *DeviceUsageRepository) List(filters DeviceUsageFilters) ([]DeviceUsageRow, error) {
	query := `SELECT u.id, u.device_id, d.name, d.asset_code, u.user_name, u.user_type, u.usage_date,
		u.quantity, u.is_available, u.purpose FROM device_usages u JOIN devices d ON u.device_id = d.id WHERE 1=1`
	var args []interface{}

	if filters.DeviceID != "" {
		query += ` AND u.device_id = ?`
		args = append(args, filters.DeviceID)
	}
	if filters.Search != "" {
		query += ` AND (u.user_name LIKE ? OR d.name LIKE ?)`
		s := "%" + filters.Search + "%"
		args = append(args, s, s)
	}

	sortBy := "u.usage_date"
	if filters.SortBy == "user_name" {
		sortBy = "u.user_name"
	}
	query += ` ORDER BY ` + sortBy + ` DESC LIMIT 100`

	rows, err := r.db.Query(query, args...)
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
		u.usage_date, u.quantity, u.is_available, u.purpose, u.notes
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

func (r *DeviceUsageRepository) GetDeviceAndQuantity(id int) (deviceID, oldQty int, oldAvail string, err error) {
	err = r.db.QueryRow(`SELECT device_id, quantity, is_available FROM device_usages WHERE id = ?`, id).Scan(&deviceID, &oldQty, &oldAvail)
	return
}

func (r *DeviceUsageRepository) Create(deviceID int, userName, userType string, usageDate time.Time, quantity int, isAvailable, purpose string) (int64, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if isAvailable == "no" {
		res, err := tx.Exec(`UPDATE devices SET quantity_available = quantity_available - ? WHERE id = ? AND quantity_available >= ?`, quantity, deviceID, quantity)
		if err != nil {
			return 0, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return 0, sql.ErrNoRows
		}
	}

	result, err := tx.Exec(`INSERT INTO device_usages (device_id, user_name, user_type, usage_date, quantity, is_available, purpose) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		deviceID, userName, userType, usageDate, quantity, isAvailable, purpose)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *DeviceUsageRepository) Update(id int, userName, userType string, usageDate time.Time, quantity int, isAvailable, purpose, notes string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	devID, oldQty, oldAvail, err := r.GetDeviceAndQuantityTx(tx, id)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`UPDATE device_usages SET user_name=?, user_type=?, usage_date=?, quantity=?, is_available=?, purpose=?, notes=? WHERE id=?`,
		userName, userType, usageDate, quantity, isAvailable, purpose, notes, id)
	if err != nil {
		return err
	}

	if oldAvail != isAvailable {
		if isAvailable == "yes" {
			_, err = tx.Exec(`UPDATE devices SET quantity_available = quantity_available + ? WHERE id = ?`, quantity, devID)
		} else {
			_, err = tx.Exec(`UPDATE devices SET quantity_available = quantity_available - ? WHERE id = ?`, quantity, devID)
		}
	} else if quantity != oldQty {
		_, err = tx.Exec(`UPDATE devices SET quantity_available = quantity_available + ? WHERE id = ?`, oldQty-quantity, devID)
	}
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *DeviceUsageRepository) UpdateAvailability(id int, isAvailable string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	devID, quantity, oldAvail, err := r.GetDeviceAndQuantityTx(tx, id)
	if err != nil {
		return err
	}
	if oldAvail == isAvailable {
		return nil
	}

	_, err = tx.Exec(`UPDATE device_usages SET is_available = ? WHERE id = ?`, isAvailable, id)
	if err != nil {
		return err
	}

	if isAvailable == "yes" {
		_, err = tx.Exec(`UPDATE devices SET quantity_available = quantity_available + ? WHERE id = ?`, quantity, devID)
	} else {
		_, err = tx.Exec(`UPDATE devices SET quantity_available = quantity_available - ? WHERE id = ?`, quantity, devID)
	}

	return tx.Commit()
}

func (r *DeviceUsageRepository) GetDeviceAndQuantityTx(tx *database.Tx, id int) (deviceID, oldQty int, oldAvail string, err error) {
	err = tx.QueryRow(`SELECT device_id, quantity, is_available FROM device_usages WHERE id = ?`, id).Scan(&deviceID, &oldQty, &oldAvail)
	return
}

func (r *DeviceUsageRepository) Delete(id int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	devID, qty, avail, err := r.GetDeviceAndQuantityTx(tx, id)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM device_usages WHERE id = ?`, id); err != nil {
		return err
	}

	if avail == "no" {
		_, err = tx.Exec(`UPDATE devices SET quantity_available = quantity_available + ? WHERE id = ?`, qty, devID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
