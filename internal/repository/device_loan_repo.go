package repository

import (
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
)

type DeviceLoanRepository struct {
	db DBTX
}

func NewDeviceLoanRepository(db *database.DB) *DeviceLoanRepository {
	return &DeviceLoanRepository{db: db}
}

func (r *DeviceLoanRepository) WithTx(tx *database.Tx) *DeviceLoanRepository {
	return &DeviceLoanRepository{db: tx}
}

type DeviceLoanFilters struct {
	Status string
	Search string
	SortBy string
}

func (r *DeviceLoanRepository) List(filters DeviceLoanFilters) ([]DeviceLoanRow, error) {
	return r.listWithQuery(filters, "")
}

func (r *DeviceLoanRepository) ListPaginated(filters DeviceLoanFilters, page, pageSize int) ([]DeviceLoanRow, int, error) {
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 20 }

	loanClause, loanArgs := r.buildLoanClause(filters)

	var total int
	r.db.QueryRow(`SELECT COUNT(*) FROM device_loans l JOIN devices d ON l.device_id = d.id WHERE 1=1`+loanClause, loanArgs...).Scan(&total)

	query := `SELECT l.id, l.device_id, d.name, d.asset_code, l.borrower_name, l.borrower_type,
		l.loan_date, l.expected_return_date, l.actual_return_date, l.quantity, l.status, l.purpose,
		CASE WHEN l.actual_return_date IS NOT NULL THEN 'returned'
			WHEN l.expected_return_date IS NOT NULL AND CURRENT_DATE > l.expected_return_date THEN 'overdue'
			ELSE 'active' END as computed_status
		FROM device_loans l JOIN devices d ON l.device_id = d.id WHERE 1=1` + loanClause

	orderBy := "l.loan_date"
	if filters.SortBy == "borrower_name" {
		orderBy = "l.borrower_name"
	}
	query += ` ORDER BY ` + orderBy + ` DESC LIMIT ? OFFSET ?`

	allArgs := append(loanArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.Query(query, allArgs...)
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

func (r *DeviceLoanRepository) buildLoanClause(filters DeviceLoanFilters) (string, []any) {
	var clause string
	var args []any
	if filters.Status != "" {
		clause += ` AND CASE WHEN l.actual_return_date IS NOT NULL THEN 'returned'
			WHEN l.expected_return_date IS NOT NULL AND CURRENT_DATE > l.expected_return_date THEN 'overdue'
			ELSE 'active' END = ?`
		args = append(args, filters.Status)
	}
	if filters.Search != "" {
		clause += ` AND (l.borrower_name LIKE ? OR d.name LIKE ? OR d.asset_code LIKE ?)`
		s := "%" + filters.Search + "%"
		args = append(args, s, s, s)
	}
	return clause, args
}

func (r *DeviceLoanRepository) listWithQuery(filters DeviceLoanFilters, suffix string) ([]DeviceLoanRow, error) {
	_, loanArgs := r.buildLoanClause(filters)
	query := `SELECT l.id, l.device_id, d.name, d.asset_code, l.borrower_name, l.borrower_type,
		l.loan_date, l.expected_return_date, l.actual_return_date, l.quantity, l.status, l.purpose,
		CASE WHEN l.actual_return_date IS NOT NULL THEN 'returned'
			WHEN l.expected_return_date IS NOT NULL AND CURRENT_DATE > l.expected_return_date THEN 'overdue'
			ELSE 'active' END as computed_status
		FROM device_loans l JOIN devices d ON l.device_id = d.id WHERE 1=1`
	clause, _ := r.buildLoanClause(filters)
	query += clause

	orderBy := "l.loan_date"
	if filters.SortBy == "borrower_name" {
		orderBy = "l.borrower_name"
	}
	query += ` ORDER BY ` + orderBy + ` DESC` + suffix

	rows, err := r.db.Query(query, loanArgs...)
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

type DeviceLoanRow struct {
	models.DeviceLoan
	DeviceName      string
	DeviceAssetCode string
}

func (r *DeviceLoanRepository) GetByID(id int) (*DeviceLoanRow, error) {
	var l DeviceLoanRow
	err := r.db.QueryRow(`SELECT l.id, l.device_id, d.name, d.asset_code, l.borrower_name, l.borrower_type,
		l.loan_date, l.expected_return_date, l.actual_return_date, l.quantity, l.status, l.purpose, COALESCE(l.notes,'')
		FROM device_loans l JOIN devices d ON l.device_id = d.id WHERE l.id = ?`, id).
		Scan(&l.ID, &l.DeviceID, &l.DeviceName, &l.DeviceAssetCode, &l.BorrowerName, &l.BorrowerType,
			&l.LoanDate, &l.ExpectedReturnDate, &l.ActualReturnDate, &l.Quantity, &l.Status, &l.Purpose, &l.Notes)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *DeviceLoanRepository) GetLoanableDevices() ([]models.Device, error) {
	rows, err := r.db.Query(`SELECT id, asset_code, name, item_type, quantity_available, is_loanable FROM devices WHERE is_loanable = TRUE AND quantity_available > 0 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var d models.Device
		if rows.Scan(&d.ID, &d.AssetCode, &d.Name, &d.ItemType, &d.QuantityAvailable, &d.IsLoanable) == nil {
			devices = append(devices, d)
		}
	}
	return devices, nil
}

func (r *DeviceLoanRepository) Create(deviceID int, borrowerName, borrowerType string, loanDate time.Time, expectedReturnDate *time.Time, quantity int, purpose string) (int64, error) {
	result, err := r.db.Exec(`INSERT INTO device_loans (device_id, borrower_name, borrower_type, loan_date, expected_return_date, quantity, status, purpose) VALUES (?, ?, ?, ?, ?, ?, 'active', ?)`,
		deviceID, borrowerName, borrowerType, loanDate, expectedReturnDate, quantity, purpose)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *DeviceLoanRepository) Update(id int, borrowerName, borrowerType string, loanDate time.Time, expectedReturnDate, actualReturnDate *time.Time, status, purpose, notes string) error {
	_, err := r.db.Exec(`UPDATE device_loans SET borrower_name=?, borrower_type=?, loan_date=?, expected_return_date=?, actual_return_date=?, status=?, purpose=?, notes=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		borrowerName, borrowerType, loanDate, expectedReturnDate, actualReturnDate, status, purpose, notes, id)
	return err
}

func (r *DeviceLoanRepository) GetDeviceAndQuantity(id int) (deviceID, quantity int, err error) {
	err = r.db.QueryRow(`SELECT device_id, quantity FROM device_loans WHERE id = ?`, id).Scan(&deviceID, &quantity)
	return
}

func (r *DeviceLoanRepository) Delete(loanID int) error {
	_, err := r.db.Exec(`DELETE FROM device_loans WHERE id = ?`, loanID)
	return err
}
