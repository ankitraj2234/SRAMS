package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/srams/backend/internal/models"
)

var (
	ErrNotSuperAdmin    = errors.New("only super admin can perform this action")
	ErrAuditLogNotFound = errors.New("audit log not found")
	ErrReasonRequired   = errors.New("deletion reason is required")
)

type AuditService struct {
	db *sql.DB
}

func NewAuditService(db *sql.DB) *AuditService {
	return &AuditService{db: db}
}

func (s *AuditService) Create(ctx context.Context, input models.CreateAuditLogInput) (*models.AuditLog, error) {
	sanitizedMetadata := sanitizeMetadata(input.Metadata)
	metadataJSON, err := json.Marshal(sanitizedMetadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	log := &models.AuditLog{
		ID:         uuid.New(),
		ActorID:    input.ActorID,
		ActorRole:  input.ActorRole,
		ActionType: input.ActionType,
		TargetType: input.TargetType,
		TargetID:   input.TargetID,
		Metadata:   metadataJSON,
		IPAddress:  input.IPAddress,
		DeviceID:   input.DeviceID,
		UserAgent:  input.UserAgent,
		CreatedAt:  time.Now(),
	}

	// Handle nullable UUID fields
	var actorIDStr, targetIDStr interface{}
	if input.ActorID != nil {
		actorIDStr = input.ActorID.String()
	}
	if input.TargetID != nil {
		targetIDStr = input.TargetID.String()
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO audit_logs (id, actor_id, actor_role, action_type, target_type, target_id, metadata, ip_address, device_id, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, log.ID.String(), actorIDStr, log.ActorRole, log.ActionType, log.TargetType, targetIDStr, string(log.Metadata), log.IPAddress, log.DeviceID, log.UserAgent, now)

	if err != nil {
		return nil, fmt.Errorf("failed to create audit log: %w", err)
	}

	return log, nil
}

func sanitizeMetadata(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return make(map[string]interface{})
	}

	sanitized := make(map[string]interface{})
	for key, value := range data {
		switch v := value.(type) {
		case string:
			sanitized[key] = sanitizeString(v)
		case map[string]interface{}:
			sanitized[key] = sanitizeMetadata(v)
		case []interface{}:
			sanitized[key] = sanitizeSlice(v)
		default:
			sanitized[key] = value
		}
	}
	return sanitized
}

func sanitizeSlice(data []interface{}) []interface{} {
	sanitized := make([]interface{}, len(data))
	for i, value := range data {
		switch v := value.(type) {
		case string:
			sanitized[i] = sanitizeString(v)
		case map[string]interface{}:
			sanitized[i] = sanitizeMetadata(v)
		case []interface{}:
			sanitized[i] = sanitizeSlice(v)
		default:
			sanitized[i] = value
		}
	}
	return sanitized
}

func sanitizeString(s string) string {
	s = html.EscapeString(s)
	s = strings.ReplaceAll(s, "javascript:", "")
	s = strings.ReplaceAll(s, "onerror=", "")
	s = strings.ReplaceAll(s, "onclick=", "")
	return s
}

func (s *AuditService) List(ctx context.Context, filter models.AuditLogFilter) ([]*models.AuditLog, int, error) {
	whereClause := "WHERE 1=1"
	args := []interface{}{}

	if !filter.IncludeDeleted {
		whereClause += " AND deleted_at IS NULL"
	}

	if filter.ActorID != nil {
		whereClause += " AND actor_id = ?"
		args = append(args, filter.ActorID.String())
	}

	if filter.ActionType != "" {
		whereClause += " AND action_type = ?"
		args = append(args, filter.ActionType)
	}

	if filter.TargetType != "" {
		whereClause += " AND target_type = ?"
		args = append(args, filter.TargetType)
	}

	if filter.StartDate != nil {
		whereClause += " AND created_at >= ?"
		args = append(args, filter.StartDate.UTC().Format(time.RFC3339))
	}

	if filter.EndDate != nil {
		whereClause += " AND created_at <= ?"
		args = append(args, filter.EndDate.UTC().Format(time.RFC3339))
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM audit_logs " + whereClause
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// Get logs
	listQuery := fmt.Sprintf(`
		SELECT id, actor_id, actor_role, action_type, target_type, target_id, metadata, ip_address, device_id, user_agent, created_at, deleted_at, deleted_by, deletion_reason
		FROM audit_logs %s
		ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, whereClause)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list audit logs: %w", err)
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		log := &models.AuditLog{}
		var actorID, targetID, deletedBy sql.NullString
		var createdAt, deletedAt sql.NullString
		var metadata, deletionReason sql.NullString

		err := rows.Scan(
			&log.ID, &actorID, &log.ActorRole, &log.ActionType, &log.TargetType,
			&targetID, &metadata, &log.IPAddress, &log.DeviceID, &log.UserAgent,
			&createdAt, &deletedAt, &deletedBy, &deletionReason,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan audit log: %w", err)
		}

		if actorID.Valid {
			if id, err := uuid.Parse(actorID.String); err == nil {
				log.ActorID = &id
			}
		}
		if targetID.Valid {
			if id, err := uuid.Parse(targetID.String); err == nil {
				log.TargetID = &id
			}
		}
		if metadata.Valid {
			log.Metadata = []byte(metadata.String)
		}
		if createdAt.Valid {
			if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
				log.CreatedAt = t
			}
		}
		if deletedAt.Valid {
			if t, err := time.Parse(time.RFC3339, deletedAt.String); err == nil {
				log.DeletedAt = &t
			}
		}
		if deletedBy.Valid {
			if id, err := uuid.Parse(deletedBy.String); err == nil {
				log.DeletedBy = &id
			}
		}
		if deletionReason.Valid {
			log.DeletionReason = &deletionReason.String
		}

		logs = append(logs, log)
	}

	return logs, total, nil
}

func (s *AuditService) GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	log := &models.AuditLog{}
	var actorID, targetID, deletedBy sql.NullString
	var createdAt, deletedAt sql.NullString
	var metadata, deletionReason sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, actor_id, actor_role, action_type, target_type, target_id, metadata, ip_address, device_id, user_agent, created_at, deleted_at, deleted_by, deletion_reason
		FROM audit_logs WHERE id = ?
	`, id.String()).Scan(
		&log.ID, &actorID, &log.ActorRole, &log.ActionType, &log.TargetType,
		&targetID, &metadata, &log.IPAddress, &log.DeviceID, &log.UserAgent,
		&createdAt, &deletedAt, &deletedBy, &deletionReason,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAuditLogNotFound
		}
		return nil, err
	}

	if actorID.Valid {
		if id, err := uuid.Parse(actorID.String); err == nil {
			log.ActorID = &id
		}
	}
	if targetID.Valid {
		if id, err := uuid.Parse(targetID.String); err == nil {
			log.TargetID = &id
		}
	}
	if metadata.Valid {
		log.Metadata = []byte(metadata.String)
	}
	if createdAt.Valid {
		if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
			log.CreatedAt = t
		}
	}

	return log, nil
}

