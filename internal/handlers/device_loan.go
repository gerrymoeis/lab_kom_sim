package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"inventaris-lab-kom/internal/models"

	"github.com/gin-gonic/gin"
)

func (h *Handler) DeviceLoanList(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	search := c.Query("search")
	status := c.Query("status")
	sortBy := c.DefaultQuery("sort_by", "loan_date")
	sortOrder := c.DefaultQuery("sort_order", "DESC")

	query := `SELECT l.id, l.device_id, d.name, d.asset_code, l.borrower_name, l.borrower_type,
		l.loan_date, l.expected_return_date, l.actual_return_date, l.quantity, l.status, l.purpose,
		CASE WHEN l.actual_return_date IS NOT NULL THEN 'returned'
			WHEN l.expected_return_date IS NOT NULL AND CURRENT_DATE > l.expected_return_date THEN 'overdue'
			ELSE 'active' END as computed_status
		FROM device_loans l JOIN devices d ON l.device_id = d.id WHERE 1=1`
	var args []interface{}

	if search != "" {
		query += ` AND (l.borrower_name LIKE ? OR d.name LIKE ? OR d.asset_code LIKE ?)`
		s := "%" + search + "%"
		args = append(args, s, s, s)
	}
	if status != "" {
		query += ` AND (CASE WHEN l.actual_return_date IS NOT NULL THEN 'returned'
			WHEN l.expected_return_date IS NOT NULL AND CURRENT_DATE > l.expected_return_date THEN 'overdue'
			ELSE 'active' END) = ?`
		args = append(args, status)
	}

	validSort := map[string]bool{"loan_date": true, "borrower_name": true, "status": true}
	if !validSort[sortBy] { sortBy = "loan_date" }
	if sortOrder != "ASC" { sortOrder = "DESC" }
	query += fmt.Sprintf(" ORDER BY l.%s %s", sortBy, sortOrder)

	rows, err := h.db.Query(query, args...)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data peminjaman")
		return
	}
	defer rows.Close()

	type LoanWithDevice struct {
		models.DeviceLoan
		DeviceName, AssetCode, ComputedStatus string
	}
	var loans []LoanWithDevice
	for rows.Next() {
		var l LoanWithDevice
		var expectedReturn, actualReturn sql.NullTime
		if rows.Scan(&l.ID, &l.DeviceID, &l.DeviceName, &l.AssetCode, &l.BorrowerName, &l.BorrowerType,
			&l.LoanDate, &expectedReturn, &actualReturn, &l.Quantity, &l.Status, &l.Purpose, &l.ComputedStatus) != nil {
			continue
		}
		if expectedReturn.Valid { l.ExpectedReturnDate = &expectedReturn.Time }
		if actualReturn.Valid { l.ActualReturnDate = &actualReturn.Time }
		loans = append(loans, l)
	}

	c.HTML(http.StatusOK, "device_loan/list.html", gin.H{
		"title": "Peminjaman Perangkat", "currentPage": "devices",
		"username": username, "role": role, "loans": loans,
		"filters": gin.H{"search": search, "status": status, "sort_by": sortBy, "sort_order": sortOrder},
	})
}

func (h *Handler) DeviceLoanCreatePage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	rows, err := h.db.Query(`SELECT id, asset_code, name, item_type, quantity_available, is_loanable FROM devices WHERE is_loanable = TRUE AND quantity_available > 0 ORDER BY name`)
	if err != nil {
		h.errHTML(c, "Gagal mengambil data perangkat")
		return
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var d models.Device
		if rows.Scan(&d.ID, &d.AssetCode, &d.Name, &d.ItemType, &d.QuantityAvailable, &d.IsLoanable) == nil {
			devices = append(devices, d)
		}
	}

	c.HTML(http.StatusOK, "device_loan/create.html", gin.H{
		"title": "Pinjam Perangkat", "currentPage": "devices",
		"username": username, "role": role, "devices": devices,
		"deviceID": c.Query("device_id"),
	})
}

func (h *Handler) DeviceLoanCreate(c *gin.Context) {
	deviceID := c.PostForm("device_id")
	borrowerName := c.PostForm("borrower_name")
	borrowerType := c.PostForm("borrower_type")
	loanDateStr := c.PostForm("loan_date")
	expectedReturnDateStr := c.PostForm("expected_return_date")
	quantityStr := c.PostForm("quantity")
	purpose := c.PostForm("purpose")

	quantity, _ := strconv.Atoi(quantityStr)
	if deviceID == "" || borrowerName == "" || loanDateStr == "" || quantity <= 0 {
		h.errHTML(c, "Perangkat, nama peminjam, tanggal, dan jumlah harus diisi")
		return
	}

	loanDate, _ := time.Parse("2006-01-02", loanDateStr)
	var expectedReturnDate *time.Time
	if expectedReturnDateStr != "" {
		if t, err := time.Parse("2006-01-02", expectedReturnDateStr); err == nil {
			expectedReturnDate = &t
		}
	}

	var qtyAvail int
	h.db.QueryRow(`SELECT quantity_available FROM devices WHERE id = ?`, deviceID).Scan(&qtyAvail)
	if qtyAvail < quantity {
		h.errHTML(c, fmt.Sprintf("Stok tidak cukup. Tersedia: %d", qtyAvail))
		return
	}

	tx, _ := h.db.Begin()
	defer tx.Rollback()

	result, err := tx.Exec(`INSERT INTO device_loans (device_id, borrower_name, borrower_type, loan_date, expected_return_date, quantity, status, purpose) VALUES (?, ?, ?, ?, ?, ?, 'active', ?)`,
		deviceID, borrowerName, borrowerType, loanDate, expectedReturnDate, quantity, purpose)
	if err != nil {
		h.errHTML(c, "Gagal menyimpan data peminjaman")
		return
	}

	tx.Exec(`UPDATE devices SET quantity_available = quantity_available - ? WHERE id = ?`, quantity, deviceID)
	tx.Commit()

	loanID, _ := result.LastInsertId()
	h.logCreate(c, "device_loan", int(loanID), map[string]interface{}{
		"device_id": deviceID, "borrower_name": borrowerName, "quantity": quantity,
	})
	c.Redirect(http.StatusFound, "/devices?tab=loans")
}

