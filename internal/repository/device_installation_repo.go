package repository

import (
	"database/sql"
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/search"
)

type DeviceInstallationRepository struct {
	db     DBTX
	search *search.Builder
}

func NewDeviceInstallationRepository(db *database.DB) *DeviceInstallationRepository {
	return &DeviceInstallationRepository{db: db, search: search.New(db)}
}

type InstallationFilters struct {
	Search    string
	Status    string
	Category  string
	SortBy    string
	SortOrder string
}

func (r *DeviceInstallationRepository) ListPaginated(filters InstallationFilters, page, pageSize int) ([]InstallationRow, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}

	where := ""
	var args []any
	if filters.Search != "" {
		sql, sArgs := r.search.Where("device_installation", filters.Search)
		where += sql
		args = append(args, sArgs...)
	}
	if filters.Status != "" {
		switch filters.Status {
		case "selesai":
			where += ` AND di.installation_finish_date IS NOT NULL`
		case "berlangsung":
			where += ` AND di.installation_start_date IS NOT NULL AND di.installation_finish_date IS NULL`
		case "belum":
			where += ` AND di.installation_start_date IS NULL`
		}
	}
	if filters.Category != "" {
		where += ` AND c.name = ?`
		args = append(args, filters.Category)
	}

	var total int
	r.db.QueryRow(`SELECT COUNT(*) FROM device_installations di
		JOIN devices d ON d.id = di.device_id
		JOIN device_types dt ON dt.id = d.device_type_id
		JOIN categories c ON c.id = dt.category_id WHERE 1=1`+where, args...).Scan(&total)

	orderBy := "di.installation_start_date"
	switch filters.SortBy {
	case "location":
		orderBy = "di.location_installed"
	case "asset_code":
		orderBy = "d.asset_code"
	case "start_date":
		orderBy = "di.installation_start_date"
	case "category":
		orderBy = "c.name"
	case "status":
		orderBy = "CASE WHEN di.installation_finish_date IS NOT NULL THEN 2 WHEN di.installation_start_date IS NOT NULL THEN 1 ELSE 0 END"
	}

	orderPrefix := ""
	if orderBy == "di.installation_start_date" {
		orderPrefix = "CASE WHEN di.installation_start_date IS NULL THEN 0 ELSE 1 END, "
	}

	orderDir := "DESC"
	if filters.SortOrder == "ASC" {
		orderDir = "ASC"
	}

	query := `SELECT di.id, di.device_id, d.asset_code, dt.name, c.name,
		c.default_prefix, dt.asset_code_prefix,
		di.location_installed, di.installation_start_date, di.installation_finish_date,
		COALESCE(di.photo,''), COALESCE(di.notes,''), di.created_at, di.updated_at
		FROM device_installations di
		JOIN devices d ON d.id = di.device_id
		JOIN device_types dt ON dt.id = d.device_type_id
		JOIN categories c ON c.id = dt.category_id WHERE 1=1` + where +
		` ORDER BY ` + orderPrefix + orderBy + ` ` + orderDir + ` LIMIT ? OFFSET ?`

	allArgs := append(args, pageSize, (page-1)*pageSize)
	rows, err := r.db.Query(query, allArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var installations []InstallationRow
	for rows.Next() {
		var ir InstallationRow
		if err := rows.Scan(&ir.ID, &ir.DeviceID, &ir.DeviceAssetCode, &ir.DeviceTypeName, &ir.CategoryName,
			&ir.CategoryPrefix, &ir.DeviceTypePrefix,
			&ir.LocationInstalled, &ir.InstallationStartDate, &ir.InstallationFinishDate,
			&ir.Photo, &ir.Notes, &ir.CreatedAt, &ir.UpdatedAt); err != nil {
			return nil, 0, err
		}
		installations = append(installations, ir)
	}
	return installations, total, nil
}