func (s *AuditService) Delete(ctx context.Context, logID, deletedBy uuid.UUID, reason string, actorRole string) error {
	if actorRole != models.RoleSuperAdmin {
		return ErrNotSuperAdmin
	}

	if reason == "" {
		return ErrReasonRequired
	}

	log, err := s.GetByID(ctx, logID)
	if err != nil {
		return err
	}

	if log.DeletedAt != nil {
		return errors.New("log already deleted")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx, `
		UPDATE audit_logs SET deleted_at = ?, deleted_by = ?, deletion_reason = ?
		WHERE id = ?
	`, now, deletedBy.String(), reason, logID.String())

	return err
}

func (s *AuditService) BulkDelete(ctx context.Context, logIDs []uuid.UUID, deletedBy uuid.UUID, reason string, actorRole string) (int64, error) {
	if actorRole != models.RoleSuperAdmin {
		return 0, ErrNotSuperAdmin
	}

	if reason == "" {
		return 0, ErrReasonRequired
	}

	if len(logIDs) == 0 {
		return 0, nil
	}

	// SQLite doesn't have ANY(), so we use IN with placeholders
	placeholders := make([]string, len(logIDs))
	args := make([]interface{}, 0, len(logIDs)+3)
	now := time.Now().UTC().Format(time.RFC3339)
	args = append(args, now, deletedBy.String(), reason)

	for i, id := range logIDs {
		placeholders[i] = "?"
		args = append(args, id.String())
	}

	query := fmt.Sprintf(`
		UPDATE audit_logs SET deleted_at = ?, deleted_by = ?, deletion_reason = ?
		WHERE id IN (%s) AND deleted_at IS NULL
	`, strings.Join(placeholders, ","))

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	return rowsAffected, nil
}

