package services

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/srams/backend/internal/middleware" // For DesktopSession
)

type SystemService struct {
	db          *sql.DB
	storagePath string
}

func NewSystemService(db *sql.DB, storagePath string) *SystemService {
	return &SystemService{
		db:          db,
		storagePath: storagePath,
	}
}

func (s *SystemService) GetHealth(ctx context.Context) (map[string]interface{}, error) {
	err := s.db.PingContext(ctx)
	status := "healthy"
	if err != nil {
		status = "unhealthy"
	}
	stats := s.db.Stats()
	return map[string]interface{}{
		"status": status,
		"database": map[string]interface{}{
			"open_connections": stats.OpenConnections,
			"in_use":           stats.InUse,
			"idle":             stats.Idle,
			"max_open":         stats.MaxOpenConnections,
		},
	}, nil
}

func (s *SystemService) GetServerStatus(ctx context.Context) (map[string]interface{}, error) {
	err := s.db.PingContext(ctx)
	hasDesktopSession := middleware.GetDesktopSession().HasActiveSession()
	return map[string]interface{}{
		"server_running":     true,
		"database_connected": err == nil,
		"desktop_session":    hasDesktopSession,
	}, nil
}

// Desktop Sessions
func (s *SystemService) CreateDesktopSession(ctx context.Context, ip string) (string, error) {
	// IP check is done in Handler usually, but Service just creates token
	token := middleware.GetDesktopSession().CreateSession()
	return token, nil
}

func (s *SystemService) EndDesktopSession(ctx context.Context, token string) error {
	middleware.GetDesktopSession().EndSession()
	return nil
}

func (s *SystemService) ValidateDesktopSession(ctx context.Context, token string) (bool, error) {
	isValid := middleware.GetDesktopSession().ValidateSession(token)
	return isValid, nil
}

// Config
func (s *SystemService) GetConfig(ctx context.Context) (map[string]string, error) {
	// Not implemented: Get single key?
	// Interface says GetConfig returns map[string]string. This seems to mean GetAllConfigs?
	// Handler calls "SELECT key, value FROM system_config".
	return s.GetAllConfigs(ctx)
}

func (s *SystemService) SetConfig(ctx context.Context, key, value string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	// SQLite UPSERT
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO system_config (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = ?
	`, key, value, now, value, now)
	return err
}

func (s *SystemService) GetAllConfigs(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT key, value FROM system_config")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	config := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		config[key] = value
	}
	return config, nil
}

// Analytics
func (s *SystemService) GetDatabaseStats(ctx context.Context) (map[string]interface{}, error) {
	var stats struct {
		UsersCount     int64
		DocumentsCount int64
		SessionsCount  int64
		AuditLogsCount int64
	}

	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&stats.UsersCount)
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM documents WHERE is_active = 1").Scan(&stats.DocumentsCount)
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE is_active = 1").Scan(&stats.SessionsCount)
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs WHERE deleted_at IS NULL").Scan(&stats.AuditLogsCount)

	// SQLite specific size check
	var pageCount, pageSize int64
	s.db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount)
	s.db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize)
	sizeMB := float64(pageCount*pageSize) / (1024 * 1024)

	return map[string]interface{}{
		"users_count":      stats.UsersCount,
		"documents_count":  stats.DocumentsCount,
		"sessions_count":   stats.SessionsCount,
		"audit_logs_count": stats.AuditLogsCount,
		"database_size_mb": sizeMB,
	}, nil
}

func (s *SystemService) GetSessionAnalytics(ctx context.Context) (map[string]interface{}, error) {
	var active, today, week int64
	now := time.Now().UTC()
	todayStr := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	weekAgo := now.AddDate(0, 0, -7).Format(time.RFC3339)
	nowStr := now.Format(time.RFC3339)

	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE is_active = 1 AND expires_at > ?", nowStr).Scan(&active)
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE created_at > ?", todayStr).Scan(&today)
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE created_at > ?", weekAgo).Scan(&week)

	return map[string]interface{}{
		"active_sessions": active,
		"today_sessions":  today,
		"week_sessions":   week,
	}, nil
}

func (s *SystemService) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(ctx, "UPDATE sessions SET is_active = 0 WHERE expires_at < ? AND is_active = 1", now)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Logo
func (s *SystemService) SaveLogo(ctx context.Context, content io.Reader, filename string, size int64) error {
	logoDir := filepath.Join(s.storagePath, "system")
	os.MkdirAll(logoDir, 0755)

	// Determine extension from filename
	ext := filepath.Ext(filename)
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" {
		return fmt.Errorf("invalid file type")
	}

	// Remove existings
	s.DeleteLogo(ctx)

	logoPath := filepath.Join(logoDir, "logo"+ext)
	outFile, err := os.Create(logoPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, content)
	return err
}

func (s *SystemService) GetLogoPath(ctx context.Context) (string, error) {
	logoDir := filepath.Join(s.storagePath, "system")
	extensions := []string{".png", ".jpg", ".jpeg", ".gif"}
	for _, ext := range extensions {
		logoPath := filepath.Join(logoDir, "logo"+ext)
		if _, err := os.Stat(logoPath); err == nil {
			return logoPath, nil
		}
	}
	return "", os.ErrNotExist
}

func (s *SystemService) DeleteLogo(ctx context.Context) error {
	logoDir := filepath.Join(s.storagePath, "system")
	extensions := []string{".png", ".jpg", ".jpeg", ".gif"}
	for _, ext := range extensions {
		os.Remove(filepath.Join(logoDir, "logo"+ext)) // Ignore errors
	}
	return nil
}
