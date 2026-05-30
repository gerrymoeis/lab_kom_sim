package repository

import (
	"database/sql"
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/search"
)

type DeviceLoanRepository struct {
	db     DBTX
	search *search.Builder
}

func NewDeviceLoanRepository(db *database.DB) *DeviceLoanRepository {
	return &DeviceLoanRepository{db: db, search: search.New(db)}
}

func (r *DeviceLoanRepository) WithTx(tx *database.Tx) *DeviceLoanRepository {
	return &DeviceLoanRepository{db: tx, search: r.search}
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
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}

	loanClause, loanArgs := r.buildLoanClause(filters)

	var total int
	r.db.QueryRow(`SELECT COUNT(*) FROM device_loans l
		JOIN devices d ON d.id = l.device_id
		JOIN device_types dt ON dt.id = d.device_type_id
		JOIN categories c ON c.id = dt.category_id WHERE 1=1`+loanClause, loanArgs...).Scan(&total)

	query := `SELECT l.id, l.device_id, d.asset_code, dt.name, c.name,
		l.borrower_name, l.borrower_type, l.loan_date, l.return_date, l.actual_return_date,
		COALESCE(l.purpose,''), COALESCE(l.notes,''),
		(SELECT COUNT(*) FROM loan_extensions WHERE loan_id = l.id),
		CASE WHEN l.actual_return_date IS NOT NULL THEN 'returned'
			WHEN CURRENT_DATE > l.return_date THEN 'overdue'
			ELSE 'active' END as computed_status
		FROM device_loans l
		JOIN devices d ON d.id = l.device_id
		JOIN device_types dt ON dt.id = d.device_type_id
		JOIN categories c ON c.id = dt.category_id WHERE 1=1` + loanClause

	orderBy := "l.loan_date"
	if filters.SortBy == "borrower_name" {
		orderBy = "l.borrower_name"
	} else if filters.SortBy == "return_date" {
		orderBy = "l.return_date"
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
		if err := rows.Scan(&l.ID, &l.DeviceID, &l.DeviceAssetCode, &l.DeviceTypeName, &l.CategoryName,
			&l.BorrowerName, &l.BorrowerType, &l.LoanDate, &l.ReturnDate, &l.ActualReturnDate,
			&l.Purpose, &l.Notes, &l.ExtensionCount, &l.ComputedStatus); err != nil {
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
			WHEN CURRENT_DATE > l.return_date THEN 'overdue'
			ELSE 'active' END = ?`
		args = append(args, filters.Status)
	}
	if filters.Search != "" {
		sClause, sArgs := r.search.Where("device_loan", filters.Search)
		clause += sClause
		args = append(args, sArgs...)
	}
	return clause, args
}

func (r *DeviceLoanRepository) listWithQuery(filters DeviceLoanFilters, suffix string) ([]DeviceLoanRow, error) {
	_, loanArgs := r.buildLoanClause(filters)
	query := `SELECT l.id, l.device_id, d.asset_code, dt.name, c.name,
		l.borrower_name, l.borrower_type, l.loan_date, l.return_date, l.actual_return_date,
		COALESCE(l.purpose,''), COALESCE(l.notes,''),
		(SELECT COUNT(*) FROM loan_extensions WHERE loan_id = l.id),
		CASE WHEN l.actual_return_date IS NOT NULL THEN 'returned'
			WHEN CURRENT_DATE > l.return_date THEN 'overdue'
			ELSE 'active' END as computed_status
		FROM device_loans l
		JOIN devices d ON d.id = l.device_id
		JOIN device_types dt ON dt.id = d.device_type_id
		JOIN categories c ON c.id = dt.category_id WHERE 1=1`
	clause, _ := r.buildLoanClause(filters)
	query += clause

	orderBy := "l.loan_date"
	if filters.SortBy == "borrower_name" {
		orderBy = "l.borrower_name"
	} else if filters.SortBy == "return_date" {
		orderBy = "l.return_date"
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
		if err := rows.Scan(&l.ID, &l.DeviceID, &l.DeviceAssetCode, &l.DeviceTypeName, &l.CategoryName,
			&l.BorrowerName, &l.BorrowerType, &l.LoanDate, &l.ReturnDate, &l.ActualReturnDate,
			&l.Purpose, &l.Notes, &l.ExtensionCount, &l.ComputedStatus); err != nil {
			return nil, err
		}
		loans = append(loans, l)
	}
	return loans, nil
}

type DeviceLoanRow struct {
	models.DeviceLoan
	DeviceTypeName string
	CategoryName   string
	ComputedStatus string
}

func (r *DeviceLoanRepository) GetByID(id int) (*DeviceLoanRow, error) {
	var l DeviceLoanRow
	err := r.db.QueryRow(`SELECT l.id, l.device_id, d.asset_code, dt.name, c.name,
		l.borrower_name, l.borrower_type, l.loan_date, l.return_date, l.actual_return_date,
		COALESCE(l.purpose,''), COALESCE(l.notes,''),
		(SELECT COUNT(*) FROM loan_extensions WHERE loan_id = l.id),
		CASE WHEN l.actual_return_date IS NOT NULL THEN 'returned'
			WHEN CURRENT_DATE > l.return_date THEN 'overdue'
			ELSE 'active' END
		FROM device_loans l
		JOIN devices d ON d.id = l.device_id
		JOIN device_types dt ON dt.id = d.device_type_id
		JOIN categories c ON c.id = dt.category_id WHERE l.id = ?`, id).
		Scan(&l.ID, &l.DeviceID, &l.DeviceAssetCode, &l.DeviceTypeName, &l.CategoryName,
			&l.BorrowerName, &l.BorrowerType, &l.LoanDate, &l.ReturnDate, &l.ActualReturnDate,
			&l.Purpose, &l.Notes, &l.ExtensionCount, &l.ComputedStatus)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *DeviceLoanRepository) GetLoanableDevices() ([]models.Device, error) {
	rows, err := r.db.Query(`SELECT d.id, d.asset_code, d.serial_number, d.condition
		FROM devices d
		JOIN device_types dt ON dt.id = d.device_type_id
		WHERE dt.usage_type = 'loanable'
		AND d.id NOT IN (SELECT device_id FROM device_loans WHERE actual_return_date IS NULL)
		ORDER BY d.asset_code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var d models.Device
		var serial, cond sql.NullString
		if rows.Scan(&d.ID, &d.AssetCode, &serial, &cond) == nil {
			d.SerialNumber = valStr(serial)
			d.Condition = valStr(cond)
			devices = append(devices, d)
		}
	}
	return devices, nil
}

