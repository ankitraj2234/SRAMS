package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srams/backend/internal/models"
)

// AuditService handles audit log operations with PostgreSQL
type AuditService struct {
	pool *pgxpool.Pool
}

// NewAuditService creates a new PostgreSQL audit service
func NewAuditService(pool *pgxpool.Pool) *AuditService {
	return &AuditService{pool: pool}
}

// Create creates a new audit log entry
func (s *AuditService) Create(ctx context.Context, input models.CreateAuditLogInput) (*models.AuditLog, error) {
	log := &models.AuditLog{
		ID:         uuid.New(),
		ActorID:    input.ActorID,
		ActorRole:  input.ActorRole,
		ActionType: input.ActionType,
		TargetType: input.TargetType,
		TargetID:   input.TargetID,
		IPAddress:  input.IPAddress,
		DeviceID:   input.DeviceID,
		UserAgent:  input.UserAgent,
	}

	// Convert metadata to JSON
	var metadataJSON []byte
	if input.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(input.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
	} else {
		metadataJSON = []byte("{}")
	}
	log.Metadata = metadataJSON

	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit.logs (
			id, actor_id, actor_role, action_type, target_type, target_id,
			metadata, ip_address, device_id, user_agent
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::inet, $9, $10)
	`, log.ID, log.ActorID, log.ActorRole, log.ActionType, log.TargetType,
		log.TargetID, metadataJSON, nullableString(input.IPAddress), input.DeviceID, input.UserAgent)

	if err != nil {
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	return log, nil
}

// LogLogin logs a login event
func (s *AuditService) LogLogin(ctx context.Context, userID uuid.UUID, role, ip, deviceID, userAgent string, success bool) error {
	actionType := models.ActionLogin
	if !success {
		actionType = models.ActionLoginFailed
	}

	_, err := s.Create(ctx, models.CreateAuditLogInput{
		ActorID:    &userID,
		ActorRole:  role,
		ActionType: actionType,
		TargetType: "user",
		TargetID:   &userID,
		Metadata:   map[string]interface{}{"success": success},
		IPAddress:  ip,
		DeviceID:   deviceID,
		UserAgent:  userAgent,
	})
	return err
}

// LogLogout logs a logout event
func (s *AuditService) LogLogout(ctx context.Context, userID uuid.UUID, role, ip, deviceID, userAgent string) error {
	_, err := s.Create(ctx, models.CreateAuditLogInput{
		ActorID:    &userID,
		ActorRole:  role,
		ActionType: models.ActionLogout,
		TargetType: "user",
		TargetID:   &userID,
		IPAddress:  ip,
		DeviceID:   deviceID,
		UserAgent:  userAgent,
	})
	return err
}

// List returns audit logs with pagination and filtering
func (s *AuditService) List(ctx context.Context, filter models.AuditLogFilter) ([]*models.AuditLog, int, error) {
	// Build query dynamically
	baseQuery := "SELECT id, actor_id, actor_role, action_type, target_type, target_id, metadata, ip_address, device_id, user_agent, created_at, deleted_at, deleted_by, deletion_reason FROM audit.logs WHERE deleted_at IS NULL"
	countQuery := "SELECT COUNT(*) FROM audit.logs WHERE deleted_at IS NULL"

	args := []interface{}{}
	argIndex := 1

	// Add filters
	if filter.ActorID != nil {
		baseQuery += fmt.Sprintf(" AND actor_id = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND actor_id = $%d", argIndex)
		args = append(args, *filter.ActorID)
		argIndex++
	}
	if filter.ActionType != "" {
		baseQuery += fmt.Sprintf(" AND action_type = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND action_type = $%d", argIndex)
		args = append(args, filter.ActionType)
		argIndex++
	}
	if filter.TargetType != "" {
		baseQuery += fmt.Sprintf(" AND target_type = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND target_type = $%d", argIndex)
		args = append(args, filter.TargetType)
		argIndex++
	}
	if filter.TargetID != nil {
		baseQuery += fmt.Sprintf(" AND target_id = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND target_id = $%d", argIndex)
		args = append(args, *filter.TargetID)
		argIndex++
	}
	if filter.UserID != nil {
		// Assuming UserID filter means ActorID or TargetID is the user?
		// Usually AuditLogFilter has UserID which corresponds to ActorID for user activity
		baseQuery += fmt.Sprintf(" AND actor_id = $%d", argIndex)
		countQuery += fmt.Sprintf(" AND actor_id = $%d", argIndex)
		args = append(args, *filter.UserID)
		argIndex++
	}
	if filter.StartDate != nil {
		baseQuery += fmt.Sprintf(" AND created_at >= $%d", argIndex)
		countQuery += fmt.Sprintf(" AND created_at >= $%d", argIndex)
		args = append(args, *filter.StartDate)
		argIndex++
	}
	if filter.EndDate != nil {
		baseQuery += fmt.Sprintf(" AND created_at <= $%d", argIndex)
		countQuery += fmt.Sprintf(" AND created_at <= $%d", argIndex)
		args = append(args, *filter.EndDate)
		argIndex++
	}

	// Count total
	var total int
	err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// Add ordering and pagination
	baseQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, filter.Limit, filter.Offset)

	// Execute query
	rows, err := s.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list audit logs: %w", err)
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		log := &models.AuditLog{}
		var createdAt time.Time
		var deletedAt sql.NullTime
		var ipAddress sql.NullString

		err := rows.Scan(
			&log.ID, &log.ActorID, &log.ActorRole, &log.ActionType, &log.TargetType,
			&log.TargetID, &log.Metadata, &ipAddress, &log.DeviceID, &log.UserAgent,
			&createdAt, &deletedAt, &log.DeletedBy, &log.DeletionReason,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan audit log: %w", err)
		}

		log.CreatedAt = createdAt
		if deletedAt.Valid {
			log.DeletedAt = &deletedAt.Time
		}
		if ipAddress.Valid {
			log.IPAddress = ipAddress.String
		}

		logs = append(logs, log)
	}

	return logs, total, nil
}

// GetByID retrieves an audit log by ID
func (s *AuditService) GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	log := &models.AuditLog{}
	var createdAt time.Time
	var deletedAt sql.NullTime
	var ipAddress sql.NullString

	err := s.pool.QueryRow(ctx, `
		SELECT id, actor_id, actor_role, action_type, target_type, target_id,
			metadata, ip_address, device_id, user_agent, created_at,
			deleted_at, deleted_by, deletion_reason
		FROM audit.logs WHERE id = $1
	`, id).Scan(
		&log.ID, &log.ActorID, &log.ActorRole, &log.ActionType, &log.TargetType,
		&log.TargetID, &log.Metadata, &ipAddress, &log.DeviceID, &log.UserAgent,
		&createdAt, &deletedAt, &log.DeletedBy, &log.DeletionReason,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get audit log: %w", err)
	}

	log.CreatedAt = createdAt
	if deletedAt.Valid {
		log.DeletedAt = &deletedAt.Time
	}
	if ipAddress.Valid {
		log.IPAddress = ipAddress.String
	}

	return log, nil
}

// Delete soft-deletes an audit log (only super admin)
func (s *AuditService) Delete(ctx context.Context, id, deletedBy uuid.UUID, reason string, role string) error {
	if role != "super_admin" { // Assuming models.RoleSuperAdmin is "super_admin" or import models
		return fmt.Errorf("only super admin can perform this action")
	}
	// The trigger will enforce append-only semantics, but we do soft delete here
	_, err := s.pool.Exec(ctx, `
		UPDATE audit.logs 
		SET deleted_at = NOW(), deleted_by = $1, deletion_reason = $2
		WHERE id = $3 AND deleted_at IS NULL
	`, deletedBy, reason, id)
	return err
}

// BulkDelete soft-deletes multiple audit logs
func (s *AuditService) BulkDelete(ctx context.Context, ids []uuid.UUID, deletedBy uuid.UUID, reason string, role string) (int64, error) {
	if role != "super_admin" {
		return 0, fmt.Errorf("only super admin can perform this action")
	}

	// Begin transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var totalDeleted int64
	for _, id := range ids {
		tag, err := tx.Exec(ctx, `
			UPDATE audit.logs 
			SET deleted_at = NOW(), deleted_by = $1, deletion_reason = $2 
			WHERE id = $3 AND deleted_at IS NULL
		`, deletedBy, reason, id)
		if err != nil {
			return 0, err
		}
		totalDeleted += tag.RowsAffected()
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return totalDeleted, nil
}

// GetActionTypes returns all distinct action types
func (s *AuditService) GetActionTypes(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT action_type FROM audit.logs ORDER BY action_type
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []string
	for rows.Next() {
		var actionType string
		if err := rows.Scan(&actionType); err != nil {
			return nil, err
		}
		types = append(types, actionType)
	}

	return types, nil
}

// GetStats returns audit log statistics
func (s *AuditService) GetStats(ctx context.Context, period string) (map[string]interface{}, error) {
	var since time.Time
	now := time.Now().UTC()

	switch period {
	case "day":
		since = now.AddDate(0, 0, -1)
	case "week":
		since = now.AddDate(0, 0, -7)
	case "month":
		since = now.AddDate(0, -1, 0)
	default: // "all" or empty
		// No since filter for all time
	}

	whereClause := "WHERE deleted_at IS NULL"
	args := []interface{}{}
	if !since.IsZero() {
		whereClause += " AND created_at >= $1"
		args = append(args, since)
	}

	// Re-implement stats logic with filter support
	stats := make(map[string]interface{})

	// Total logs
	var total int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM audit.logs "+whereClause, args...).Scan(&total)
	if err != nil {
		return nil, err
	}
	stats["total_logs"] = total

	return stats, nil
}

func (s *AuditService) GetStatsByAction(ctx context.Context, since time.Time) (map[string]interface{}, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT action_type, COUNT(*) 
		FROM audit.logs 
		WHERE created_at >= $1 AND deleted_at IS NULL
		GROUP BY action_type
		ORDER BY COUNT(*) DESC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]interface{})
	for rows.Next() {
		var actionType string
		var count int
		if err := rows.Scan(&actionType, &count); err != nil {
			return nil, err
		}
		stats[actionType] = count
	}
	return stats, nil
}

// GetUserActivityTimeline returns a user's activity timeline
func (s *AuditService) GetUserActivityTimeline(ctx context.Context, userID uuid.UUID, days int) ([]map[string]interface{}, error) {
	since := time.Now().AddDate(0, 0, -days)

	rows, err := s.pool.Query(ctx, `
		SELECT date(created_at) as day, action_type, COUNT(*) 
		FROM audit.logs 
		WHERE actor_id = $1 AND created_at >= $2 AND deleted_at IS NULL
		GROUP BY date(created_at), action_type
		ORDER BY day DESC, count DESC
	`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var timeline []map[string]interface{}
	for rows.Next() {
		var day time.Time // or string depending on postgres date cast driver support, usually time.Time
		var actionType string
		var count int
		if err := rows.Scan(&day, &actionType, &count); err != nil {
			return nil, err
		}
		timeline = append(timeline, map[string]interface{}{
			"date":        day.Format("2006-01-02"),
			"action_type": actionType,
			"count":       count,
		})
	}

	return timeline, nil
}

// GetUserTimeline returns a user's activity logs
func (s *AuditService) GetUserTimeline(ctx context.Context, userID uuid.UUID, limit int) ([]*models.AuditLog, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, actor_id, actor_role, action_type, target_type, target_id,
			metadata, ip_address, device_id, user_agent, created_at
		FROM audit.logs 
		WHERE actor_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		log := &models.AuditLog{}
		var createdAt time.Time
		var ipAddress sql.NullString

		err := rows.Scan(
			&log.ID, &log.ActorID, &log.ActorRole, &log.ActionType, &log.TargetType,
			&log.TargetID, &log.Metadata, &ipAddress, &log.DeviceID, &log.UserAgent,
			&createdAt,
		)
		if err != nil {
			return nil, err
		}

		log.CreatedAt = createdAt
		if ipAddress.Valid {
			log.IPAddress = ipAddress.String
		}

		logs = append(logs, log)
	}

	return logs, nil
}

// Helper function to handle nullable strings for INET type
func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
