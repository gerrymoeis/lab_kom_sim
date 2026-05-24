package repository

import (
	"database/sql"
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
)

type LogbookRepository struct {
	db *database.DB
}

func NewLogbookRepository(db *database.DB) *LogbookRepository {
	return &LogbookRepository{db: db}
}

type LogbookFilters struct {
	StartDate    string
	EndDate      string
	Search       string
	SortBy       string
	SortOrder    string
	Page         int
	PageSize     int
	CursorID     int64
	CursorDate   string
	CursorTimeIn string
	Direction    string
}

func (r *LogbookRepository) List(filters LogbookFilters) ([]models.LogbookEntry, int, error) {
	where := ` WHERE 1=1`
	var args []any

	if filters.StartDate != "" {
		where += ` AND date >= ?`
		args = append(args, filters.StartDate)
	}
	if filters.EndDate != "" {
		where += ` AND date <= ?`
		args = append(args, filters.EndDate)
	}
	if filters.Search != "" {
		where += ` AND (student_name LIKE ? OR nim LIKE ?)`
		s := "%" + filters.Search + "%"
		args = append(args, s, s)
	}

	var total int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM logbook_entries`+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	sortBy := "date"
	if filters.SortBy == "student_name" {
		sortBy = "student_name"
	}
	sortOrder := "DESC"
	if filters.SortOrder == "ASC" {
		sortOrder = "ASC"
	}
	offset := (filters.Page - 1) * filters.PageSize
	if offset < 0 {
		offset = 0
	}

	query := `SELECT id, date, student_name, nim, time_in, time_out, purpose, source_file, created_at FROM logbook_entries` + where +
		` ORDER BY ` + sortBy + ` ` + sortOrder + `, time_in ` + sortOrder + ` LIMIT ? OFFSET ?`
	args = append(args, filters.PageSize, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []models.LogbookEntry
	for rows.Next() {
		var e models.LogbookEntry
		if err := rows.Scan(&e.ID, &e.Date, &e.StudentName, &e.NIM, &e.TimeIn, &e.TimeOut, &e.Purpose, &e.SourceFile, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}
	return entries, total, nil
}

func (r *LogbookRepository) ListCursor(filters LogbookFilters) ([]models.LogbookEntry, bool, error) {
	var args []any
	where := ` WHERE 1=1`
	from := ` FROM logbook_entries`

	if filters.StartDate != "" {
		where += ` AND date >= ?`
		args = append(args, filters.StartDate)
	}
	if filters.EndDate != "" {
		where += ` AND date <= ?`
		args = append(args, filters.EndDate)
	}
	if filters.Search != "" {
		s := "%" + filters.Search + "%"
		if r.db.IsPostgres() {
			where += ` AND (student_name LIKE ? OR nim LIKE ?)`
			args = append(args, s, s)
		} else {
			where += ` AND id IN (SELECT rowid FROM logbook_fts WHERE student_name LIKE ? OR nim LIKE ? OR purpose LIKE ?)`
			args = append(args, s, s, s)
		}
	}

	if filters.CursorID > 0 {
		if filters.Direction == "prev" {
			where += ` AND (date, time_in, id) < (?, ?, ?)`
		} else {
			where += ` AND (date, time_in, id) > (?, ?, ?)`
		}
		args = append(args, filters.CursorDate, filters.CursorTimeIn, filters.CursorID)
	}

	limit := filters.PageSize + 1
	orderDir := "ASC"
	if filters.Direction == "prev" {
		orderDir = "DESC"
	}

	query := `SELECT id, date, student_name, nim, time_in, time_out, purpose` + from + where +
		` ORDER BY date ` + orderDir + `, time_in ` + orderDir + `, id ` + orderDir + ` LIMIT ?`
	args = append(args, limit)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var entries []models.LogbookEntry
	for rows.Next() {
		var e models.LogbookEntry
		if err := rows.Scan(&e.ID, &e.Date, &e.StudentName, &e.NIM, &e.TimeIn, &e.TimeOut, &e.Purpose); err != nil {
			return nil, false, err
		}
		entries = append(entries, e)
	}

	hasMore := false
	if len(entries) > filters.PageSize {
		hasMore = true
		entries = entries[:filters.PageSize]
	}

	if filters.Direction == "prev" {
		for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
			entries[i], entries[j] = entries[j], entries[i]
		}
	}

	return entries, hasMore, nil
}

func (r *LogbookRepository) GetByID(id int) (*models.LogbookEntry, error) {
	var e models.LogbookEntry
	err := r.db.QueryRow(`SELECT id, date, student_name, nim, time_in, time_out, purpose, source_file, created_at, updated_at FROM logbook_entries WHERE id = ?`, id).
		Scan(&e.ID, &e.Date, &e.StudentName, &e.NIM, &e.TimeIn, &e.TimeOut, &e.Purpose, &e.SourceFile, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *LogbookRepository) GetDuplicateCheck(date time.Time, timeIn string) ([]models.LogbookEntry, error) {
	rows, err := r.db.Query(`SELECT date, student_name, nim, time_in FROM logbook_entries WHERE date = ? AND time_in = ?`, date, timeIn)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.LogbookEntry
	for rows.Next() {
		var e models.LogbookEntry
		if err := rows.Scan(&e.Date, &e.StudentName, &e.NIM, &e.TimeIn); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (r *LogbookRepository) GetOldValues(id int) (oldDate time.Time, oldName, oldNIM string, err error) {
	err = r.db.QueryRow(`SELECT date, student_name, nim FROM logbook_entries WHERE id = ?`, id).Scan(&oldDate, &oldName, &oldNIM)
	return
}

func (r *LogbookRepository) GetDeleteInfo(id int) (int, time.Time, string, string, error) {
	var eID int
	var date time.Time
	var name, nim string
	err := r.db.QueryRow(`SELECT id, date, student_name, nim FROM logbook_entries WHERE id = ?`, id).Scan(&eID, &date, &name, &nim)
	return eID, date, name, nim, err
}

func (r *LogbookRepository) GetMaxID() (int, error) {
	var id int
	err := r.db.QueryRow(`SELECT MAX(id) FROM logbook_entries`).Scan(&id)
	return id, err
}

func (r *LogbookRepository) Create(date time.Time, studentName, nim, timeIn, timeOut, purpose string) (sql.Result, error) {
	return r.db.Exec(`INSERT INTO logbook_entries (date, student_name, nim, time_in, time_out, purpose, source_file, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 'manual_entry', ?, ?)`,
		date, studentName, nim, timeIn, timeOut, purpose, time.Now().UTC(), time.Now().UTC())
}

func (r *LogbookRepository) Update(id int, date time.Time, studentName, nim, timeIn, timeOut, purpose string) error {
	_, err := r.db.Exec(`UPDATE logbook_entries SET date=?, student_name=?, nim=?, time_in=?, time_out=?, purpose=?, updated_at=? WHERE id=?`,
		date, studentName, nim, timeIn, timeOut, purpose, time.Now().UTC(), id)
	return err
}

func (r *LogbookRepository) Delete(id int) error {
	_, err := r.db.Exec(`DELETE FROM logbook_entries WHERE id = ?`, id)
	return err
}

// BulkImport inserts multiple logbook entries in a transaction using a prepared statement
func (r *LogbookRepository) BulkImport(entries []BulkEntry, sourceFile string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO logbook_entries (date, student_name, nim, time_in, time_out, purpose, source_file, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, e := range entries {
		if _, err := stmt.Exec(e.Date, e.StudentName, e.NIM, e.TimeIn, e.TimeOut, e.Purpose, sourceFile, now, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

type BulkEntry struct {
	Date        time.Time
	StudentName string
	NIM         string
	TimeIn      string
	TimeOut     string
	Purpose     string
}

type ExportFilters struct {
	StartDate string
	EndDate   string
	Search    string
}

func (r *LogbookRepository) Export(filters ExportFilters) ([]models.LogbookEntry, error) {
	query := `SELECT id, date, student_name, nim, time_in, time_out, purpose, source_file, created_at FROM logbook_entries WHERE 1=1`
	var args []any

	if filters.StartDate != "" {
		query += ` AND date >= ?`
		args = append(args, filters.StartDate)
	}
	if filters.EndDate != "" {
		query += ` AND date <= ?`
		args = append(args, filters.EndDate)
	}
	if filters.Search != "" {
		query += ` AND (student_name LIKE ? OR nim LIKE ?)`
		s := "%" + filters.Search + "%"
		args = append(args, s, s)
	}
	query += ` ORDER BY date DESC, time_in DESC`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.LogbookEntry
	for rows.Next() {
		var e models.LogbookEntry
		if err := rows.Scan(&e.ID, &e.Date, &e.StudentName, &e.NIM, &e.TimeIn, &e.TimeOut, &e.Purpose, &e.SourceFile, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}