func (r *DeviceInstallationRepository) ExportAll() ([]InstallationRow, error) {
	rows, err := r.db.Query(`SELECT di.id, di.device_id, d.asset_code, dt.name, c.name,
		c.default_prefix, dt.asset_code_prefix,
		di.location_installed, di.installation_start_date, di.installation_finish_date,
		COALESCE(di.photo,''), COALESCE(di.notes,''), di.created_at, di.updated_at
		FROM device_installations di
		JOIN devices d ON d.id = di.device_id
		JOIN device_types dt ON dt.id = d.device_type_id
		JOIN categories c ON c.id = dt.category_id
		ORDER BY CASE WHEN di.installation_start_date IS NULL THEN 0 ELSE 1 END, di.installation_start_date DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var installations []InstallationRow
	for rows.Next() {
		var ir InstallationRow
		if err := rows.Scan(&ir.ID, &ir.DeviceID, &ir.DeviceAssetCode, &ir.DeviceTypeName, &ir.CategoryName,
			&ir.CategoryPrefix, &ir.DeviceTypePrefix,
			&ir.LocationInstalled, &ir.InstallationStartDate, &ir.InstallationFinishDate,
			&ir.Photo, &ir.Notes, &ir.CreatedAt, &ir.UpdatedAt); err != nil {
			return nil, err
		}
		installations = append(installations, ir)
	}
	return installations, nil
}

func (r *DeviceInstallationRepository) GetByID(id int) (*InstallationRow, error) {
	var ir InstallationRow
	err := r.db.QueryRow(`SELECT di.id, di.device_id, d.asset_code, dt.name, c.name,
		c.default_prefix, dt.asset_code_prefix,
		di.location_installed, di.installation_start_date, di.installation_finish_date,
		COALESCE(di.photo,''), COALESCE(di.notes,''), di.created_at, di.updated_at
		FROM device_installations di
		JOIN devices d ON d.id = di.device_id
		JOIN device_types dt ON dt.id = d.device_type_id
		JOIN categories c ON c.id = dt.category_id WHERE di.id = ?`, id).
		Scan(&ir.ID, &ir.DeviceID, &ir.DeviceAssetCode, &ir.DeviceTypeName, &ir.CategoryName,
			&ir.CategoryPrefix, &ir.DeviceTypePrefix,
			&ir.LocationInstalled, &ir.InstallationStartDate, &ir.InstallationFinishDate,
			&ir.Photo, &ir.Notes, &ir.CreatedAt, &ir.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &ir, nil
}

func (r *DeviceInstallationRepository) GetByDeviceID(deviceID int) (*models.DeviceInstallation, error) {
	var di models.DeviceInstallation
	var photo, notes sql.NullString
	err := r.db.QueryRow(`SELECT id, device_id, location_installed, installation_start_date,
		installation_finish_date, COALESCE(photo,''), COALESCE(notes,''), created_at, updated_at
		FROM device_installations WHERE device_id = ?`, deviceID).
		Scan(&di.ID, &di.DeviceID, &di.LocationInstalled, &di.InstallationStartDate, &di.InstallationFinishDate,
			&photo, &notes, &di.CreatedAt, &di.UpdatedAt)
	if err != nil {
		return nil, err
	}
	di.Photo = valStr(photo)
	di.Notes = valStr(notes)
	return &di, nil
}

func (r *DeviceInstallationRepository) GetDistinctLocations() ([]string, error) {
	rows, err := r.db.Query("SELECT DISTINCT location_installed FROM device_installations ORDER BY location_installed")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locs []string
	for rows.Next() {
		var l string
		if rows.Scan(&l) == nil {
			locs = append(locs, l)
		}
	}
	return locs, nil
}

func (r *DeviceInstallationRepository) GetInstallableDevices() ([]models.Device, error) {
	rows, err := r.db.Query(`SELECT d.id, d.asset_code, COALESCE(d.serial_number,''), d.condition
		FROM devices d
		JOIN device_types dt ON dt.id = d.device_type_id
		WHERE dt.usage_type = 'installable'
		AND d.condition = 'normal'
		AND d.id NOT IN (SELECT device_id FROM device_installations)
		ORDER BY d.asset_code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var d models.Device
		if rows.Scan(&d.ID, &d.AssetCode, &d.SerialNumber, &d.Condition) == nil {
			devices = append(devices, d)
		}
	}
	return devices, nil
}

func (r *DeviceInstallationRepository) Create(deviceID int, location string, startDate, finishDate *time.Time, photo, notes string) (sql.Result, error) {
	return r.db.Exec(`INSERT INTO device_installations (device_id, location_installed, installation_start_date, installation_finish_date, photo, notes)
		VALUES (?, ?, ?, ?, ?, ?)`, deviceID, location, startDate, finishDate, photo, notes)
}

func (r *DeviceInstallationRepository) Update(id int, location string, startDate, finishDate *time.Time, photo, notes string) error {
	_, err := r.db.Exec(`UPDATE device_installations SET location_installed=?, installation_start_date=?,
		installation_finish_date=?, photo=?, notes=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		location, startDate, finishDate, photo, notes, id)
	return err
}

func (r *DeviceInstallationRepository) Delete(id int) error {
	_, err := r.db.Exec("DELETE FROM device_installations WHERE id = ?", id)
	return err
}

type InstallationRow struct {
	models.DeviceInstallation
	DeviceAssetCode  string
	DeviceTypeName   string
	CategoryName     string
	CategoryPrefix   string
	DeviceTypePrefix string
}


