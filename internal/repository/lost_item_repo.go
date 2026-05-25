package repository

import (
	"database/sql"
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/search"
)

type LostItemRepository struct {
	db     DBTX
	search *search.Builder
}

func NewLostItemRepository(db *database.DB) *LostItemRepository {
	return &LostItemRepository{db: db, search: search.New(db)}
}

func (r *LostItemRepository) WithTx(tx *database.Tx) *LostItemRepository {
	return &LostItemRepository{db: tx, search: r.search}
}

type LostItemFilters struct {
	Status    string
	Search    string
	SortBy    string
	SortOrder string
}

func (r *LostItemRepository) List(filters LostItemFilters) ([]models.LostItem, error) {
	query := `SELECT id, device_id, item_name, item_description, reported_by, reported_date, last_seen_at, location_last_seen, status, owner_name, owner_class, owner_nim, returned_date, photo, created_at, updated_at FROM lost_items WHERE 1=1`
	var args []any
	if filters.Status != "" {
		query += ` AND status = ?`
		args = append(args, filters.Status)
	}
	if filters.Search != "" {
		sClause, sArgs := r.search.Where("lost_item", filters.Search)
		query += sClause
		args = append(args, sArgs...)
	}
	sortBy := "reported_date"
	switch filters.SortBy {
	case "item_name":
		sortBy = "item_name"
	case "status":
		sortBy = "status"
	}
	sortOrder := "DESC"
	if filters.SortOrder == "ASC" {
		sortOrder = "ASC"
	}
	query += ` ORDER BY ` + sortBy + ` ` + sortOrder

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.LostItem
	for rows.Next() {
		var li models.LostItem
		var deviceID sql.NullInt64
		var itemDesc, loc, ownerName, ownerClass, ownerNim, photo sql.NullString
		var lastSeenAt, returnedDate sql.NullTime
		if err := rows.Scan(&li.ID, &deviceID, &li.ItemName, &itemDesc,
			&li.ReportedBy, &li.ReportedDate, &lastSeenAt, &loc,
			&li.Status, &ownerName, &ownerClass, &ownerNim,
			&returnedDate, &photo, &li.CreatedAt, &li.UpdatedAt); err != nil {
			return nil, err
		}
		if deviceID.Valid {
			v := int(deviceID.Int64)
			li.DeviceID = &v
		}
		li.ItemDescription = itemDesc.String
		li.LocationLastSeen = loc.String
		li.OwnerName = ownerName.String
		li.OwnerClass = ownerClass.String
		li.OwnerNim = ownerNim.String
		li.Photo = photo.String
		if lastSeenAt.Valid {
			li.LastSeenAt = &lastSeenAt.Time
		}
		if returnedDate.Valid {
			li.ReturnedDate = &returnedDate.Time
		}
		items = append(items, li)
	}
	return items, nil
}

