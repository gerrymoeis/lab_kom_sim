package repository

import (
	"database/sql"
	"fmt"
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/search"
)

type DeviceRepository struct {
	db     DBTX
	search *search.Builder
}

func NewDeviceRepository(db *database.DB) *DeviceRepository {
	return &DeviceRepository{db: db, search: search.New(db)}
}

func (r *DeviceRepository) WithTx(tx *database.Tx) *DeviceRepository {
	return &DeviceRepository{db: tx, search: r.search}
}

type DeviceFilters struct {
	Search    string
	Category  string
	Condition string
	SortBy    string
	SortOrder string
}

func (r *DeviceRepository) List(filters DeviceFilters) ([]models.Device, error) {
	return r.listWithQuery(filters, "", 0, 0)
}

func (r *DeviceRepository) ListPaginated(filters DeviceFilters, page, pageSize int) ([]models.Device, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}

	var total int
	r.db.QueryRow(r.buildDeviceCountQuery(filters), r.buildDeviceArgs(filters)...).Scan(&total)

	devices, err := r.listWithQuery(filters, ` LIMIT ? OFFSET ?`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	return devices, total, nil
}

func (r *DeviceRepository) buildDeviceClause(filters DeviceFilters) (string, []any) {
	var clause string
	var args []any
	if filters.Search != "" {
		sClause, sArgs := r.search.Where("device", filters.Search)
		clause += sClause
		args = append(args, sArgs...)
	}
	if filters.Category != "" {
		clause += ` AND c.name = ?`
		args = append(args, filters.Category)
	}
	if filters.Condition != "" {
		clause += ` AND d.condition = ?`
		args = append(args, filters.Condition)
	}
	return clause, args
}

func (r *DeviceRepository) buildDeviceCountQuery(filters DeviceFilters) string {
	clause, _ := r.buildDeviceClause(filters)
	return `SELECT COUNT(*) FROM devices d
		JOIN device_types dt ON d.device_type_id = dt.id
		JOIN categories c ON c.id = dt.category_id WHERE 1=1` + clause
}

func (r *DeviceRepository) buildDeviceArgs(filters DeviceFilters) []any {
	_, args := r.buildDeviceClause(filters)
	return args
}

func (r *DeviceRepository) listWithQuery(filters DeviceFilters, suffix string, limit, offset int) ([]models.Device, error) {
	query := `SELECT d.id, d.device_type_id, d.asset_code, COALESCE(d.serial_number,''),
		d.condition, COALESCE(d.location,''), d.purchase_date, COALESCE(d.notes,''),
		d.created_at, d.updated_at,
		c.name, c.default_prefix, dt.name, dt.asset_code_prefix,
		COALESCE(d.usage_type, dt.usage_type) AS usage_type,
		COALESCE(d.usage_type, '') AS usage_type_override,
		COALESCE(dt.photo,'')
		FROM devices d
		JOIN device_types dt ON d.device_type_id = dt.id
		JOIN categories c ON c.id = dt.category_id WHERE 1=1`
	clause, args := r.buildDeviceClause(filters)
	query += clause

	sortBy := map[string]string{
		"asset_code": "d.asset_code",
		"category":   "c.name",
		"condition":  "d.condition",
		"created_at": "d.created_at",
	}[filters.SortBy]
	if sortBy == "" {
		sortBy = "d.asset_code"
	}
	sortOrder := "ASC"
	if filters.SortOrder == "DESC" {
		sortOrder = "DESC"
	}
	query += ` ORDER BY ` + sortBy + ` ` + sortOrder

	query += suffix
	if suffix != "" {
		args = append(args, limit, offset)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var d models.Device
		var pDate sql.NullString
		if err := rows.Scan(&d.ID, &d.DeviceTypeID, &d.AssetCode, &d.SerialNumber,
			&d.Condition, &d.Location, &pDate, &d.Notes,
			&d.CreatedAt, &d.UpdatedAt,
			&d.CategoryName, &d.CategoryPrefix, &d.DeviceTypeName, &d.DeviceTypePrefix,
			&d.UsageType, &d.UsageTypeOverride, &d.DeviceTypePhoto); err != nil {
			return nil, err
		}
		d.PurchaseDate = parseDate(pDate)
		devices = append(devices, d)
	}
	return devices, nil
}

