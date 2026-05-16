package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
)

// ActivityLogService handles activity logging operations
type ActivityLogService struct {
	db      *database.DB
	logChan chan *models.ActivityLog
}

// NewActivityLogService creates a new activity log service
func NewActivityLogService(db *database.DB) *ActivityLogService {
	s := &ActivityLogService{
		db:      db,
		logChan: make(chan *models.ActivityLog, 64),
	}
	go s.logWriter()
	return s
}

func (s *ActivityLogService) logWriter() {
	for l := range s.logChan {
		if err := s.Log(l); err != nil {
			log.Printf("async log write: %v", err)
		}
	}
}

func (s *ActivityLogService) enqueueLog(al *models.ActivityLog) {
	select {
	case s.logChan <- al:
	default:
		log.Printf("async log channel full, dropping entry")
	}
}

// Log creates a new activity log entry (called by the background writer goroutine)
func (s *ActivityLogService) Log(al *models.ActivityLog) error {
	_, err := s.db.Exec(`
		INSERT INTO activity_logs (
			user_id, username, user_role, action, entity_type, entity_id,
			description, old_values, new_values, created_at, ip_address,
			user_agent, status, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, al.UserID, al.Username, al.UserRole, al.Action, al.EntityType,
		al.EntityID, al.Description, al.OldValues, al.NewValues,
		al.CreatedAt, al.IPAddress, al.UserAgent, al.Status, al.ErrorMessage)
	if err != nil { return fmt.Errorf("failed to insert activity log: %w", err) }
	return nil
}

type logParams struct {
	userID, entityID int
	username, role, action, entityType string
	oldValues, newValues interface{}
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
		CreatedAt: time.Now(), IPAddress: p.ipAddress, UserAgent: p.userAgent,
		Status: status, ErrorMessage: errText,
	})
	return nil
}

func (s *ActivityLogService) LogAction(userID int, username, role, action, entityType string, entityID int, oldValues, newValues interface{}, ipAddress, userAgent string, errorMsg ...string) error {
	errMsg := ""; if len(errorMsg) > 0 { errMsg = errorMsg[0] }
	return s.logAction(logParams{userID: userID, username: username, role: role, action: action, entityType: entityType, entityID: entityID, oldValues: oldValues, newValues: newValues, errMsg: errMsg, ipAddress: ipAddress, userAgent: userAgent})
}

func (s *ActivityLogService) LogCreate(userID int, username, role, entityType string, entityID int, newValues interface{}, ipAddress, userAgent string, errorMsg ...string) error {
	errMsg := ""; if len(errorMsg) > 0 { errMsg = errorMsg[0] }
	return s.logAction(logParams{userID: userID, username: username, role: role, action: "create", entityType: entityType, entityID: entityID, newValues: newValues, errMsg: errMsg, ipAddress: ipAddress, userAgent: userAgent})
}

func (s *ActivityLogService) LogUpdate(userID int, username, role, entityType string, entityID int, oldValues, newValues interface{}, ipAddress, userAgent string, errorMsg ...string) error {
	errMsg := ""; if len(errorMsg) > 0 { errMsg = errorMsg[0] }
	return s.logAction(logParams{userID: userID, username: username, role: role, action: "update", entityType: entityType, entityID: entityID, oldValues: oldValues, newValues: newValues, errMsg: errMsg, ipAddress: ipAddress, userAgent: userAgent})
}

func (s *ActivityLogService) LogDelete(userID int, username, role, entityType string, entityID int, oldValues interface{}, ipAddress, userAgent string, errorMsg ...string) error {
	errMsg := ""; if len(errorMsg) > 0 { errMsg = errorMsg[0] }
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
		CreatedAt: time.Now(), IPAddress: ipAddress, UserAgent: userAgent,
		Status: status, ErrorMessage: errorMsg,
	})
	return nil
}

// ActivityLogFilters represents filters for querying activity logs
type ActivityLogFilters struct {
	DateFrom   *time.Time
	DateTo     *time.Time
	Action     string
	EntityType string
	UserID     *int
	Username   string
	Status     string
	Limit      int
	Offset     int
}

// GetLogs retrieves activity logs with filters
func (s *ActivityLogService) GetLogs(filters ActivityLogFilters) ([]models.ActivityLog, int, error) {
	baseQuery := `
		SELECT id, user_id, username, user_role, action, entity_type, entity_id,
			description, old_values, new_values, created_at, ip_address,
			user_agent, status, error_message FROM activity_logs WHERE 1=1`
	countQ := "SELECT COUNT(*) FROM activity_logs WHERE 1=1"
	args := []interface{}{}

	addCond := func(cond string, val interface{}) {
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

	baseQuery += " ORDER BY created_at DESC"
	if filters.Limit > 0 { baseQuery += " LIMIT ?"; args = append(args, filters.Limit) }
	if filters.Offset > 0 { baseQuery += " OFFSET ?"; args = append(args, filters.Offset) }

	rows, err := s.db.Query(baseQuery, args...)
	if err != nil { return nil, 0, fmt.Errorf("failed to query logs: %w", err) }
	defer rows.Close()

	return scanLogs(rows, totalCount)
}

// SearchLogs searches activity logs by keyword
func (s *ActivityLogService) SearchLogs(keyword string, limit, offset int) ([]models.ActivityLog, int, error) {
	searchTerm := "%" + keyword + "%"
	var totalCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM activity_logs WHERE description LIKE ? OR username LIKE ? OR entity_type LIKE ?`,
		searchTerm, searchTerm, searchTerm).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	rows, err := s.db.Query(`SELECT id, user_id, username, user_role, action, entity_type, entity_id,
		description, old_values, new_values, created_at, ip_address,
		user_agent, status, error_message FROM activity_logs
		WHERE description LIKE ? OR username LIKE ? OR entity_type LIKE ?
		ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		searchTerm, searchTerm, searchTerm, limit, offset)
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
