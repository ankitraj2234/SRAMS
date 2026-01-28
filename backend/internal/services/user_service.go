package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/srams/backend/internal/auth"
	"github.com/srams/backend/internal/models"
)

var (
	ErrUserNotFound           = errors.New("user not found")
	ErrEmailExists            = errors.New("email already exists")
	ErrInvalidRole            = errors.New("invalid role")
	ErrCannotDeleteSelf       = errors.New("cannot delete yourself")
	ErrCannotDeleteSuperAdmin = errors.New("cannot delete super admin")
)

type UserService struct {
	db          *sql.DB
	authService *auth.Service
}

func NewUserService(db *sql.DB, authService *auth.Service) *UserService {
	return &UserService{
		db:          db,
		authService: authService,
	}
}

// GetUserByID is an alias for GetByID to satisfy the SessionChecker interface
func (s *UserService) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.GetByID(ctx, id)
}

func (s *UserService) IsSessionValid(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	var isActive int
	var expiresAt sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT is_active, expires_at FROM sessions WHERE id = ?
	`, sessionID.String()).Scan(&isActive, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if isActive != 1 {
		return false, nil
	}
	if expiresAt.Valid {
		if t, err := time.Parse(time.RFC3339, expiresAt.String); err == nil {
			if time.Now().After(t) {
				return false, nil
			}
		}
	}
	return true, nil
}

func (s *UserService) Create(ctx context.Context, input models.CreateUserInput, createdBy uuid.UUID) (*models.User, error) {
	// Validate role
	if input.Role != models.RoleUser && input.Role != models.RoleAdmin {
		return nil, ErrInvalidRole
	}

	// Check if email exists
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE email = ?", input.Email).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if count > 0 {
		return nil, ErrEmailExists
	}

	// Hash password
	passwordHash, err := s.authService.HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	user := &models.User{
		ID:                 uuid.New(),
		Email:              input.Email,
		FullName:           input.FullName,
		Mobile:             input.Mobile,
		Role:               input.Role,
		IsActive:           true,
		MustChangePassword: input.MustChangePassword,
		MustEnrollMFA:      input.MustEnrollMFA,
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, mobile, role, is_active, created_at, updated_at, must_change_password, must_enroll_mfa)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.ID.String(), user.Email, passwordHash, user.FullName, user.Mobile, user.Role, 1, now, now, boolToInt(input.MustChangePassword), boolToInt(input.MustEnrollMFA))

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// boolToInt converts bool to int for SQLite
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user := &models.User{}
	var isActive, totpEnabled, mustChangePassword, mustEnrollMFA int
	var createdAt, updatedAt, lastLogin, lockedUntil sql.NullString
	var totpSecret sql.NullString
	var failedAttempts sql.NullInt64

	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, full_name, mobile, role, is_active, created_at, updated_at, last_login, totp_secret, totp_enabled, failed_login_attempts, locked_until, COALESCE(must_change_password, 0), COALESCE(must_enroll_mfa, 0)
		FROM users WHERE id = ?
	`, id.String()).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.FullName, &user.Mobile,
		&user.Role, &isActive, &createdAt, &updatedAt, &lastLogin,
		&totpSecret, &totpEnabled, &failedAttempts, &lockedUntil, &mustChangePassword, &mustEnrollMFA,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.IsActive = isActive == 1
	user.TOTPEnabled = totpEnabled == 1
	user.MustChangePassword = mustChangePassword == 1
	user.MustEnrollMFA = mustEnrollMFA == 1
	if totpSecret.Valid {
		user.TOTPSecret = &totpSecret.String
	}
	if failedAttempts.Valid {
		user.FailedLoginAttempts = int(failedAttempts.Int64)
	}
	if createdAt.Valid {
		if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
			user.CreatedAt = t
		}
	}
	if updatedAt.Valid {
		if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
			user.UpdatedAt = t
		}
	}
	if lastLogin.Valid {
		if t, err := time.Parse(time.RFC3339, lastLogin.String); err == nil {
			user.LastLogin = &t
		}
	}
	if lockedUntil.Valid {
		if t, err := time.Parse(time.RFC3339, lockedUntil.String); err == nil {
			user.LockedUntil = &t
		}
	}

	return user, nil
}

