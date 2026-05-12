package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"inventaris-lab-kom/internal/middleware"
	"inventaris-lab-kom/internal/models"

	"github.com/gin-gonic/gin"
)

// DeviceLoanList renders list of device loans with filter/sort/search
func (h *Handler) DeviceLoanList(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Parse filters
	search := c.Query("search")
	status := c.Query("status")
	sortBy := c.DefaultQuery("sort_by", "loan_date")
	sortOrder := c.DefaultQuery("sort_order", "DESC")

	// Build query with JOIN to get device name and computed status
	query := `
		SELECT l.id, l.device_id, d.name as device_name, d.asset_code, l.borrower_name, l.borrower_type,
		       l.loan_date, l.expected_return_date, l.actual_return_date, l.quantity, l.status, l.purpose,
		       CASE 
		         WHEN l.actual_return_date IS NOT NULL THEN 'returned'
		         WHEN l.expected_return_date IS NOT NULL AND CURRENT_DATE > l.expected_return_date THEN 'overdue'
		         ELSE 'active'
		       END as computed_status
		FROM device_loans l
		JOIN devices d ON l.device_id = d.id
		WHERE 1=1
	`
	args := []interface{}{}

	if search != "" {
		query += ` AND (l.borrower_name LIKE ? OR d.name LIKE ? OR d.asset_code LIKE ?)`
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	if status != "" {
		// Filter by computed status
		query += ` AND (CASE 
		              WHEN l.actual_return_date IS NOT NULL THEN 'returned'
		              WHEN l.expected_return_date IS NOT NULL AND CURRENT_DATE > l.expected_return_date THEN 'overdue'
		              ELSE 'active'
		            END) = ?`
		args = append(args, status)
	}

	// Sorting
	validSortColumns := map[string]bool{
		"loan_date": true, "borrower_name": true, "status": true,
	}
	if !validSortColumns[sortBy] {
		sortBy = "loan_date"
	}
	if sortOrder != "ASC" && sortOrder != "DESC" {
		sortOrder = "DESC"
	}
	query += fmt.Sprintf(" ORDER BY l.%s %s", sortBy, sortOrder)

	rows, err := h.db.Query(query, args...)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data peminjaman",
		})
		return
	}
	defer rows.Close()

	type LoanWithDevice struct {
		models.DeviceLoan
		DeviceName      string
		AssetCode       string
		ComputedStatus  string
	}

	var loans []LoanWithDevice
	for rows.Next() {
		var loan LoanWithDevice
		err := rows.Scan(&loan.ID, &loan.DeviceID, &loan.DeviceName, &loan.AssetCode, &loan.BorrowerName,
			&loan.BorrowerType, &loan.LoanDate, &loan.ExpectedReturnDate, &loan.ActualReturnDate,
			&loan.Quantity, &loan.Status, &loan.Purpose, &loan.ComputedStatus)
		if err != nil {
			continue
		}
		loans = append(loans, loan)
	}

	c.HTML(http.StatusOK, "device_loan/list.html", gin.H{
		"title":       "Peminjaman Perangkat - Sistem Inventaris Lab",
		"currentPage": "devices",
		"username":    username,
		"role":        role,
		"loans":       loans,
		"filters": gin.H{
			"search":     search,
			"status":     status,
			"sort_by":    sortBy,
			"sort_order": sortOrder,
		},
	})
}

// DeviceLoanCreatePage renders device loan creation form
func (h *Handler) DeviceLoanCreatePage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	deviceID := c.Query("device_id")

	// Get loanable devices
	rows, err := h.db.Query(`
		SELECT id, asset_code, name, item_type, quantity_available, is_loanable
		FROM devices
		WHERE is_loanable = 1 AND quantity_available > 0
		ORDER BY name
	`)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data perangkat",
		})
		return
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var d models.Device
		rows.Scan(&d.ID, &d.AssetCode, &d.Name, &d.ItemType, &d.QuantityAvailable, &d.IsLoanable)
		devices = append(devices, d)
	}

	c.HTML(http.StatusOK, "device_loan/create.html", gin.H{
		"title":       "Pinjam Perangkat - Sistem Inventaris Lab",
		"currentPage": "devices",
		"username":    username,
		"role":        role,
		"devices":     devices,
		"deviceID":    deviceID,
	})
}

