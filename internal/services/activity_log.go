package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
	"inventaris-lab-kom/internal/search"
)

// ActivityLogService handles activity logging operations
type ActivityLogService struct {
	db       *database.DB
	notifier CUDNotifier
	logChan  chan *models.ActivityLog
	stmt     *sql.Stmt
	close    chan struct{}
	search   *search.Builder
}

// NewActivityLogService creates a new activity log service
func NewActivityLogService(db *database.DB, notifier CUDNotifier) *ActivityLogService {
	stmt, err := db.Prepare(`
		INSERT INTO activity_logs (
			user_id, username, user_role, action, entity_type, entity_id,
			description, old_values, new_values, created_at, ip_address,
			user_agent, status, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		log.Printf("failed to prepare activity log stmt: %v", err)
	}
	s := &ActivityLogService{
		db:       db,
		notifier: notifier,
		logChan:  make(chan *models.ActivityLog, 4096),
		stmt:     stmt,
		close:    make(chan struct{}),
		search:   search.New(db),
	}
	go s.logWriter()
	return s
}

func (s *ActivityLogService) logWriter() {
	const batchSize = 100
	batch := make([]*models.ActivityLog, 0, batchSize)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 { return }
		if err := s.batchLog(batch); err != nil {
			log.Printf("batch log write: %v", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case l, ok := <-s.logChan:
			if !ok {
				flush(); return
			}
			batch = append(batch, l)
			if len(batch) >= batchSize { flush() }
		case <-ticker.C:
			flush()
		case <-s.close:
			flush(); return
		}
	}
}

func (s *ActivityLogService) batchLog(batch []*models.ActivityLog) error {
	tx, err := s.db.Begin()
	if err != nil { return err }
	defer tx.Rollback()

	for _, l := range batch {
		_, err := tx.Stmt(s.stmt).Exec(
			l.UserID, l.Username, l.UserRole, l.Action, l.EntityType,
			l.EntityID, l.Description, l.OldValues, l.NewValues,
			l.CreatedAt, l.IPAddress, l.UserAgent, l.Status, l.ErrorMessage,
		)
		if err != nil { return fmt.Errorf("failed to batch insert activity log: %w", err) }
	}
	return tx.Commit()
}

func (s *ActivityLogService) Close() {
	close(s.close)
	if s.stmt != nil {
		s.stmt.Close()
	}
}

// enqueueLog sends a log entry to the background writer
func (s *ActivityLogService) enqueueLog(al *models.ActivityLog) {
	select {
	case s.logChan <- al:
	default:
		log.Printf("async log channel full, dropping entry")
	}
}

type logParams struct {
	userID, entityID int
	username, role, action, entityType string
	oldValues, newValues any
	fileNewValues map[string]string
	errMsg string
	ipAddress, userAgent string
}

func (s *ActivityLogService) logAction(p logParams) error {
	status, errText := "success", p.errMsg
	if errText != "" { status = "failed" }

	var oldJSON, newJSON string
	if p.oldValues != nil { if b, e := json.Marshal(p.oldValues); e == nil { oldJSON = string(b) } }
	if p.newValues != nil { if b, e := json.Marshal(p.newValues); e == nil { newJSON = string(b) } }
	if p.fileNewValues != nil { if b, e := json.Marshal(p.fileNewValues); e == nil { newJSON = string(b) } }

	actionLabel := map[string]string{"create": "Created", "update": "Updated", "delete": "Deleted", "upload": "Uploaded"}
	desc := fmt.Sprintf("%s %s #%d", actionLabel[p.action], p.entityType, p.entityID)
	if p.action == "upload" && p.fileNewValues != nil {
		if ft, ok := p.fileNewValues["file_type"]; ok { desc = fmt.Sprintf("Uploaded %s for %s #%d: %s", ft, p.entityType, p.entityID, p.fileNewValues["filename"]) }
	}
	if errText != "" { desc = fmt.Sprintf("Failed to %s %s #%d: %s", p.action, p.entityType, p.entityID, errText) }

	s.enqueueLog(&models.ActivityLog{
		UserID: p.userID, Username: p.username, UserRole: p.role,
		Action: p.action, EntityType: p.entityType, EntityID: &p.entityID,
		Description: desc, OldValues: oldJSON, NewValues: newJSON,
		CreatedAt: time.Now().UTC(), IPAddress: p.ipAddress, UserAgent: p.userAgent,
		Status: status, ErrorMessage: errText,
	})
	return nil
}

func (s *ActivityLogService) LogAction(userID int, username, role, action, entityType string, entityID int, oldValues, newValues any, ipAddress, userAgent string, errorMsg ...string) error {
	errMsg := ""; if len(errorMsg) > 0 { errMsg = errorMsg[0] }
	return s.logAction(logParams{userID: userID, username: username, role: role, action: action, entityType: entityType, entityID: entityID, oldValues: oldValues, newValues: newValues, errMsg: errMsg, ipAddress: ipAddress, userAgent: userAgent})
}

func (s *ActivityLogService) notifyChange() {
	if s.notifier != nil {
		s.notifier.NotifyChange()
	}
}

func (s *ActivityLogService) LogCreate(userID int, username, role, entityType string, entityID int, newValues any, ipAddress, userAgent string, errorMsg ...string) error {
	errMsg := ""; if len(errorMsg) > 0 { errMsg = errorMsg[0] }
	s.notifyChange()
	return s.logAction(logParams{userID: userID, username: username, role: role, action: "create", entityType: entityType, entityID: entityID, newValues: newValues, errMsg: errMsg, ipAddress: ipAddress, userAgent: userAgent})
}

func (s *ActivityLogService) LogUpdate(userID int, username, role, entityType string, entityID int, oldValues, newValues any, ipAddress, userAgent string, errorMsg ...string) error {
	errMsg := ""; if len(errorMsg) > 0 { errMsg = errorMsg[0] }
	s.notifyChange()
	return s.logAction(logParams{userID: userID, username: username, role: role, action: "update", entityType: entityType, entityID: entityID, oldValues: oldValues, newValues: newValues, errMsg: errMsg, ipAddress: ipAddress, userAgent: userAgent})
}

func (s *ActivityLogService) LogDelete(userID int, username, role, entityType string, entityID int, oldValues any, ipAddress, userAgent string, errorMsg ...string) error {
	errMsg := ""; if len(errorMsg) > 0 { errMsg = errorMsg[0] }
	s.notifyChange()
	return s.logAction(logParams{userID: userID, username: username, role: role, action: "delete", entityType: entityType, entityID: entityID, oldValues: oldValues, errMsg: errMsg, ipAddress: ipAddress, userAgent: userAgent})
}

func (s *ActivityLogService) LogUpload(userID int, username, role, entityType string, entityID int, filename, fileType string, ipAddress, userAgent string, errorMsg ...string) error {
	errMsg := ""; if len(errorMsg) > 0 { errMsg = errorMsg[0] }
	return s.logAction(logParams{userID: userID, username: username, role: role, action: "upload", entityType: entityType, entityID: entityID, fileNewValues: map[string]string{"filename": filename, "file_type": fileType}, errMsg: errMsg, ipAddress: ipAddress, userAgent: userAgent})
}

func (s *ActivityLogService) LogAuth(userID int, username, role, action string, success bool, ipAddress, userAgent string, errorMsg string) error {
	status := "success"; if !success { status = "failed" }
	desc := fmt.Sprintf("User '%s' %s", username, action)
	if !success { desc += " (failed)" }
	s.enqueueLog(&models.ActivityLog{
		UserID: userID, Username: username, UserRole: role,
		Action: action, EntityType: "auth", EntityID: nil,
		Description: desc, OldValues: "", NewValues: "",
		CreatedAt: time.Now().UTC(), IPAddress: ipAddress, UserAgent: userAgent,
		Status: status, ErrorMessage: errorMsg,
	})
	return nil
}

// ActivityLogFilters represents filters for querying activity logs
type ActivityLogFilters struct {
	DateFrom        *time.Time
	DateTo          *time.Time
	Action          string
	EntityType      string
	UserID          *int
	Username        string
	Status          string
	SortBy          string
	Limit           int
	Offset          int
	CursorID        int64
	CursorCreatedAt time.Time
	Direction       string
}

// GetLogs retrieves activity logs with filters
func (s *ActivityLogService) GetAllUsernames() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT username FROM activity_logs ORDER BY username`)
	if err != nil { return nil, err }
	defer rows.Close()
	var usernames []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil { return nil, err }
		usernames = append(usernames, u)
	}
	return usernames, rows.Err()
}