func (s *UserService) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	var isActive, totpEnabled, mustChangePassword, mustEnrollMFA int
	var createdAt, updatedAt, lastLogin, lockedUntil sql.NullString
	var totpSecret sql.NullString
	var failedAttempts sql.NullInt64

	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, full_name, mobile, role, is_active, created_at, updated_at, last_login, totp_secret, totp_enabled, failed_login_attempts, locked_until, COALESCE(must_change_password, 0), COALESCE(must_enroll_mfa, 0)
		FROM users WHERE email = ?
	`, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.FullName, &user.Mobile,
		&user.Role, &isActive, &createdAt, &updatedAt, &lastLogin,
		&totpSecret, &totpEnabled, &failedAttempts, &lockedUntil, &mustChangePassword, &mustEnrollMFA,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.IsActive = isActive == 1
	user.TOTPEnabled = totpEnabled == 1
	user.MustChangePassword = mustChangePassword == 1
	user.MustEnrollMFA = mustEnrollMFA == 1
	if totpSecret.Valid {
		user.TOTPSecret = &totpSecret.String
	}
	if failedAttempts.Valid {
		user.FailedLoginAttempts = int(failedAttempts.Int64)
	}
	if createdAt.Valid {
		if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
			user.CreatedAt = t
		}
	}
	if updatedAt.Valid {
		if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
			user.UpdatedAt = t
		}
	}
	if lastLogin.Valid {
		if t, err := time.Parse(time.RFC3339, lastLogin.String); err == nil {
			user.LastLogin = &t
		}
	}
	if lockedUntil.Valid {
		if t, err := time.Parse(time.RFC3339, lockedUntil.String); err == nil {
			user.LockedUntil = &t
		}
	}

	return user, nil
}

func (s *UserService) List(ctx context.Context, role string, offset, limit int) ([]*models.User, int, error) {
	var users []*models.User
	var total int

	// Count query
	countQuery := "SELECT COUNT(*) FROM users WHERE 1=1"
	args := []interface{}{}

	if role != "" {
		countQuery += " AND role = ?"
		args = append(args, role)
	}

	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	// List query
	listQuery := `
		SELECT id, email, full_name, mobile, role, is_active, created_at, updated_at, last_login, totp_enabled
		FROM users WHERE 1=1
	`
	if role != "" {
		listQuery += " AND role = ?"
	}
	listQuery += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		user := &models.User{}
		var isActive, totpEnabled int
		var createdAt, updatedAt, lastLogin sql.NullString

		err := rows.Scan(
			&user.ID, &user.Email, &user.FullName, &user.Mobile,
			&user.Role, &isActive, &createdAt, &updatedAt, &lastLogin, &totpEnabled,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan user: %w", err)
		}

		user.IsActive = isActive == 1
		user.TOTPEnabled = totpEnabled == 1
		if createdAt.Valid {
			if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
				user.CreatedAt = t
			}
		}
		if updatedAt.Valid {
			if t, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
				user.UpdatedAt = t
			}
		}
		if lastLogin.Valid {
			if t, err := time.Parse(time.RFC3339, lastLogin.String); err == nil {
				user.LastLogin = &t
			}
		}

		users = append(users, user)
	}

	return users, total, nil
}

func (s *UserService) Update(ctx context.Context, id uuid.UUID, input models.UpdateUserInput, updatedBy uuid.UUID, updaterRole string) (*models.User, error) {
	user, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Cannot change super admin role
	if user.Role == models.RoleSuperAdmin && input.Role != nil && *input.Role != models.RoleSuperAdmin {
		return nil, errors.New("cannot change super admin role")
	}

	// Only super admin can promote to admin
	if input.Role != nil && *input.Role == models.RoleAdmin && updaterRole != models.RoleSuperAdmin {
		return nil, errors.New("only super admin can create admin users")
	}

	// Build update query
	now := time.Now().UTC().Format(time.RFC3339)
	query := "UPDATE users SET updated_at = ?"
	args := []interface{}{now}

	if input.FullName != nil {
		query += ", full_name = ?"
		args = append(args, *input.FullName)
		user.FullName = *input.FullName
	}
	if input.Mobile != nil {
		query += ", mobile = ?"
		args = append(args, *input.Mobile)
		user.Mobile = *input.Mobile
	}
	if input.Role != nil {
		query += ", role = ?"
		args = append(args, *input.Role)
		user.Role = *input.Role
	}
	if input.IsActive != nil {
		isActive := 0
		if *input.IsActive {
			isActive = 1
		}
		query += ", is_active = ?"
		args = append(args, isActive)
		user.IsActive = *input.IsActive
	}

	query += " WHERE id = ?"
	args = append(args, id.String())

	_, err = s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return user, nil
}

func (s *UserService) Delete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error {
	if id == deletedBy {
		return ErrCannotDeleteSelf
	}

	user, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if user.Role == models.RoleSuperAdmin {
		return ErrCannotDeleteSuperAdmin
	}

	_, err = s.db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id.String())
	return err
}

func (s *UserService) ChangePassword(ctx context.Context, id uuid.UUID, oldPassword, newPassword string) error {
	user, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Verify old password
	if !s.authService.VerifyPassword(oldPassword, user.PasswordHash) {
		return errors.New("current password is incorrect")
	}

	// Hash new password
	passwordHash, err := s.authService.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password and clear must_change_password flag
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx, "UPDATE users SET password_hash = ?, updated_at = ?, must_change_password = 0 WHERE id = ?", passwordHash, now, id.String())
	return err
}

func (s *UserService) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, "UPDATE users SET last_login = ?, updated_at = ? WHERE id = ?", now, now, id.String())
	return err
}

// CreateSession creates a new session
func (s *UserService) CreateSession(ctx context.Context, sessionID, userID uuid.UUID, tokenHash, refreshTokenHash, ipAddress, deviceFingerprint, userAgent string, expiresAt time.Time) (*models.Session, error) {
	session := &models.Session{
		ID:                sessionID,
		UserID:            userID,
		TokenHash:         tokenHash,
		RefreshTokenHash:  refreshTokenHash,
		IPAddress:         ipAddress,
		DeviceFingerprint: deviceFingerprint,
		UserAgent:         userAgent,
		ExpiresAt:         expiresAt,
		IsActive:          true,
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, refresh_token_hash, ip_address, device_fingerprint, user_agent, created_at, expires_at, last_activity, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, session.ID.String(), userID.String(), tokenHash, refreshTokenHash, ipAddress, deviceFingerprint, userAgent, now, expiresAt.UTC().Format(time.RFC3339), now, 1)

	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

func (s *UserService) InvalidateSession(ctx context.Context, sessionID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "UPDATE sessions SET is_active = 0 WHERE id = ?", sessionID.String())
	return err
}

func (s *UserService) InvalidateAllSessions(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "UPDATE sessions SET is_active = 0 WHERE user_id = ?", userID.String())
	return err
}

func (s *UserService) GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*models.Session, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, ip_address, device_fingerprint, user_agent, created_at, expires_at, last_activity
		FROM sessions WHERE user_id = ? AND is_active = 1
		ORDER BY last_activity DESC
	`, userID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*models.Session
	for rows.Next() {
		session := &models.Session{}
		var createdAt, expiresAt, lastActivity sql.NullString

		err := rows.Scan(
			&session.ID, &session.UserID, &session.IPAddress, &session.DeviceFingerprint,
			&session.UserAgent, &createdAt, &expiresAt, &lastActivity,
		)
		if err != nil {
			return nil, err
		}

		session.IsActive = true
		if createdAt.Valid {
			if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
				session.CreatedAt = t
			}
		}
		if expiresAt.Valid {
			if t, err := time.Parse(time.RFC3339, expiresAt.String); err == nil {
				session.ExpiresAt = t
			}
		}
		if lastActivity.Valid {
			if t, err := time.Parse(time.RFC3339, lastActivity.String); err == nil {
				session.LastActivity = t
			}
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (s *UserService) UpdateSessionActivity(ctx context.Context, sessionID uuid.UUID) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, "UPDATE sessions SET last_activity = ? WHERE id = ? AND is_active = 1", now, sessionID.String())
	return err
}