// DeviceLoanCreate handles device loan creation
func (h *Handler) DeviceLoanCreate(c *gin.Context) {
	deviceID := c.PostForm("device_id")
	borrowerName := c.PostForm("borrower_name")
	borrowerType := c.PostForm("borrower_type")
	loanDateStr := c.PostForm("loan_date")
	expectedReturnDateStr := c.PostForm("expected_return_date")
	quantityStr := c.PostForm("quantity")
	purpose := c.PostForm("purpose")

	// Validate
	if deviceID == "" || borrowerName == "" || loanDateStr == "" || quantityStr == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "Error",
			"message": "Perangkat, nama peminjam, tanggal, dan jumlah harus diisi",
		})
		return
	}

	quantity, _ := strconv.Atoi(quantityStr)
	if quantity <= 0 {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "Error",
			"message": "Jumlah harus lebih dari 0",
		})
		return
	}

	// Check available quantity
	var quantityAvailable int
	err := h.db.QueryRow(`SELECT quantity_available FROM devices WHERE id = ?`, deviceID).Scan(&quantityAvailable)
	if err != nil || quantityAvailable < quantity {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "Error",
			"message": fmt.Sprintf("Stok tidak cukup. Tersedia: %d", quantityAvailable),
		})
		return
	}

	// Parse dates
	loanDate, _ := time.Parse("2006-01-02", loanDateStr)
	var expectedReturnDate *time.Time
	if expectedReturnDateStr != "" {
		parsed, _ := time.Parse("2006-01-02", expectedReturnDateStr)
		expectedReturnDate = &parsed
	}

	// Begin transaction
	tx, err := h.db.Begin()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal memulai transaksi",
		})
		return
	}
	defer tx.Rollback()

	// Insert loan
	result, err := tx.Exec(`
		INSERT INTO device_loans (device_id, borrower_name, borrower_type, loan_date, expected_return_date, quantity, status, purpose)
		VALUES (?, ?, ?, ?, ?, ?, 'active', ?)
	`, deviceID, borrowerName, borrowerType, loanDate, expectedReturnDate, quantity, purpose)

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal menyimpan data peminjaman",
		})
		return
	}

	// Update device quantity
	_, err = tx.Exec(`UPDATE devices SET quantity_available = quantity_available - ? WHERE id = ?`, quantity, deviceID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengupdate stok perangkat",
		})
		return
	}

	if err := tx.Commit(); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal menyimpan data",
		})
		return
	}

	// Log
	loanID, _ := result.LastInsertId()
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		h.activityLogService.LogCreate(
			userID, username, role,
			"device_loan", int(loanID),
			map[string]interface{}{
				"device_id":     deviceID,
				"borrower_name": borrowerName,
				"quantity":      quantity,
			},
			ipAddress, userAgent,
		)
	}

	c.Redirect(http.StatusFound, "/devices?tab=loans")
}

// DeviceLoanEditPage renders device loan edit form
func (h *Handler) DeviceLoanEditPage(c *gin.Context) {
	_, username, role, ok := middleware.GetCurrentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	id := c.Param("id")

	var loan models.DeviceLoan
	var deviceName, assetCode, computedStatus string
	var purpose, notes sql.NullString
	err := h.db.QueryRow(`
		SELECT l.id, l.device_id, d.name, d.asset_code, l.borrower_name, l.borrower_type,
		       l.loan_date, l.expected_return_date, l.actual_return_date, l.quantity, l.status, l.purpose, l.notes,
		       CASE 
		         WHEN l.actual_return_date IS NOT NULL THEN 'returned'
		         WHEN l.expected_return_date IS NOT NULL AND CURRENT_DATE > l.expected_return_date THEN 'overdue'
		         ELSE 'active'
		       END as computed_status
		FROM device_loans l
		JOIN devices d ON l.device_id = d.id
		WHERE l.id = ?
	`, id).Scan(&loan.ID, &loan.DeviceID, &deviceName, &assetCode, &loan.BorrowerName, &loan.BorrowerType,
		&loan.LoanDate, &loan.ExpectedReturnDate, &loan.ActualReturnDate, &loan.Quantity, &loan.Status, &purpose, &notes, &computedStatus)
	
	// Convert NullString to string
	if purpose.Valid {
		loan.Purpose = purpose.String
	}
	if notes.Valid {
		loan.Notes = notes.String
	}

	if err == sql.ErrNoRows {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Peminjaman Tidak Ditemukan",
			"message": "Data peminjaman tidak ditemukan",
		})
		return
	}

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data peminjaman",
		})
		return
	}

	c.HTML(http.StatusOK, "device_loan/edit.html", gin.H{
		"title":          "Edit Peminjaman - Sistem Inventaris Lab",
		"currentPage":    "devices",
		"username":       username,
		"role":           role,
		"loan":           loan,
		"deviceName":     deviceName,
		"assetCode":      assetCode,
		"computedStatus": computedStatus,
	})
}