func (r *LostItemRepository) ListPaginated(filters LostItemFilters, page, pageSize int) ([]models.LostItem, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	countQuery := `SELECT COUNT(*) FROM lost_items WHERE 1=1`
	var args []any
	if filters.Status != "" {
		countQuery += ` AND status = ?`
		args = append(args, filters.Status)
	}
	if filters.Search != "" {
		sClause, sArgs := r.search.Where("lost_item", filters.Search)
		countQuery += sClause
		args = append(args, sArgs...)
	}

	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT id, device_id, item_name, item_description, reported_by, reported_date, last_seen_at, location_last_seen, status, owner_name, owner_class, owner_nim, returned_date, photo, created_at, updated_at FROM lost_items WHERE 1=1`
	args = []any{}
	if filters.Status != "" {
		query += ` AND status = ?`
		args = append(args, filters.Status)
	}
	if filters.Search != "" {
		sClause, sArgs := r.search.Where("lost_item", filters.Search)
		query += sClause
		args = append(args, sArgs...)
	}
	sortBy := "reported_date"
	switch filters.SortBy {
	case "item_name":
		sortBy = "item_name"
	case "status":
		sortBy = "status"
	}
	sortOrder := "DESC"
	if filters.SortOrder == "ASC" {
		sortOrder = "ASC"
	}
	query += ` ORDER BY ` + sortBy + ` ` + sortOrder + ` LIMIT ? OFFSET ?`
	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []models.LostItem
	for rows.Next() {
		var li models.LostItem
		var deviceID sql.NullInt64
		var itemDesc, loc, ownerName, ownerClass, ownerNim, photo sql.NullString
		var lastSeenAt, returnedDate sql.NullTime
		if err := rows.Scan(&li.ID, &deviceID, &li.ItemName, &itemDesc,
			&li.ReportedBy, &li.ReportedDate, &lastSeenAt, &loc,
			&li.Status, &ownerName, &ownerClass, &ownerNim,
			&returnedDate, &photo, &li.CreatedAt, &li.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if deviceID.Valid {
			v := int(deviceID.Int64)
			li.DeviceID = &v
		}
		li.ItemDescription = itemDesc.String
		li.LocationLastSeen = loc.String
		li.OwnerName = ownerName.String
		li.OwnerClass = ownerClass.String
		li.OwnerNim = ownerNim.String
		li.Photo = photo.String
		if lastSeenAt.Valid {
			li.LastSeenAt = &lastSeenAt.Time
		}
		if returnedDate.Valid {
			li.ReturnedDate = &returnedDate.Time
		}
		items = append(items, li)
	}
	return items, total, nil
}

func (r *LostItemRepository) GetByID(id int) (*models.LostItem, error) {
	var li models.LostItem
	var deviceID sql.NullInt64
	var itemDesc, loc, ownerName, ownerClass, ownerNim, photo sql.NullString
	var lastSeenAt, returnedDate sql.NullTime
	err := r.db.QueryRow(`SELECT id, device_id, item_name, item_description, reported_by, reported_date, last_seen_at, location_last_seen, status, owner_name, owner_class, owner_nim, returned_date, photo, created_at, updated_at FROM lost_items WHERE id = ?`, id).
		Scan(&li.ID, &deviceID, &li.ItemName, &itemDesc,
			&li.ReportedBy, &li.ReportedDate, &lastSeenAt, &loc,
			&li.Status, &ownerName, &ownerClass, &ownerNim,
			&returnedDate, &photo, &li.CreatedAt, &li.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if deviceID.Valid {
		v := int(deviceID.Int64)
		li.DeviceID = &v
	}
	li.ItemDescription = itemDesc.String
	li.LocationLastSeen = loc.String
	li.OwnerName = ownerName.String
	li.OwnerClass = ownerClass.String
	li.OwnerNim = ownerNim.String
	li.Photo = photo.String
	if lastSeenAt.Valid {
		li.LastSeenAt = &lastSeenAt.Time
	}
	if returnedDate.Valid {
		li.ReturnedDate = &returnedDate.Time
	}
	return &li, nil
}

func (r *LostItemRepository) Create(deviceID *int, itemName, itemDescription, reportedBy, reportedDate string, lastSeenAt *time.Time, locationLastSeen, status, photo string) (int64, error) {
	result, err := r.db.Exec(`INSERT INTO lost_items (device_id, item_name, item_description, reported_by, reported_date, last_seen_at, location_last_seen, status, photo) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		deviceID, itemName, itemDescription, reportedBy, reportedDate, lastSeenAt, locationLastSeen, status, photo)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

type UpdateLostItemParams struct {
	DeviceID         *int
	ItemName         string
	ItemDescription  string
	ReportedBy       string
	ReportedDate     string
	LastSeenAt       *time.Time
	LocationLastSeen string
	Status           string
	OwnerName        string
	OwnerClass       string
	OwnerNim         string
	ReturnedDate     *time.Time
	Photo            string
}

func (r *LostItemRepository) Update(id int, p UpdateLostItemParams) error {
	_, err := r.db.Exec(`UPDATE lost_items SET device_id=?, item_name=?, item_description=?, reported_by=?, reported_date=?, last_seen_at=?, location_last_seen=?, status=?, owner_name=?, owner_class=?, owner_nim=?, returned_date=?, photo=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		p.DeviceID, p.ItemName, p.ItemDescription, p.ReportedBy, p.ReportedDate, p.LastSeenAt, p.LocationLastSeen, p.Status, p.OwnerName, p.OwnerClass, p.OwnerNim, p.ReturnedDate, p.Photo, id)
	return err
}

func (r *LostItemRepository) Delete(id int) error {
	_, err := r.db.Exec(`DELETE FROM lost_items WHERE id = ?`, id)
	return err
}

func (r *LostItemRepository) GetPhoto(id int) (string, error) {
	var photo string
	err := r.db.QueryRow(`SELECT COALESCE(photo, '') FROM lost_items WHERE id = ?`, id).Scan(&photo)
	return photo, err
}

func (r *LostItemRepository) UpdatePhoto(id int, photo string) error {
	_, err := r.db.Exec(`UPDATE lost_items SET photo=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, photo, id)
	return err
}