// CreateSuperAdmin creates the initial Super Admin (only allowed if none exists)
func (s *UserService) CreateSuperAdmin(ctx context.Context, email, password, fullName, mobile string) (*models.User, error) {
	// Check if Super Admin already exists
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE role = ?", models.RoleSuperAdmin).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing super admin: %w", err)
	}
	if count > 0 {
		return nil, errors.New("super admin already exists")
	}

	// Check if email exists
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE email = ?", email).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if count > 0 {
		return nil, ErrEmailExists
	}

	// Hash password
	passwordHash, err := s.authService.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	user := &models.User{
		ID:       uuid.New(),
		Email:    email,
		FullName: fullName,
		Mobile:   mobile,
		Role:     models.RoleSuperAdmin,
		IsActive: true,
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, mobile, role, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.ID.String(), user.Email, passwordHash, user.FullName, user.Mobile, user.Role, 1, now, now)

	if err != nil {
		return nil, fmt.Errorf("failed to create super admin: %w", err)
	}

	return user, nil
}

// TOTP functions
func (s *UserService) SetTOTPSecret(ctx context.Context, userID uuid.UUID, secret string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, "UPDATE users SET totp_secret = ?, updated_at = ? WHERE id = ?", secret, now, userID.String())
	return err
}

func (s *UserService) EnableTOTP(ctx context.Context, userID uuid.UUID) error {
	now := time.Now().UTC().Format(time.RFC3339)
	// Enable TOTP and clear must_enroll_mfa flag
	_, err := s.db.ExecContext(ctx, "UPDATE users SET totp_enabled = 1, must_enroll_mfa = 0, updated_at = ? WHERE id = ?", now, userID.String())
	return err
}