// DeviceLoanEdit handles device loan update
func (h *Handler) DeviceLoanEdit(c *gin.Context) {
	id := c.Param("id")
	actualReturnDateStr := c.PostForm("actual_return_date")
	status := c.PostForm("status")
	notes := c.PostForm("notes")

	// Get current loan data
	var currentStatus string
	var deviceID, quantity int
	err := h.db.QueryRow(`SELECT device_id, quantity, status FROM device_loans WHERE id = ?`, id).Scan(&deviceID, &quantity, &currentStatus)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengambil data peminjaman",
		})
		return
	}

	// Parse return date
	var actualReturnDate *time.Time
	if actualReturnDateStr != "" {
		parsed, _ := time.Parse("2006-01-02", actualReturnDateStr)
		actualReturnDate = &parsed
	}

	// Begin transaction
	tx, err := h.db.Begin()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal memulai transaksi",
		})
		return
	}
	defer tx.Rollback()

	// Update loan
	_, err = tx.Exec(`
		UPDATE device_loans 
		SET actual_return_date = ?, status = ?, notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, actualReturnDate, status, notes, id)

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal mengupdate data peminjaman",
		})
		return
	}

	// If status changed from active to returned, restore quantity
	if currentStatus == "active" && status == "returned" {
		_, err = tx.Exec(`UPDATE devices SET quantity_available = quantity_available + ? WHERE id = ?`, quantity, deviceID)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"title":   "Error",
				"message": "Gagal mengupdate stok perangkat",
			})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Error",
			"message": "Gagal menyimpan data",
		})
		return
	}

	// Log
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		loanIDInt, _ := strconv.Atoi(id)
		h.activityLogService.LogUpdate(
			userID, username, role,
			"device_loan", loanIDInt,
			map[string]interface{}{"status": currentStatus},
			map[string]interface{}{"status": status},
			ipAddress, userAgent,
		)
	}

	c.Redirect(http.StatusFound, "/devices?tab=loans")
}

// DeviceLoanDelete handles device loan deletion
func (h *Handler) DeviceLoanDelete(c *gin.Context) {
	id := c.Param("id")

	// Get loan data
	var loanID, deviceID, quantity int
	var status, borrowerName string
	err := h.db.QueryRow(`SELECT id, device_id, quantity, status, borrower_name FROM device_loans WHERE id = ?`, id).Scan(&loanID, &deviceID, &quantity, &status, &borrowerName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengambil data peminjaman",
		})
		return
	}

	// Begin transaction
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal memulai transaksi",
		})
		return
	}
	defer tx.Rollback()

	// If loan is active, restore quantity
	if status == "active" {
		_, err = tx.Exec(`UPDATE devices SET quantity_available = quantity_available + ? WHERE id = ?`, quantity, deviceID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Gagal mengupdate stok perangkat",
			})
			return
		}
	}

	// Delete loan
	_, err = tx.Exec("DELETE FROM device_loans WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menghapus data peminjaman",
		})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal menyimpan data",
		})
		return
	}

	// Log
	userID, username, role, ok := middleware.GetCurrentUser(c)
	if ok {
		ipAddress, userAgent := getRequestContext(c)
		h.activityLogService.LogDelete(
			userID, username, role,
			"device_loan", loanID,
			map[string]interface{}{
				"borrower_name": borrowerName,
				"quantity":      quantity,
			},
			ipAddress, userAgent,
		)
	}

	c.Redirect(http.StatusFound, "/devices?tab=loans")
}
