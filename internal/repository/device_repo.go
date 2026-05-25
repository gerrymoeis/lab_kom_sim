package repository

import (
	"database/sql"
	"fmt"
	"strings"
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

func (r *DeviceRepository) List(filters DeviceFilters) ([]models.DeviceWithCategory, error) {
	return r.listWithQuery(filters, "", 0, 0)
}

func (r *DeviceRepository) ListPaginated(filters DeviceFilters, page, pageSize int) ([]models.DeviceWithCategory, int, error) {
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 20 }

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
		clause += ` AND dt.category = ?`
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
	return `SELECT COUNT(*) FROM devices d JOIN device_types dt ON d.device_type_id = dt.id WHERE 1=1` + clause
}

func (r *DeviceRepository) buildDeviceArgs(filters DeviceFilters) []any {
	_, args := r.buildDeviceClause(filters)
	return args
}

func (r *DeviceRepository) listWithQuery(filters DeviceFilters, suffix string, limit, offset int) ([]models.DeviceWithCategory, error) {
	query := `SELECT d.id, d.device_type_id, d.asset_code, d.name, dt.category, d.brand, d.model,
		d.item_type, d.quantity_total, d.quantity_available, d.condition, d.location, d.created_at
		FROM devices d JOIN device_types dt ON d.device_type_id = dt.id WHERE 1=1`
	clause, args := r.buildDeviceClause(filters)
	query += clause

	sortBy := map[string]string{
		"name":       "d.name",
		"asset_code": "d.asset_code",
		"category":   "dt.category",
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

	var devices []models.DeviceWithCategory
	for rows.Next() {
		var d models.DeviceWithCategory
		var brand, model, cond, loc sql.NullString
		if err := rows.Scan(&d.ID, &d.DeviceTypeID, &d.AssetCode, &d.Name, &d.Category, &brand, &model,
			&d.ItemType, &d.QuantityTotal, &d.QuantityAvailable, &cond, &loc, &d.CreatedAt); err != nil {
			return nil, err
		}
		d.Brand = valStr(brand)
		d.Model = valStr(model)
		d.Condition = valStr(cond)
		d.Location = valStr(loc)
		devices = append(devices, d)
	}
	return devices, nil
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

func (r *DeviceRepository) GetByID(id int) (*models.DeviceWithCategory, error) {
	var d models.DeviceWithCategory
	var brand, model, serial, cond, loc, notes sql.NullString
	var pDate sql.NullString

	err := r.db.QueryRow(`SELECT d.id, d.device_type_id, d.asset_code, d.name, dt.category, d.brand, d.model,
		d.serial_number, d.item_type, d.is_loanable, d.is_consumable, d.quantity_total, d.quantity_available,
		d.condition, d.location, d.purchase_date, d.notes, d.created_at, d.updated_at
		FROM devices d JOIN device_types dt ON d.device_type_id = dt.id WHERE d.id = ?`, id).
		Scan(&d.ID, &d.DeviceTypeID, &d.AssetCode, &d.Name, &d.Category, &brand, &model,
			&serial, &d.ItemType, &d.IsLoanable, &d.IsConsumable, &d.QuantityTotal,
			&d.QuantityAvailable, &cond, &loc, &pDate, &notes, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	d.Brand = valStr(brand)
	d.Model = valStr(model)
	d.SerialNumber = valStr(serial)
	d.Condition = valStr(cond)
	d.Location = valStr(loc)
	d.Notes = valStr(notes)
	d.PurchaseDate = parseDate(pDate)
	return &d, nil
}

func (r *DeviceRepository) GetByIDSimple(id int) (*models.Device, error) {
	var d models.Device
	var brand, model, serial, cond, loc, notes sql.NullString
	var pDate sql.NullString
	err := r.db.QueryRow(`SELECT id, device_type_id, asset_code, name, brand, model, serial_number, item_type,
		is_loanable, is_consumable, quantity_total, quantity_available, condition, location, purchase_date, notes
		FROM devices WHERE id = ?`, id).
		Scan(&d.ID, &d.DeviceTypeID, &d.AssetCode, &d.Name, &brand, &model, &serial, &d.ItemType,
			&d.IsLoanable, &d.IsConsumable, &d.QuantityTotal, &d.QuantityAvailable, &cond, &loc, &pDate, &notes)
	if err != nil {
		return nil, err
	}
	d.Brand = valStr(brand)
	d.Model = valStr(model)
	d.SerialNumber = valStr(serial)
	d.Condition = valStr(cond)
	d.Location = valStr(loc)
	d.Notes = valStr(notes)
	d.PurchaseDate = parseDate(pDate)
	return &d, nil
}

func (r *DeviceRepository) GetNextAssetCode(prefix string) string {
	var next int
	r.db.QueryRow(`SELECT COALESCE(MAX(CAST(SUBSTRING(asset_code, LENGTH(?) + 2) AS INTEGER)) + 1, 1) FROM devices WHERE asset_code LIKE ? || '-%'`, prefix, prefix).Scan(&next)
	return fmt.Sprintf("%s-%03d", prefix, next)
}

func (r *DeviceRepository) GetLoansByDevice(deviceID, limit int) ([]models.DeviceLoan, error) {
	rows, err := r.db.Query(`SELECT id, borrower_name, loan_date, expected_return_date, actual_return_date, quantity, status FROM device_loans WHERE device_id = ? ORDER BY loan_date DESC LIMIT ?`, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var loans []models.DeviceLoan
	for rows.Next() {
		var l models.DeviceLoan
		if rows.Scan(&l.ID, &l.BorrowerName, &l.LoanDate, &l.ExpectedReturnDate, &l.ActualReturnDate, &l.Quantity, &l.Status) == nil {
			loans = append(loans, l)
		}
	}
	return loans, nil
}

func (r *DeviceRepository) GetUsagesByDevice(deviceID, limit int) ([]models.DeviceUsage, error) {
	rows, err := r.db.Query(`SELECT id, user_name, usage_date, quantity, purpose, is_available FROM device_usages WHERE device_id = ? ORDER BY usage_date DESC LIMIT ?`, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usages []models.DeviceUsage
	for rows.Next() {
		var u models.DeviceUsage
		if rows.Scan(&u.ID, &u.UserName, &u.UsageDate, &u.Quantity, &u.Purpose, &u.IsAvailable) == nil {
			usages = append(usages, u)
		}
	}
	return usages, nil
}

func (r *DeviceRepository) Create(dtID int, code, name, brand, model, serial, itemType string, isLoanable, isConsumable bool, qty int, condition, location, pDate, notes string) (sql.Result, error) {
	return r.db.Exec(`INSERT INTO devices (device_type_id, asset_code, name, brand, model, serial_number,
		item_type, is_loanable, is_consumable, quantity_total, quantity_available, condition, location, purchase_date, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dtID, code, name, brand, model, serial, itemType, isLoanable, isConsumable, qty, qty, condition, location, pDate, notes)
}

func (r *DeviceRepository) Update(id, dtID int, name, brand, model, serial, itemType string, isLoanable, isConsumable bool, qtyTotal, qtyAvail int, condition, location, pDate, notes string) error {
	_, err := r.db.Exec(`UPDATE devices SET device_type_id=?, name=?, brand=?, model=?, serial_number=?,
		item_type=?, is_loanable=?, is_consumable=?, quantity_total=?, quantity_available=?, condition=?,
		location=?, purchase_date=?, notes=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		dtID, name, brand, model, serial, itemType, isLoanable, isConsumable, qtyTotal, qtyAvail, condition, location, pDate, notes, id)
	return err
}

func (r *DeviceRepository) Delete(id int) error {
	_, err := r.db.Exec(`DELETE FROM devices WHERE id = ?`, id)
	return err
}

// --- List page queries (inner joins with computed status) ---

func (r *DeviceRepository) ListLoans() ([]DeviceLoanRow, error) {
	return r.listLoansWithQuery("")
}

func (r *DeviceRepository) ListLoansPaginated(search, status, sortBy string, page, pageSize int) ([]DeviceLoanRow, int, error) {
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 20 }

	countQuery := `SELECT COUNT(*) FROM device_loans l JOIN devices d ON l.device_id = d.id WHERE 1=1`
	var args []any
	if status != "" {
		countQuery += ` AND CASE WHEN l.actual_return_date IS NOT NULL THEN 'returned'
			WHEN l.expected_return_date IS NOT NULL AND CURRENT_DATE > l.expected_return_date THEN 'overdue'
			ELSE 'active' END = ?`
		args = append(args, status)
	}
	if search != "" {
		sClause, sArgs := r.search.Where("device_loan", search)
		countQuery += sClause
		args = append(args, sArgs...)
	}
	var total int
	r.db.QueryRow(countQuery, args...).Scan(&total)

	query := `SELECT l.id, l.device_id, d.name, d.asset_code, l.borrower_name, l.borrower_type,
		l.loan_date, l.expected_return_date, l.actual_return_date, l.quantity, l.status, l.purpose,
		CASE WHEN l.actual_return_date IS NOT NULL THEN 'returned'
			WHEN l.expected_return_date IS NOT NULL AND CURRENT_DATE > l.expected_return_date THEN 'overdue'
			ELSE 'active' END
		FROM device_loans l JOIN devices d ON l.device_id = d.id WHERE 1=1`
	var dataArgs []any
	if status != "" {
		query += ` AND CASE WHEN l.actual_return_date IS NOT NULL THEN 'returned'
			WHEN l.expected_return_date IS NOT NULL AND CURRENT_DATE > l.expected_return_date THEN 'overdue'
			ELSE 'active' END = ?`
		dataArgs = append(dataArgs, status)
	}
	if search != "" {
		sClause, sArgs := r.search.Where("device_loan", search)
		query += sClause
		dataArgs = append(dataArgs, sArgs...)
	}
	loanSortBy := "l.loan_date"
	switch sortBy {
	case "borrower_name":
		loanSortBy = "l.borrower_name"
	}
	query += ` ORDER BY ` + loanSortBy + ` DESC LIMIT ? OFFSET ?`
	dataArgs = append(dataArgs, pageSize, (page-1)*pageSize)

	rows, err := r.db.Query(query, dataArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var loans []DeviceLoanRow
	for rows.Next() {
		var l DeviceLoanRow
		if err := rows.Scan(&l.ID, &l.DeviceID, &l.DeviceName, &l.DeviceAssetCode,
			&l.BorrowerName, &l.BorrowerType, &l.LoanDate, &l.ExpectedReturnDate,
			&l.ActualReturnDate, &l.Quantity, &l.Status, &l.Purpose, &l.ComputedStatus); err != nil {
			return nil, 0, err
		}
		loans = append(loans, l)
	}
	return loans, total, nil
}

func (r *DeviceRepository) listLoansWithQuery(suffix string) ([]DeviceLoanRow, error) {
	rows, err := r.db.Query(`SELECT l.id, l.device_id, d.name, d.asset_code, l.borrower_name, l.borrower_type,
		l.loan_date, l.expected_return_date, l.actual_return_date, l.quantity, l.status, l.purpose,
		CASE WHEN l.actual_return_date IS NOT NULL THEN 'returned'
			WHEN l.expected_return_date IS NOT NULL AND CURRENT_DATE > l.expected_return_date THEN 'overdue'
			ELSE 'active' END
		FROM device_loans l JOIN devices d ON l.device_id = d.id ORDER BY l.loan_date DESC`+suffix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var loans []DeviceLoanRow
	for rows.Next() {
		var l DeviceLoanRow
		if err := rows.Scan(&l.ID, &l.DeviceID, &l.DeviceName, &l.DeviceAssetCode,
			&l.BorrowerName, &l.BorrowerType, &l.LoanDate, &l.ExpectedReturnDate,
			&l.ActualReturnDate, &l.Quantity, &l.Status, &l.Purpose, &l.ComputedStatus); err != nil {
			return nil, err
		}
		loans = append(loans, l)
	}
	return loans, nil
}

func (r *DeviceRepository) ListUsages() ([]DeviceUsageRow, error) {
	return r.listUsagesWithQuery("")
}

func (r *DeviceRepository) ListUsagesPaginated(search, sortBy string, page, pageSize int) ([]DeviceUsageRow, int, error) {
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 20 }

	var total int
	countQuery := `SELECT COUNT(*) FROM device_usages u JOIN devices d ON u.device_id = d.id WHERE 1=1`
	var args []any
	if search != "" {
		sClause, sArgs := r.search.Where("device_usage", search)
		countQuery += sClause
		args = append(args, sArgs...)
	}
	r.db.QueryRow(countQuery, args...).Scan(&total)

	query := `SELECT u.id, u.device_id, d.asset_code, d.name, u.user_name, u.user_type,
		u.usage_date, u.quantity, u.is_available, u.purpose
		FROM device_usages u JOIN devices d ON u.device_id = d.id WHERE 1=1`
	var dataArgs []any
	if search != "" {
		sClause, sArgs := r.search.Where("device_usage", search)
		query += sClause
		dataArgs = append(dataArgs, sArgs...)
	}
	usageSortBy := "u.usage_date"
	switch sortBy {
	case "user_name":
		usageSortBy = "u.user_name"
	case "device_name":
		usageSortBy = "d.name"
	}
	query += ` ORDER BY ` + usageSortBy + ` DESC LIMIT ? OFFSET ?`
	dataArgs = append(dataArgs, pageSize, (page-1)*pageSize)

	rows, err := r.db.Query(query, dataArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var usages []DeviceUsageRow
	for rows.Next() {
		var u DeviceUsageRow
		if err := rows.Scan(&u.ID, &u.DeviceID, &u.DeviceAssetCode, &u.DeviceName,
			&u.UserName, &u.UserType, &u.UsageDate, &u.Quantity, &u.IsAvailable, &u.Purpose); err != nil {
			return nil, 0, err
		}
		usages = append(usages, u)
	}
	return usages, total, nil
}

func (r *DeviceRepository) listUsagesWithQuery(suffix string) ([]DeviceUsageRow, error) {
	rows, err := r.db.Query(`SELECT u.id, u.device_id, d.asset_code, d.name, u.user_name, u.user_type,
		u.usage_date, u.quantity, u.is_available, u.purpose
		FROM device_usages u JOIN devices d ON u.device_id = d.id ORDER BY u.usage_date DESC`+suffix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usages []DeviceUsageRow
	for rows.Next() {
		var u DeviceUsageRow
		if err := rows.Scan(&u.ID, &u.DeviceID, &u.DeviceAssetCode, &u.DeviceName,
			&u.UserName, &u.UserType, &u.UsageDate, &u.Quantity, &u.IsAvailable, &u.Purpose); err != nil {
			return nil, err
		}
		usages = append(usages, u)
	}
	return usages, nil
}

// --- Export helpers ---

type DeviceExportRow struct {
	models.Device
	Category string
}

func (r *DeviceRepository) ExportAll() ([]DeviceExportRow, error) {
	rows, err := r.db.Query(`SELECT d.id, d.device_type_id, d.asset_code, d.name, dt.category,
		d.brand, d.model, d.serial_number, d.item_type, d.is_loanable, d.is_consumable,
		d.quantity_total, d.quantity_available, d.condition, d.location, d.purchase_date, d.notes
		FROM devices d JOIN device_types dt ON d.device_type_id = dt.id ORDER BY d.asset_code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []DeviceExportRow
	for rows.Next() {
		var d DeviceExportRow
		var brand, model, serial, cond, loc, notes sql.NullString
		var pDate sql.NullString
		if err := rows.Scan(&d.ID, &d.DeviceTypeID, &d.AssetCode, &d.Name, &d.Category,
			&brand, &model, &serial, &d.ItemType, &d.IsLoanable, &d.IsConsumable,
			&d.QuantityTotal, &d.QuantityAvailable, &cond, &loc, &pDate, &notes); err != nil {
			return nil, err
		}
		d.Brand = valStr(brand)
		d.Model = valStr(model)
		d.SerialNumber = valStr(serial)
		d.Condition = valStr(cond)
		d.Location = valStr(loc)
		d.Notes = valStr(notes)
		if strings.TrimSpace(valStr(pDate)) != "" {
			// keep as-is, exported as string
		}
		devices = append(devices, d)
	}
	return devices, nil
}

type DeviceTypeExportRow struct {
	models.DeviceType
}

func (r *DeviceRepository) ExportDeviceTypes() ([]DeviceTypeExportRow, error) {
	rows, err := r.db.Query(`SELECT id, name, category, COALESCE(brand, '-'), COALESCE(model, '-'),
		item_type, is_loanable, is_consumable, COALESCE(asset_code_prefix, '-'), COALESCE(default_location, '-')
		FROM device_types ORDER BY category, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dts []DeviceTypeExportRow
	for rows.Next() {
		var dt DeviceTypeExportRow
		if err := rows.Scan(&dt.ID, &dt.Name, &dt.Category, &dt.Brand, &dt.Model,
			&dt.ItemType, &dt.IsLoanable, &dt.IsConsumable, &dt.AssetCodePrefix, &dt.DefaultLocation); err != nil {
			return nil, err
		}
		dts = append(dts, dt)
	}
	return dts, nil
}

func (r *DeviceRepository) ExportLoans() ([]DeviceLoanRow, error) {
	return r.ListLoans()
}

func (r *DeviceRepository) ExportUsages() ([]DeviceUsageRow, error) {
	return r.ListUsages()
}

func (r *DeviceRepository) DeductQuantity(deviceID, quantity int) error {
	res, err := r.db.Exec(`UPDATE devices SET quantity_available = quantity_available - ? WHERE id = ? AND quantity_available >= ?`, quantity, deviceID, quantity)
	if err != nil { return err }
	if n, _ := res.RowsAffected(); n == 0 { return sql.ErrNoRows }
	return nil
}

func (r *DeviceRepository) RestoreQuantity(deviceID, quantity int) error {
	_, err := r.db.Exec(`UPDATE devices SET quantity_available = quantity_available + ? WHERE id = ?`, quantity, deviceID)
	return err
}

func (r *DeviceRepository) SetQuantity(deviceID, delta int) error {
	_, err := r.db.Exec(`UPDATE devices SET quantity_available = quantity_available + ? WHERE id = ?`, delta, deviceID)
	return err
}