func (r *DeviceRepository) GetByID(id int) (*models.Device, error) {
	var d models.Device
	var pDate sql.NullString
	err := r.db.QueryRow(`SELECT d.id, d.device_type_id, d.asset_code, COALESCE(d.serial_number,''),
		d.condition, COALESCE(d.location,''), d.purchase_date, COALESCE(d.notes,''),
		d.created_at, d.updated_at,
		c.name, c.default_prefix, dt.name, dt.asset_code_prefix,
		COALESCE(d.usage_type, dt.usage_type) AS usage_type,
		COALESCE(d.usage_type, '') AS usage_type_override,
		COALESCE(dt.photo,'')
		FROM devices d
		JOIN device_types dt ON d.device_type_id = dt.id
		JOIN categories c ON c.id = dt.category_id WHERE d.id = ?`, id).
		Scan(&d.ID, &d.DeviceTypeID, &d.AssetCode, &d.SerialNumber,
			&d.Condition, &d.Location, &pDate, &d.Notes,
			&d.CreatedAt, &d.UpdatedAt,
			&d.CategoryName, &d.CategoryPrefix, &d.DeviceTypeName, &d.DeviceTypePrefix,
			&d.UsageType, &d.UsageTypeOverride, &d.DeviceTypePhoto)
	if err != nil {
		return nil, err
	}
	d.PurchaseDate = parseDate(pDate)
	return &d, nil
}

func (r *DeviceRepository) GetByAssetCode(code string) (*models.Device, error) {
	var d models.Device
	var pDate sql.NullString
	err := r.db.QueryRow(`SELECT d.id, d.device_type_id, d.asset_code, COALESCE(d.serial_number,''),
		d.condition, COALESCE(d.location,''), d.purchase_date, COALESCE(d.notes,''),
		d.created_at, d.updated_at,
		c.name, c.default_prefix, dt.name, dt.asset_code_prefix,
		COALESCE(d.usage_type, dt.usage_type) AS usage_type,
		COALESCE(d.usage_type, '') AS usage_type_override,
		COALESCE(dt.photo,'')
		FROM devices d
		JOIN device_types dt ON d.device_type_id = dt.id
		JOIN categories c ON c.id = dt.category_id WHERE d.asset_code = ?`, code).
		Scan(&d.ID, &d.DeviceTypeID, &d.AssetCode, &d.SerialNumber,
			&d.Condition, &d.Location, &pDate, &d.Notes,
			&d.CreatedAt, &d.UpdatedAt,
			&d.CategoryName, &d.CategoryPrefix, &d.DeviceTypeName, &d.DeviceTypePrefix,
			&d.UsageType, &d.UsageTypeOverride, &d.DeviceTypePhoto)
	if err != nil {
		return nil, err
	}
	d.PurchaseDate = parseDate(pDate)
	return &d, nil
}

func (r *DeviceRepository) GetNextAssetCode(prefix string) string {
	var next int
	r.db.QueryRow(`SELECT COALESCE(MAX(CAST(SUBSTR(asset_code, LENGTH(?) + 2) AS INTEGER)) + 1, 1)
		FROM devices WHERE asset_code LIKE ? || '-%'`, prefix, prefix).Scan(&next)
	return fmt.Sprintf("%s-%03d", prefix, next)
}

type BatchCreateInput struct {
	DeviceTypeID int
	AssetCode    string
	SerialNumber string
	Condition    string
	Location     string
	PurchaseDate string
	Notes        string
}