func (h *Handler) DeviceLoanEditPage(c *gin.Context) {
	_, username, role, ok := h.user(c)
	if !ok { return }

	id := c.Param("id")
	var loan models.DeviceLoan
	var deviceName, assetCode string
	var purpose, notes, borrowerType sql.NullString
	var expectedReturn, actualReturn sql.NullTime
	err := h.db.QueryRow(`SELECT l.id, l.device_id, d.name, d.asset_code, l.borrower_name, l.borrower_type,
		l.loan_date, l.expected_return_date, l.actual_return_date, l.quantity, l.status, l.purpose, l.notes
		FROM device_loans l JOIN devices d ON l.device_id = d.id WHERE l.id = ?`, id).
		Scan(&loan.ID, &loan.DeviceID, &deviceName, &assetCode, &loan.BorrowerName, &borrowerType,
			&loan.LoanDate, &expectedReturn, &actualReturn, &loan.Quantity, &loan.Status, &purpose, &notes)
	if err != nil {
		h.errHTML(c, "Peminjaman tidak ditemukan")
		return
	}
	if purpose.Valid { loan.Purpose = purpose.String }
	if notes.Valid { loan.Notes = notes.String }
	if borrowerType.Valid { loan.BorrowerType = borrowerType.String }
	if expectedReturn.Valid { loan.ExpectedReturnDate = &expectedReturn.Time }
	if actualReturn.Valid { loan.ActualReturnDate = &actualReturn.Time }

	c.HTML(http.StatusOK, "device_loan/edit.html", gin.H{
		"title": "Edit Peminjaman", "currentPage": "devices",
		"username": username, "role": role,
		"loan": loan, "deviceName": deviceName, "assetCode": assetCode,
	})
}

func (h *Handler) DeviceLoanEdit(c *gin.Context) {
	id := c.Param("id")
	borrowerName := c.PostForm("borrower_name")
	borrowerType := c.PostForm("borrower_type")
	loanDateStr := c.PostForm("loan_date")
	expectedReturnDateStr := c.PostForm("expected_return_date")
	actualReturnDateStr := c.PostForm("actual_return_date")
	status := c.PostForm("status")
	purpose := c.PostForm("purpose")
	notes := c.PostForm("notes")

	loanDate, _ := time.Parse("2006-01-02", loanDateStr)
	var expectedReturnDate, actualReturnDate *time.Time
	if expectedReturnDateStr != "" {
		if t, err := time.Parse("2006-01-02", expectedReturnDateStr); err == nil { expectedReturnDate = &t }
	}
	if actualReturnDateStr != "" {
		if t, err := time.Parse("2006-01-02", actualReturnDateStr); err == nil { actualReturnDate = &t }
	}

	_, err := h.db.Exec(`UPDATE device_loans SET borrower_name=?, borrower_type=?, loan_date=?, expected_return_date=?, actual_return_date=?, status=?, purpose=?, notes=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		borrowerName, borrowerType, loanDate, expectedReturnDate, actualReturnDate, status, purpose, notes, id)
	if err != nil {
		h.logUpdateError(c, "device_loan", 0, map[string]interface{}{"id": id}, err.Error())
		h.errHTML(c, "Gagal mengupdate peminjaman")
		return
	}

	h.logUpdate(c, "device_loan", 0,
		map[string]interface{}{"id": id},
		map[string]interface{}{"borrower_name": borrowerName, "status": status},
	)
	c.Redirect(http.StatusFound, "/devices?tab=loans")
}

func (h *Handler) DeviceLoanDelete(c *gin.Context) {
	id := c.Param("id")

	var deviceID, quantity int
	h.db.QueryRow(`SELECT device_id, quantity FROM device_loans WHERE id = ?`, id).Scan(&deviceID, &quantity)

	tx, _ := h.db.Begin()
	defer tx.Rollback()

	tx.Exec(`UPDATE devices SET quantity_available = quantity_available + ? WHERE id = ?`, quantity, deviceID)
	tx.Exec(`DELETE FROM device_loans WHERE id = ?`, id)
	tx.Commit()

	h.logDelete(c, "device_loan", 0, map[string]interface{}{"id": id})
	c.Redirect(http.StatusFound, "/devices?tab=loans")
}
