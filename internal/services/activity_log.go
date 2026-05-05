package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"inventaris-lab-kom/internal/models"
)

// ActivityLogService handles activity logging operations
type ActivityLogService struct {
	db *sql.DB
}

// NewActivityLogService creates a new activity log service
func NewActivityLogService(db *sql.DB) *ActivityLogService {
	return &ActivityLogService{db: db}
}

// Log creates a new activity log entry
func (s *ActivityLogService) Log(log *models.ActivityLog) error {
	query := `
		INSERT INTO activity_logs (
			user_id, username, user_role, action, entity_type, entity_id,
			description, old_values, new_values, created_at, ip_address,
			user_agent, status, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(
		query,
		log.UserID,
		log.Username,
		log.UserRole,
		log.Action,
		log.EntityType,
		log.EntityID,
		log.Description,
		log.OldValues,
		log.NewValues,
		log.CreatedAt,
		log.IPAddress,
		log.UserAgent,
		log.Status,
		log.ErrorMessage,
	)

	if err != nil {
		return fmt.Errorf("failed to insert activity log: %w", err)
	}

	return nil
}

// LogCreate logs a create action
func (s *ActivityLogService) LogCreate(userID int, username, role, entityType string, entityID int, newValues interface{}, ipAddress, userAgent string) error {
	newValuesJSON, err := json.Marshal(newValues)
	if err != nil {
		return fmt.Errorf("failed to marshal new values: %w", err)
	}

	description := fmt.Sprintf("Created %s #%d", entityType, entityID)

	log := &models.ActivityLog{
		UserID:      userID,
		Username:    username,
		UserRole:    role,
		Action:      "create",
		EntityType:  entityType,
		EntityID:    &entityID,
		Description: description,
		OldValues:   "",
		NewValues:   string(newValuesJSON),
		CreatedAt:   time.Now(),
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Status:      "success",
	}

	return s.Log(log)
}

// LogUpdate logs an update action
func (s *ActivityLogService) LogUpdate(userID int, username, role, entityType string, entityID int, oldValues, newValues interface{}, ipAddress, userAgent string) error {
	oldValuesJSON, err := json.Marshal(oldValues)
	if err != nil {
		return fmt.Errorf("failed to marshal old values: %w", err)
	}

	newValuesJSON, err := json.Marshal(newValues)
	if err != nil {
		return fmt.Errorf("failed to marshal new values: %w", err)
	}

	description := fmt.Sprintf("Updated %s #%d", entityType, entityID)

	log := &models.ActivityLog{
		UserID:      userID,
		Username:    username,
		UserRole:    role,
		Action:      "update",
		EntityType:  entityType,
		EntityID:    &entityID,
		Description: description,
		OldValues:   string(oldValuesJSON),
		NewValues:   string(newValuesJSON),
		CreatedAt:   time.Now(),
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Status:      "success",
	}

	return s.Log(log)
}

// LogDelete logs a delete action
func (s *ActivityLogService) LogDelete(userID int, username, role, entityType string, entityID int, oldValues interface{}, ipAddress, userAgent string) error {
	oldValuesJSON, err := json.Marshal(oldValues)
	if err != nil {
		return fmt.Errorf("failed to marshal old values: %w", err)
	}

	description := fmt.Sprintf("Deleted %s #%d", entityType, entityID)

	log := &models.ActivityLog{
		UserID:      userID,
		Username:    username,
		UserRole:    role,
		Action:      "delete",
		EntityType:  entityType,
		EntityID:    &entityID,
		Description: description,
		OldValues:   string(oldValuesJSON),
		NewValues:   "",
		CreatedAt:   time.Now(),
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Status:      "success",
	}

	return s.Log(log)
}

// LogUpload logs a file upload action
func (s *ActivityLogService) LogUpload(userID int, username, role, entityType string, entityID int, filename, fileType string, ipAddress, userAgent string) error {
	description := fmt.Sprintf("Uploaded %s for %s #%d: %s", fileType, entityType, entityID, filename)

	newValues := map[string]string{
		"filename":  filename,
		"file_type": fileType,
	}

	newValuesJSON, err := json.Marshal(newValues)
	if err != nil {
		return fmt.Errorf("failed to marshal new values: %w", err)
	}

	log := &models.ActivityLog{
		UserID:      userID,
		Username:    username,
		UserRole:    role,
		Action:      "upload",
		EntityType:  entityType,
		EntityID:    &entityID,
		Description: description,
		OldValues:   "",
		NewValues:   string(newValuesJSON),
		CreatedAt:   time.Now(),
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Status:      "success",
	}

	return s.Log(log)
}

// LogAuth logs authentication events (login, logout)
func (s *ActivityLogService) LogAuth(userID int, username, role, action string, success bool, ipAddress, userAgent string, errorMsg string) error {
	status := "success"
	if !success {
		status = "failed"
	}

	description := fmt.Sprintf("User '%s' %s", username, action)
	if !success {
		description += " (failed)"
	}

	log := &models.ActivityLog{
		UserID:       userID,
		Username:     username,
		UserRole:     role,
		Action:       action,
		EntityType:   "auth",
		EntityID:     nil,
		Description:  description,
		OldValues:    "",
		NewValues:    "",
		CreatedAt:    time.Now(),
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		Status:       status,
		ErrorMessage: errorMsg,
	}

	return s.Log(log)
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