func (s *ActivityLogService) GetAllEntityTypes() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT entity_type FROM activity_logs ORDER BY entity_type`)
	if err != nil { return nil, err }
	defer rows.Close()
	var types []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil { return nil, err }
		types = append(types, t)
	}
	return types, rows.Err()
}

func (s *ActivityLogService) GetLogs(filters ActivityLogFilters) ([]models.ActivityLog, int, error) {
	baseQuery := `
		SELECT id, user_id, username, user_role, action, entity_type, entity_id,
			description, old_values, new_values, created_at, ip_address,
			user_agent, status, error_message FROM activity_logs WHERE 1=1`
	countQ := "SELECT COUNT(*) FROM activity_logs WHERE 1=1"
	args := []any{}

	addCond := func(cond string, val any) {
		baseQuery += cond; countQ += cond; args = append(args, val)
	}

	if filters.DateFrom != nil { addCond(" AND created_at >= ?", filters.DateFrom) }
	if filters.DateTo != nil { addCond(" AND created_at <= ?", filters.DateTo) }
	if filters.Action != "" { addCond(" AND action = ?", filters.Action) }
	if filters.EntityType != "" { addCond(" AND entity_type = ?", filters.EntityType) }
	if filters.UserID != nil { addCond(" AND user_id = ?", *filters.UserID) }
	if filters.Username != "" { addCond(" AND username LIKE ?", "%"+filters.Username+"%") }
	if filters.Status != "" { addCond(" AND status = ?", filters.Status) }

	var totalCount int
	if err := s.db.QueryRow(countQ, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("failed to count logs: %w", err)
	}

	switch filters.SortBy {
	case "username":
		baseQuery += " ORDER BY username"
	case "action":
		baseQuery += " ORDER BY action"
	case "entity_type":
		baseQuery += " ORDER BY entity_type"
	default:
		baseQuery += " ORDER BY created_at DESC"
	}
	if filters.Limit > 0 { baseQuery += " LIMIT ?"; args = append(args, filters.Limit) }
	if filters.Offset > 0 { baseQuery += " OFFSET ?"; args = append(args, filters.Offset) }

	rows, err := s.db.Query(baseQuery, args...)
	if err != nil { return nil, 0, fmt.Errorf("failed to query logs: %w", err) }
	defer rows.Close()

	return scanLogs(rows, totalCount)
}

func (s *ActivityLogService) GetLogsCursor(filters ActivityLogFilters) ([]models.ActivityLog, bool, error) {
	query := `SELECT id, user_id, username, user_role, action, entity_type, entity_id,
		description, old_values, new_values, created_at, ip_address,
		user_agent, status, error_message FROM activity_logs WHERE 1=1`
	args := []any{}

	if filters.DateFrom != nil { query += " AND created_at >= ?"; args = append(args, filters.DateFrom) }
	if filters.DateTo != nil { query += " AND created_at <= ?"; args = append(args, filters.DateTo) }
	if filters.Action != "" { query += " AND action = ?"; args = append(args, filters.Action) }
	if filters.EntityType != "" { query += " AND entity_type = ?"; args = append(args, filters.EntityType) }
	if filters.UserID != nil { query += " AND user_id = ?"; args = append(args, *filters.UserID) }
	if filters.Username != "" { query += " AND username LIKE ?"; args = append(args, "%"+filters.Username+"%") }
	if filters.Status != "" { query += " AND status = ?"; args = append(args, filters.Status) }

	if filters.CursorID > 0 {
		if filters.Direction == "prev" {
			query += " AND (created_at, id) > (?, ?)"
		} else {
			query += " AND (created_at, id) < (?, ?)"
		}
		args = append(args, filters.CursorCreatedAt, filters.CursorID)
	}

	limit := filters.Limit + 1
	orderDir := "DESC"
	if filters.Direction == "prev" {
		orderDir = "ASC"
	}

	query += " ORDER BY created_at " + orderDir + ", id " + orderDir + " LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil { return nil, false, fmt.Errorf("failed to query logs: %w", err) }
	defer rows.Close()

	logs := []models.ActivityLog{}
	for rows.Next() {
		var l models.ActivityLog
		if err := rows.Scan(&l.ID, &l.UserID, &l.Username, &l.UserRole, &l.Action,
			&l.EntityType, &l.EntityID, &l.Description, &l.OldValues, &l.NewValues,
			&l.CreatedAt, &l.IPAddress, &l.UserAgent, &l.Status, &l.ErrorMessage); err != nil {
			return nil, false, fmt.Errorf("failed to scan log: %w", err)
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil { return nil, false, fmt.Errorf("rows error: %w", err) }

	hasMore := false
	if len(logs) > filters.Limit {
		hasMore = true
		logs = logs[:filters.Limit]
	}

	if filters.Direction == "prev" {
		for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
			logs[i], logs[j] = logs[j], logs[i]
		}
	}

	return logs, hasMore, nil
}

// SearchLogs searches activity logs by keyword
func (s *ActivityLogService) SearchLogs(keyword string, limit, offset int) ([]models.ActivityLog, int, error) {
	whereClause, whereArgs := s.search.Where("activity_log", keyword)

	var totalCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM activity_logs WHERE 1=1`+whereClause,
		whereArgs...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	allArgs := append(whereArgs, limit, offset)
	rows, err := s.db.Query(`SELECT id, user_id, username, user_role, action, entity_type, entity_id,
		description, old_values, new_values, created_at, ip_address,
		user_agent, status, error_message FROM activity_logs WHERE 1=1`+whereClause+`
		ORDER BY created_at DESC LIMIT ? OFFSET ?`, allArgs...)
	if err != nil { return nil, 0, fmt.Errorf("failed to search logs: %w", err) }
	defer rows.Close()

	return scanLogs(rows, totalCount)
}

func scanLogs(rows *sql.Rows, totalCount int) ([]models.ActivityLog, int, error) {
	logs := []models.ActivityLog{}
	for rows.Next() {
		var log models.ActivityLog
		if err := rows.Scan(&log.ID, &log.UserID, &log.Username, &log.UserRole, &log.Action,
			&log.EntityType, &log.EntityID, &log.Description, &log.OldValues, &log.NewValues,
			&log.CreatedAt, &log.IPAddress, &log.UserAgent, &log.Status, &log.ErrorMessage); err != nil {
			return nil, 0, fmt.Errorf("failed to scan log: %w", err)
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil { return nil, 0, fmt.Errorf("rows error: %w", err) }
	return logs, totalCount, nil
}