func (r *DeviceRepository) Create(deviceTypeID int, assetCode, serial, condition, location, pDate, notes string) (sql.Result, error) {
	return r.db.Exec(`INSERT INTO devices (device_type_id, asset_code, serial_number, condition, location, purchase_date, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, deviceTypeID, assetCode, serial, condition, location, pDate, notes)
}

func (r *DeviceRepository) BatchCreate(inputs []BatchCreateInput) error {
	rawDB, ok := r.db.(*database.DB)
	if !ok {
		return fmt.Errorf("batch create requires direct database access")
	}
	tx, err := rawDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, in := range inputs {
		if _, err := tx.Exec(`INSERT INTO devices (device_type_id, asset_code, serial_number, condition, location, purchase_date, notes)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, in.DeviceTypeID, in.AssetCode, in.SerialNumber, in.Condition, in.Location, in.PurchaseDate, in.Notes); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *DeviceRepository) Update(id, deviceTypeID int, assetCode, serial, condition, location, pDate, notes, usageType string) error {
	var usageArg interface{}
	if usageType != "" {
		usageArg = usageType
	}
	_, err := r.db.Exec(`UPDATE devices SET device_type_id=?, asset_code=?, serial_number=?,
		condition=?, location=?, purchase_date=?, notes=?, usage_type=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		deviceTypeID, assetCode, serial, condition, location, pDate, notes, usageArg, id)
	return err
}

func (r *DeviceRepository) Delete(id int) error {
	_, err := r.db.Exec("DELETE FROM devices WHERE id = ?", id)
	return err
}

// --- Grouped query for main device view (single JOIN, no N+1) ---

type DeviceGroupRow struct {
	CategoryID     int
	CategoryName   string
	CategoryPrefix string
	TypeID         int
	TypeName       string
	TypePrefix     string
	TypeUsageType  string // from dt.usage_type (device type level)
	TypePhoto      string
	DeviceID       *int
	AssetCode      *string
	SerialNumber   *string
	Condition      *string
	Location       *string
	PurchaseDate   *string
	Notes          *string
	DeviceUsageType *string // from d.usage_type (device-level override, nil = no override)
}

func (r *DeviceRepository) GetAllGrouped() ([]DeviceGroupRow, error) {
	rows, err := r.db.Query(`SELECT c.id, c.name, c.default_prefix,
		dt.id, dt.name, dt.asset_code_prefix, dt.usage_type, COALESCE(dt.photo,''),
		d.id, d.asset_code, COALESCE(d.serial_number,''), d.condition,
		COALESCE(d.location,''), d.purchase_date, COALESCE(d.notes,''),
		d.usage_type
		FROM categories c
		LEFT JOIN device_types dt ON dt.category_id = c.id
		LEFT JOIN devices d ON d.device_type_id = dt.id
		ORDER BY c.name, dt.name, d.asset_code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rows2 []DeviceGroupRow
	for rows.Next() {
		var r2 DeviceGroupRow
		if err := rows.Scan(&r2.CategoryID, &r2.CategoryName, &r2.CategoryPrefix,
			&r2.TypeID, &r2.TypeName, &r2.TypePrefix, &r2.TypeUsageType, &r2.TypePhoto,
			&r2.DeviceID, &r2.AssetCode, &r2.SerialNumber, &r2.Condition,
			&r2.Location, &r2.PurchaseDate, &r2.Notes, &r2.DeviceUsageType); err != nil {
			return nil, err
		}
		rows2 = append(rows2, r2)
	}
	return rows2, nil
}

// --- Batch status queries (1 query each, not per-device) ---

func (r *DeviceRepository) GetActiveLoanDeviceIDs() (map[int]bool, error) {
	rows, err := r.db.Query("SELECT device_id FROM device_loans WHERE actual_return_date IS NULL")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]bool)
	for rows.Next() {
		var id int
		if rows.Scan(&id) == nil {
			result[id] = true
		}
	}
	return result, nil
}