func (r *DeviceLoanRepository) GetActiveLoanByDevice(deviceID int) (*models.DeviceLoan, error) {
	var l models.DeviceLoan
	err := r.db.QueryRow(`SELECT id, device_id, borrower_name, borrower_type, loan_date, return_date,
		actual_return_date, COALESCE(purpose,''), COALESCE(notes,''), created_at, updated_at
		FROM device_loans WHERE device_id = ? AND actual_return_date IS NULL ORDER BY loan_date DESC LIMIT 1`, deviceID).
		Scan(&l.ID, &l.DeviceID, &l.BorrowerName, &l.BorrowerType, &l.LoanDate, &l.ReturnDate,
			&l.ActualReturnDate, &l.Purpose, &l.Notes, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *DeviceLoanRepository) Create(deviceID int, borrowerName, borrowerType string, loanDate time.Time, returnDate time.Time, purpose string) (int64, error) {
	result, err := r.db.Exec(`INSERT INTO device_loans (device_id, borrower_name, borrower_type, loan_date, return_date, purpose)
		VALUES (?, ?, ?, ?, ?, ?)`, deviceID, borrowerName, borrowerType, loanDate, returnDate, purpose)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *DeviceLoanRepository) Update(id int, borrowerName, borrowerType string, loanDate time.Time, returnDate, actualReturnDate *time.Time, purpose, notes string) error {
	_, err := r.db.Exec(`UPDATE device_loans SET borrower_name=?, borrower_type=?, loan_date=?,
		return_date=?, actual_return_date=?, purpose=?, notes=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		borrowerName, borrowerType, loanDate, returnDate, actualReturnDate, purpose, notes, id)
	return err
}

func (r *DeviceLoanRepository) ExtendReturnDate(id int, newReturnDate string) error {
	var oldReturnDate string
	if err := r.db.QueryRow("SELECT return_date FROM device_loans WHERE id = ?", id).Scan(&oldReturnDate); err != nil {
		return err
	}
	_, err := r.db.Exec("UPDATE device_loans SET return_date=?, updated_at=CURRENT_TIMESTAMP WHERE id=?", newReturnDate, id)
	return err
}

func (r *DeviceLoanRepository) Delete(loanID int) error {
	_, err := r.db.Exec("DELETE FROM device_loans WHERE id = ?", loanID)
	return err
}
