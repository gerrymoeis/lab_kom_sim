package repository

import (
	"database/sql"

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
	Search string
	SortBy string
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
		where, args = r.search.Where("device_installation", filters.Search)
	}

	var total int
	r.db.QueryRow(`SELECT COUNT(*) FROM device_installations di
		JOIN devices d ON d.id = di.device_id
		JOIN device_types dt ON dt.id = d.device_type_id`+where, args...).Scan(&total)

	orderBy := "di.installation_start_date"
	if filters.SortBy == "location" {
		orderBy = "di.location_installed"
	} else if filters.SortBy == "asset_code" {
		orderBy = "d.asset_code"
	} else if filters.SortBy == "start_date" {
		orderBy = "di.installation_start_date"
	}

	query := `SELECT di.id, di.device_id, d.asset_code, dt.name, c.name,
		di.location_installed, di.installation_start_date, di.installation_finish_date,
		COALESCE(di.photo,''), COALESCE(di.notes,''), di.created_at, di.updated_at
		FROM device_installations di
		JOIN devices d ON d.id = di.device_id
		JOIN device_types dt ON dt.id = d.device_type_id
		JOIN categories c ON c.id = dt.category_id WHERE 1=1` + where +
		` ORDER BY ` + orderBy + ` DESC LIMIT ? OFFSET ?`

	allArgs := append(args, pageSize, (page-1)*pageSize)
	rows, err := r.db.Query(query, allArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var installations []InstallationRow
	for rows.Next() {
		var ir InstallationRow
		var startDate, finishDate sql.NullString
		if err := rows.Scan(&ir.ID, &ir.DeviceID, &ir.DeviceAssetCode, &ir.DeviceTypeName, &ir.CategoryName,
			&ir.LocationInstalled, &startDate, &finishDate,
			&ir.Photo, &ir.Notes, &ir.CreatedAt, &ir.UpdatedAt); err != nil {
			return nil, 0, err
		}
		ir.InstallationStartDate = parseDate(startDate)
		ir.InstallationFinishDate = parseDate(finishDate)
		installations = append(installations, ir)
	}
	return installations, total, nil
}

func (r *DeviceInstallationRepository) GetByID(id int) (*InstallationRow, error) {
	var ir InstallationRow
	var startDate, finishDate sql.NullString
	err := r.db.QueryRow(`SELECT di.id, di.device_id, d.asset_code, dt.name, c.name,
		di.location_installed, di.installation_start_date, di.installation_finish_date,
		COALESCE(di.photo,''), COALESCE(di.notes,''), di.created_at, di.updated_at
		FROM device_installations di
		JOIN devices d ON d.id = di.device_id
		JOIN device_types dt ON dt.id = d.device_type_id
		JOIN categories c ON c.id = dt.category_id WHERE di.id = ?`, id).
		Scan(&ir.ID, &ir.DeviceID, &ir.DeviceAssetCode, &ir.DeviceTypeName, &ir.CategoryName,
			&ir.LocationInstalled, &startDate, &finishDate,
			&ir.Photo, &ir.Notes, &ir.CreatedAt, &ir.UpdatedAt)
	if err != nil {
		return nil, err
	}
	ir.InstallationStartDate = parseDate(startDate)
	ir.InstallationFinishDate = parseDate(finishDate)
	return &ir, nil
}

func (r *DeviceInstallationRepository) GetByDeviceID(deviceID int) (*models.DeviceInstallation, error) {
	var di models.DeviceInstallation
	var startDate, finishDate, photo, notes sql.NullString
	err := r.db.QueryRow(`SELECT id, device_id, location_installed, installation_start_date,
		installation_finish_date, COALESCE(photo,''), COALESCE(notes,''), created_at, updated_at
		FROM device_installations WHERE device_id = ?`, deviceID).
		Scan(&di.ID, &di.DeviceID, &di.LocationInstalled, &startDate, &finishDate,
			&photo, &notes, &di.CreatedAt, &di.UpdatedAt)
	if err != nil {
		return nil, err
	}
	di.InstallationStartDate = parseDate(startDate)
	di.InstallationFinishDate = parseDate(finishDate)
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

func (r *DeviceInstallationRepository) Create(deviceID int, location string, startDate, finishDate sql.NullString, photo, notes string) (sql.Result, error) {
	return r.db.Exec(`INSERT INTO device_installations (device_id, location_installed, installation_start_date, installation_finish_date, photo, notes)
		VALUES (?, ?, ?, ?, ?, ?)`, deviceID, location, startDate, finishDate, photo, notes)
}

func (r *DeviceInstallationRepository) Update(id int, location string, startDate, finishDate sql.NullString, photo, notes string) error {
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
	DeviceAssetCode string
	DeviceTypeName  string
	CategoryName    string
}