func (r *DeviceRepository) GetDepletedDeviceIDs() (map[int]bool, error) {
	rows, err := r.db.Query(`SELECT du.device_id FROM device_usages du
		INNER JOIN (SELECT device_id, MAX(id) as mid FROM device_usages GROUP BY device_id) latest
		ON latest.device_id = du.device_id AND latest.mid = du.id
		WHERE du.is_available = 'no'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]bool)
	for rows.Next() {
		var id int
		if rows.Scan(&id) == nil {
			result[id] = true
		}
	}
	return result, nil
}

func (r *DeviceRepository) GetInstalledDeviceIDs() (map[int]bool, error) {
	rows, err := r.db.Query("SELECT device_id FROM device_installations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]bool)
	for rows.Next() {
		var id int
		if rows.Scan(&id) == nil {
			result[id] = true
		}
	}
	return result, nil
}

// --- Export helpers ---

type DeviceExportRow struct {
	models.Device
}

func (r *DeviceRepository) ExportAll() ([]DeviceExportRow, error) {
	rows, err := r.db.Query(`SELECT d.id, d.device_type_id, d.asset_code, COALESCE(d.serial_number,''),
		d.condition, COALESCE(d.location,''), d.purchase_date, COALESCE(d.notes,''),
		c.name, c.default_prefix, dt.name, dt.asset_code_prefix,
		COALESCE(d.usage_type, dt.usage_type) AS usage_type,
		COALESCE(d.usage_type, '') AS usage_type_override,
		COALESCE(dt.photo,'')
		FROM devices d
		JOIN device_types dt ON d.device_type_id = dt.id
		JOIN categories c ON c.id = dt.category_id
		ORDER BY d.asset_code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []DeviceExportRow
	for rows.Next() {
		var d DeviceExportRow
		var pDate sql.NullString
		if err := rows.Scan(&d.ID, &d.DeviceTypeID, &d.AssetCode, &d.SerialNumber,
			&d.Condition, &d.Location, &pDate, &d.Notes,
			&d.CategoryName, &d.CategoryPrefix, &d.DeviceTypeName, &d.DeviceTypePrefix,
			&d.UsageType, &d.UsageTypeOverride, &d.DeviceTypePhoto); err != nil {
			return nil, err
		}
		d.PurchaseDate = parseDate(pDate)
		devices = append(devices, d)
	}
	return devices, nil
}

type DeviceTypeExportRow struct {
	models.DeviceType
}

func (r *DeviceRepository) ExportDeviceTypes() ([]DeviceTypeExportRow, error) {
	rows, err := r.db.Query(`SELECT dt.id, dt.category_id, c.name, dt.name,
		COALESCE(dt.brand,''), COALESCE(dt.model,''), dt.asset_code_prefix, dt.usage_type,
		COALESCE(dt.default_location,''), COALESCE(dt.photo,'')
		FROM device_types dt JOIN categories c ON c.id = dt.category_id ORDER BY c.name, dt.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dts []DeviceTypeExportRow
	for rows.Next() {
		var dt DeviceTypeExportRow
		if err := rows.Scan(&dt.ID, &dt.CategoryID, &dt.CategoryName, &dt.Name,
			&dt.Brand, &dt.Model, &dt.AssetCodePrefix, &dt.UsageType,
			&dt.DefaultLocation, &dt.Photo); err != nil {
			return nil, err
		}
		dts = append(dts, dt)
	}
	return dts, nil
}

func (r *DeviceRepository) ExportLoans(pageSize int) ([]DeviceLoanRow, error) {
	loanRepo := &DeviceLoanRepository{db: r.db, search: r.search}
	loans, _, err := loanRepo.ListPaginated(DeviceLoanFilters{}, 1, pageSize)
	return loans, err
}

func (r *DeviceRepository) ExportUsages(pageSize int) ([]DeviceUsageRow, error) {
	usageRepo := &DeviceUsageRepository{db: r.db, search: r.search}
	usages, _, err := usageRepo.ListPaginated(DeviceUsageFilters{}, 1, pageSize)
	return usages, err
}

func parseDate(s sql.NullString) *time.Time {
	if s.Valid && s.String != "" {
		t, err := time.Parse("2006-01-02", s.String)
		if err == nil {
			return &t
		}
	}
	return nil
}