func (s *AuditService) GetActionTypes(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT action_type FROM audit_logs ORDER BY action_type")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		types = append(types, t)
	}

	return types, nil
}

func (s *AuditService) GetStatsByAction(ctx context.Context, since time.Time) (map[string]interface{}, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT action_type, COUNT(*) as count
		FROM audit_logs
		WHERE created_at >= ? AND deleted_at IS NULL
		GROUP BY action_type
		ORDER BY count DESC
	`, since.UTC().Format(time.RFC3339))
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
	default:
		since = now.AddDate(0, 0, -7)
	}

	return s.GetStatsByAction(ctx, since)
}

func (s *AuditService) GetUserTimeline(ctx context.Context, userID uuid.UUID, limit int) ([]*models.AuditLog, error) {
	// Re-use List logic implicitly or similar query
	filter := models.AuditLogFilter{
		ActorID: &userID,
		Limit:   limit,
	}
	logs, _, err := s.List(ctx, filter)
	// List returns []*models.AuditLog, int, error
	return logs, err
}

func (s *AuditService) GetUserActivityTimeline(ctx context.Context, userID uuid.UUID, days int) ([]map[string]interface{}, error) {
	sinceDate := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)
	rows, err := s.db.QueryContext(ctx, `
		SELECT date(created_at) as date, action_type, COUNT(*) as count
		FROM audit_logs
		WHERE actor_id = ? AND created_at >= ? AND deleted_at IS NULL
		GROUP BY date(created_at), action_type
		ORDER BY date DESC, count DESC
	`, userID.String(), sinceDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var timeline []map[string]interface{}
	for rows.Next() {
		var date, actionType string
		var count int
		if err := rows.Scan(&date, &actionType, &count); err != nil {
			return nil, err
		}
		timeline = append(timeline, map[string]interface{}{
			"date":        date,
			"action_type": actionType,
			"count":       count,
		})
	}

	return timeline, nil
}

func (s *AuditService) LogLogin(ctx context.Context, userID uuid.UUID, role, ip, deviceID, userAgent string, success bool) error {
	actionType := models.ActionLogin
	if !success {
		actionType = models.ActionLoginFailed
	}

	_, err := s.Create(ctx, models.CreateAuditLogInput{
		ActorID:    &userID,
		ActorRole:  role,
		ActionType: actionType,
		TargetType: "session",
		Metadata:   map[string]interface{}{"success": success},
		IPAddress:  ip,
		DeviceID:   deviceID,
		UserAgent:  userAgent,
	})
	return err
}

func (s *AuditService) LogLogout(ctx context.Context, userID uuid.UUID, role, ip, deviceID, userAgent string) error {
	_, err := s.Create(ctx, models.CreateAuditLogInput{
		ActorID:    &userID,
		ActorRole:  role,
		ActionType: models.ActionLogout,
		TargetType: "session",
		IPAddress:  ip,
		DeviceID:   deviceID,
		UserAgent:  userAgent,
	})
	return err
}

func (s *AuditService) LogDocumentAccess(ctx context.Context, userID, documentID uuid.UUID, role, action, ip, deviceID, userAgent string, metadata map[string]interface{}) error {
	_, err := s.Create(ctx, models.CreateAuditLogInput{
		ActorID:    &userID,
		ActorRole:  role,
		ActionType: action,
		TargetType: "document",
		TargetID:   &documentID,
		Metadata:   metadata,
		IPAddress:  ip,
		DeviceID:   deviceID,
		UserAgent:  userAgent,
	})
	return err
}
