package services

import (
	"encoding/json"
	"fmt"
	"time"

	"inventaris-lab-kom/internal/database"
	"inventaris-lab-kom/internal/models"
)

// ActivityLogService handles activity logging operations
type ActivityLogService struct {
	db *database.DB
}

// NewActivityLogService creates a new activity log service
func NewActivityLogService(db *database.DB) *ActivityLogService {
	return &ActivityLogService{db: db}
}

// Log creates a new activity log entry
func (s *ActivityLogService) Log(log *models.ActivityLog) error {
	_, err := s.db.Exec(`
		INSERT INTO activity_logs (
			user_id, username, user_role, action, entity_type, entity_id,
			description, old_values, new_values, created_at, ip_address,
			user_agent, status, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, log.UserID, log.Username, log.UserRole, log.Action, log.EntityType,
		log.EntityID, log.Description, log.OldValues, log.NewValues,
		log.CreatedAt, log.IPAddress, log.UserAgent, log.Status, log.ErrorMessage)
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

	return s.Log(&models.ActivityLog{
		UserID: p.userID, Username: p.username, UserRole: p.role,
		Action: p.action, EntityType: p.entityType, EntityID: &p.entityID,
		Description: desc, OldValues: oldJSON, NewValues: newJSON,
		CreatedAt: time.Now(), IPAddress: p.ipAddress, UserAgent: p.userAgent,
		Status: status, ErrorMessage: errText,
	})
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
	return s.Log(&models.ActivityLog{
		UserID: userID, Username: username, UserRole: role,
		Action: action, EntityType: "auth", EntityID: nil,
		Description: desc, OldValues: "", NewValues: "",
		CreatedAt: time.Now(), IPAddress: ipAddress, UserAgent: userAgent,
		Status: status, ErrorMessage: errorMsg,
	})
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
	// Build query with filters
	query := `
		SELECT 
			id, user_id, username, user_role, action, entity_type, entity_id,
			description, old_values, new_values, created_at, ip_address,
			user_agent, status, error_message
		FROM activity_logs
		WHERE 1=1
	`

	countQuery := "SELECT COUNT(*) FROM activity_logs WHERE 1=1"

	args := []interface{}{}
	countArgs := []interface{}{}

	// Apply filters
	if filters.DateFrom != nil {
		query += " AND created_at >= ?"
		countQuery += " AND created_at >= ?"
		args = append(args, filters.DateFrom)
		countArgs = append(countArgs, filters.DateFrom)
	}

	if filters.DateTo != nil {
		query += " AND created_at <= ?"
		countQuery += " AND created_at <= ?"
		args = append(args, filters.DateTo)
		countArgs = append(countArgs, filters.DateTo)
	}

	if filters.Action != "" {
		query += " AND action = ?"
		countQuery += " AND action = ?"
		args = append(args, filters.Action)
		countArgs = append(countArgs, filters.Action)
	}

	if filters.EntityType != "" {
		query += " AND entity_type = ?"
		countQuery += " AND entity_type = ?"
		args = append(args, filters.EntityType)
		countArgs = append(countArgs, filters.EntityType)
	}

	if filters.UserID != nil {
		query += " AND user_id = ?"
		countQuery += " AND user_id = ?"
		args = append(args, *filters.UserID)
		countArgs = append(countArgs, *filters.UserID)
	}

	if filters.Username != "" {
		query += " AND username LIKE ?"
		countQuery += " AND username LIKE ?"
		searchTerm := "%" + filters.Username + "%"
		args = append(args, searchTerm)
		countArgs = append(countArgs, searchTerm)
	}

	if filters.Status != "" {
		query += " AND status = ?"
		countQuery += " AND status = ?"
		args = append(args, filters.Status)
		countArgs = append(countArgs, filters.Status)
	}

	// Get total count
	var totalCount int
	err := s.db.QueryRow(countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count logs: %w", err)
	}

	// Order by created_at DESC (newest first)
	query += " ORDER BY created_at DESC"

	// Apply pagination
	if filters.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filters.Limit)
	}

	if filters.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filters.Offset)
	}

	// Execute query
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	logs := []models.ActivityLog{}

	for rows.Next() {
		var log models.ActivityLog
		err := rows.Scan(
			&log.ID,
			&log.UserID,
			&log.Username,
			&log.UserRole,
			&log.Action,
			&log.EntityType,
			&log.EntityID,
			&log.Description,
			&log.OldValues,
			&log.NewValues,
			&log.CreatedAt,
			&log.IPAddress,
			&log.UserAgent,
			&log.Status,
			&log.ErrorMessage,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan log: %w", err)
		}

		logs = append(logs, log)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}

	return logs, totalCount, nil
}

// SearchLogs searches activity logs by keyword
func (s *ActivityLogService) SearchLogs(keyword string, limit, offset int) ([]models.ActivityLog, int, error) {
	searchTerm := "%" + keyword + "%"

	countQuery := `
		SELECT COUNT(*) FROM activity_logs
		WHERE description LIKE ? OR username LIKE ? OR entity_type LIKE ?
	`

	var totalCount int
	err := s.db.QueryRow(countQuery, searchTerm, searchTerm, searchTerm).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	query := `
		SELECT 
			id, user_id, username, user_role, action, entity_type, entity_id,
			description, old_values, new_values, created_at, ip_address,
			user_agent, status, error_message
		FROM activity_logs
		WHERE description LIKE ? OR username LIKE ? OR entity_type LIKE ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.Query(query, searchTerm, searchTerm, searchTerm, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search logs: %w", err)
	}
	defer rows.Close()

	logs := []models.ActivityLog{}

	for rows.Next() {
		var log models.ActivityLog
		err := rows.Scan(
			&log.ID,
			&log.UserID,
			&log.Username,
			&log.UserRole,
			&log.Action,
			&log.EntityType,
			&log.EntityID,
			&log.Description,
			&log.OldValues,
			&log.NewValues,
			&log.CreatedAt,
			&log.IPAddress,
			&log.UserAgent,
			&log.Status,
			&log.ErrorMessage,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan log: %w", err)
		}

		logs = append(logs, log)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}

	return logs, totalCount, nil
}
