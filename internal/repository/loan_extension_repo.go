package repository

import (
	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
)

type LoanExtensionRepository struct {
	db DBTX
}

func NewLoanExtensionRepository(db *database.DB) *LoanExtensionRepository {
	return &LoanExtensionRepository{db: db}
}

func (r *LoanExtensionRepository) ListByLoanID(loanID int) ([]models.LoanExtension, error) {
	rows, err := r.db.Query(`SELECT id, loan_id, previous_return_date, new_return_date, extended_at
		FROM loan_extensions WHERE loan_id = ? ORDER BY extended_at`, loanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exts []models.LoanExtension
	for rows.Next() {
		var e models.LoanExtension
		if err := rows.Scan(&e.ID, &e.LoanID, &e.PreviousReturnDate, &e.NewReturnDate, &e.ExtendedAt); err != nil {
			return nil, err
		}
		exts = append(exts, e)
	}
	return exts, nil
}

func (r *LoanExtensionRepository) CountByLoanID(loanID int) (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM loan_extensions WHERE loan_id = ?", loanID).Scan(&count)
	return count, err
}

func (r *LoanExtensionRepository) Create(loanID int, prevReturnDate, newReturnDate string) (int64, error) {
	res, err := r.db.Exec("INSERT INTO loan_extensions (loan_id, previous_return_date, new_return_date) VALUES (?, ?, ?)",
		loanID, prevReturnDate, newReturnDate)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