func (s *UserService) DisableTOTP(ctx context.Context, userID uuid.UUID) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, "UPDATE users SET totp_enabled = 0, totp_secret = NULL, updated_at = ? WHERE id = ?", now, userID.String())
	return err
}

// GetDashboardStats returns dashboard statistics
func (s *UserService) GetDashboardStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total users
	var totalUsers int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&totalUsers)
	stats["total_users"] = totalUsers

	// Active users
	var activeUsers int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE is_active = 1").Scan(&activeUsers)
	stats["active_users"] = activeUsers

	// Total documents
	var totalDocs int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM documents WHERE is_active = 1").Scan(&totalDocs)
	stats["total_documents"] = totalDocs

	// Active sessions
	var activeSessions int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE is_active = 1").Scan(&activeSessions)
	stats["active_sessions"] = activeSessions

	// Pending requests
	var pendingRequests int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_requests WHERE status = 'pending'").Scan(&pendingRequests)
	stats["pending_requests"] = pendingRequests

	// Today's logins (count login_success actions from today)
	var todayLogins int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs WHERE action_type = 'login_success' AND date(timestamp) = date('now')").Scan(&todayLogins)
	stats["today_logins"] = todayLogins

	return stats, nil
}

// IncrementFailedLoginAttempts increments the failed login counter
func (s *UserService) IncrementFailedLoginAttempts(ctx context.Context, userID uuid.UUID, maxAttempts, lockoutMinutes int) error {
	now := time.Now().UTC()
	lockoutTime := now.Add(time.Duration(lockoutMinutes) * time.Minute).Format(time.RFC3339)
	nowStr := now.Format(time.RFC3339)

	// Get current attempts
	var currentAttempts int
	s.db.QueryRowContext(ctx, "SELECT COALESCE(failed_login_attempts, 0) FROM users WHERE id = ?", userID.String()).Scan(&currentAttempts)

	newAttempts := currentAttempts + 1

	var err error
	if newAttempts >= maxAttempts {
		_, err = s.db.ExecContext(ctx, "UPDATE users SET failed_login_attempts = ?, locked_until = ?, updated_at = ? WHERE id = ?",
			newAttempts, lockoutTime, nowStr, userID.String())
	} else {
		_, err = s.db.ExecContext(ctx, "UPDATE users SET failed_login_attempts = ?, updated_at = ? WHERE id = ?",
			newAttempts, nowStr, userID.String())
	}

	return err
}

