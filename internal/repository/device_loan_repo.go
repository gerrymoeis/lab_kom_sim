package repository

import (
	"database/sql"
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
)

type DeviceLoanRepository struct {
	db *database.DB
}

func NewDeviceLoanRepository(db *database.DB) *DeviceLoanRepository {
	return &DeviceLoanRepository{db: db}
}

type DeviceLoanFilters struct {
	Status string
	Search string
	SortBy string
}

func (r *DeviceLoanRepository) List(filters DeviceLoanFilters) ([]DeviceLoanRow, error) {
	query := `SELECT l.id, l.device_id, d.name, d.asset_code, l.borrower_name, l.borrower_type,
		l.loan_date, l.expected_return_date, l.actual_return_date, l.quantity, l.status, l.purpose,
		CASE WHEN l.actual_return_date IS NOT NULL THEN 'returned'
			WHEN l.expected_return_date IS NOT NULL AND CURRENT_DATE > l.expected_return_date THEN 'overdue'
			ELSE 'active' END as computed_status
		FROM device_loans l JOIN devices d ON l.device_id = d.id WHERE 1=1`
	var args []interface{}

	if filters.Status != "" {
		query += ` AND CASE WHEN l.actual_return_date IS NOT NULL THEN 'returned'
			WHEN l.expected_return_date IS NOT NULL AND CURRENT_DATE > l.expected_return_date THEN 'overdue'
			ELSE 'active' END = ?`
		args = append(args, filters.Status)
	}
	if filters.Search != "" {
		query += ` AND (l.borrower_name LIKE ? OR d.name LIKE ? OR d.asset_code LIKE ?)`
		s := "%" + filters.Search + "%"
		args = append(args, s, s, s)
	}

	orderBy := "l.loan_date"
	if filters.SortBy == "borrower_name" {
		orderBy = "l.borrower_name"
	}
	query += ` ORDER BY ` + orderBy + ` DESC LIMIT 100`

	rows, err := r.db.Query(query, args...)
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
		l.loan_date, l.expected_return_date, l.actual_return_date, l.quantity, l.status, l.purpose, l.notes
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

func (r *DeviceLoanRepository) GetDeviceAndQuantity(loanID int) (deviceID, quantity int, err error) {
	err = r.db.QueryRow(`SELECT device_id, quantity FROM device_loans WHERE id = ?`, loanID).Scan(&deviceID, &quantity)
	return
}

func (r *DeviceLoanRepository) Create(deviceID int, borrowerName, borrowerType string, loanDate time.Time, expectedReturnDate *time.Time, quantity int, purpose string) (int64, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`UPDATE devices SET quantity_available = quantity_available - ? WHERE id = ? AND quantity_available >= ?`, quantity, deviceID, quantity)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, sql.ErrNoRows
	}

	result, err := tx.Exec(`INSERT INTO device_loans (device_id, borrower_name, borrower_type, loan_date, expected_return_date, quantity, status, purpose) VALUES (?, ?, ?, ?, ?, ?, 'active', ?)`,
		deviceID, borrowerName, borrowerType, loanDate, expectedReturnDate, quantity, purpose)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *DeviceLoanRepository) Update(id int, borrowerName, borrowerType string, loanDate time.Time, expectedReturnDate, actualReturnDate *time.Time, status, purpose, notes string) error {
	_, err := r.db.Exec(`UPDATE device_loans SET borrower_name=?, borrower_type=?, loan_date=?, expected_return_date=?, actual_return_date=?, status=?, purpose=?, notes=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		borrowerName, borrowerType, loanDate, expectedReturnDate, actualReturnDate, status, purpose, notes, id)
	return err
}

func (r *DeviceLoanRepository) Delete(loanID int) error {
	deviceID, quantity, err := r.GetDeviceAndQuantity(loanID)
	if err != nil {
		return err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE devices SET quantity_available = quantity_available + ? WHERE id = ?`, quantity, deviceID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`DELETE FROM device_loans WHERE id = ?`, loanID)
	if err != nil {
		return err
	}

	return tx.Commit()
}