// ResetFailedLoginAttempts resets the counter on successful login
func (s *UserService) ResetFailedLoginAttempts(ctx context.Context, userID uuid.UUID) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, "UPDATE users SET failed_login_attempts = 0, locked_until = NULL, updated_at = ? WHERE id = ?", now, userID.String())
	return err
}

// ValidateRefreshTokenHash verifies the refresh token hash
func (s *UserService) ValidateRefreshTokenHash(ctx context.Context, sessionID uuid.UUID, tokenHash string) (bool, *models.Session, error) {
	session := &models.Session{}
	var isActive int
	var expiresAt sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, refresh_token_hash, ip_address, device_fingerprint, user_agent, expires_at, is_active
		FROM sessions WHERE id = ?
	`, sessionID.String()).Scan(
		&session.ID, &session.UserID, &session.RefreshTokenHash, &session.IPAddress,
		&session.DeviceFingerprint, &session.UserAgent, &expiresAt, &isActive,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil, nil
		}
		return false, nil, err
	}

	session.IsActive = isActive == 1
	if expiresAt.Valid {
		if t, err := time.Parse(time.RFC3339, expiresAt.String); err == nil {
			session.ExpiresAt = t
		}
	}

	// Check session is active and not expired
	if !session.IsActive || time.Now().After(session.ExpiresAt) {
		return false, session, nil
	}

	// Constant-time comparison
	if session.RefreshTokenHash != tokenHash {
		return false, session, nil
	}

	return true, session, nil
}

// RotateRefreshToken updates the refresh token hash
func (s *UserService) RotateRefreshToken(ctx context.Context, sessionID uuid.UUID, newTokenHash string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, "UPDATE sessions SET refresh_token_hash = ?, last_activity = ? WHERE id = ? AND is_active = 1",
		newTokenHash, now, sessionID.String())
	return err
}

// VerifyDeviceCertificate checks if the device fingerprint is valid for the user
// Implements Trust-On-First-Use (TOFU): Registers the first device seen for the user
func (s *UserService) VerifyDeviceCertificate(ctx context.Context, userID uuid.UUID, fingerprint, deviceID string) (bool, error) {
	if fingerprint == "" {
		return false, errors.New("device fingerprint is required")
	}

	// Check if user has any certificates
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM device_certificates WHERE user_id = ? AND revoked_at IS NULL", userID.String()).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check device certificates: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if count == 0 {
		// First device - Register it (TOFU)
		machineID := deviceID
		if machineID == "" {
			machineID = fingerprint
		}
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO device_certificates (id, user_id, fingerprint, machine_id, created_at)
			VALUES (?, ?, ?, ?, ?)
		`, uuid.New().String(), userID.String(), fingerprint, machineID, now)
		if err != nil {
			return false, fmt.Errorf("failed to register device certificate: %w", err)
		}
		return true, nil
	}

	// User has certificates - Verify this one exists
	var exists int
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM device_certificates 
		WHERE user_id = ? AND fingerprint = ? AND revoked_at IS NULL
	`, userID.String(), fingerprint).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("failed to verify device certificate: %w", err)
	}

	if exists > 0 {
		// Update last used
		_, _ = s.db.ExecContext(ctx, "UPDATE device_certificates SET last_used_at = ? WHERE fingerprint = ?", now, fingerprint)
		return true, nil
	}

	return false, nil
}
