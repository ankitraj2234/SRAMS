package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srams/backend/internal/auth"
	"github.com/srams/backend/internal/models"
)

var (
	ErrUserNotFound           = errors.New("user not found")
	ErrEmailExists            = errors.New("email already exists")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrInvalidRole            = errors.New("invalid role")
	ErrCannotDeleteSelf       = errors.New("cannot delete yourself")
	ErrCannotDeleteSuperAdmin = errors.New("cannot delete super admin")
	ErrAccountLocked          = errors.New("account is locked")
)

// UserService handles user operations with PostgreSQL
type UserService struct {
	pool        *pgxpool.Pool
	authService *auth.Service
}

// NewUserService creates a new PostgreSQL user service
func NewUserService(pool *pgxpool.Pool, authService *auth.Service) *UserService {
	return &UserService{
		pool:        pool,
		authService: authService,
	}
}

// Create creates a new user
func (s *UserService) Create(ctx context.Context, input models.CreateUserInput, createdBy uuid.UUID) (*models.User, error) {
	// Validate role
	if input.Role != models.RoleUser && input.Role != models.RoleAdmin && input.Role != models.RoleSuperAdmin {
		return nil, ErrInvalidRole
	}

	// Check if email exists
	var count int
	err := s.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM srams.users WHERE email = $1",
		input.Email,
	).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if count > 0 {
		return nil, ErrEmailExists
	}

	// Check if mobile exists (if provided)
	if input.Mobile != "" {
		var mobileCount int
		err := s.pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM srams.users WHERE mobile = $1",
			input.Mobile,
		).Scan(&mobileCount)
		if err != nil {
			return nil, fmt.Errorf("failed to check mobile: %w", err)
		}
		if mobileCount > 0 {
			return nil, errors.New("mobile number already exists")
		}
	}

	// Hash password
	passwordHash, err := s.authService.HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
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

	_, err = s.pool.Exec(ctx, `
		INSERT INTO srams.users (
			id, email, password_hash, full_name, mobile, role, 
			is_active, must_change_password, must_enroll_mfa, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, user.ID, user.Email, passwordHash, user.FullName, user.Mobile,
		user.Role, true, input.MustChangePassword, input.MustEnrollMFA, createdBy)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// GetByID retrieves a user by ID
func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user := &models.User{}
	var createdAt, updatedAt time.Time
	var lastLogin, lockedUntil sql.NullTime
	var totpSecret sql.NullString

	err := s.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, full_name, mobile, role, is_active,
			created_at, updated_at, last_login, totp_secret, totp_enabled,
			failed_login_attempts, locked_until, must_change_password, must_enroll_mfa
		FROM srams.users WHERE id = $1
	`, id).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.FullName, &user.Mobile,
		&user.Role, &user.IsActive, &createdAt, &updatedAt, &lastLogin,
		&totpSecret, &user.TOTPEnabled, &user.FailedLoginAttempts, &lockedUntil,
		&user.MustChangePassword, &user.MustEnrollMFA,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.CreatedAt = createdAt
	user.UpdatedAt = updatedAt
	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}
	if lockedUntil.Valid {
		user.LockedUntil = &lockedUntil.Time
	}
	if totpSecret.Valid {
		user.TOTPSecret = &totpSecret.String
	}

	return user, nil
}

// GetByEmail retrieves a user by email
func (s *UserService) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	var createdAt, updatedAt time.Time
	var lastLogin, lockedUntil sql.NullTime
	var totpSecret sql.NullString

	err := s.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, full_name, mobile, role, is_active,
			created_at, updated_at, last_login, totp_secret, totp_enabled,
			failed_login_attempts, locked_until, must_change_password, must_enroll_mfa
		FROM srams.users WHERE email = $1
	`, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.FullName, &user.Mobile,
		&user.Role, &user.IsActive, &createdAt, &updatedAt, &lastLogin,
		&totpSecret, &user.TOTPEnabled, &user.FailedLoginAttempts, &lockedUntil,
		&user.MustChangePassword, &user.MustEnrollMFA,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.CreatedAt = createdAt
	user.UpdatedAt = updatedAt
	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}
	if lockedUntil.Valid {
		user.LockedUntil = &lockedUntil.Time
	}
	if totpSecret.Valid {
		user.TOTPSecret = &totpSecret.String
	}

	return user, nil
}

// GetUserByID is an alias for GetByID (for interface compatibility)
func (s *UserService) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.GetByID(ctx, id)
}

// List returns all users with pagination
func (s *UserService) List(ctx context.Context, role string, offset, limit int) ([]*models.User, int, error) {
	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM srams.users WHERE 1=1"
	args := []interface{}{}
	argIndex := 1

	if role != "" {
		countQuery += fmt.Sprintf(" AND role = $%d", argIndex)
		args = append(args, role)
		argIndex++
	}

	err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	// Build list query
	listQuery := `
		SELECT id, email, full_name, mobile, role, is_active,
			created_at, updated_at, last_login, totp_enabled,
			must_change_password, must_enroll_mfa
		FROM srams.users WHERE 1=1
	`
	if role != "" {
		listQuery += fmt.Sprintf(" AND role = $%d", 1)
	}
	listQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		var createdAt, updatedAt time.Time
		var lastLogin sql.NullTime

		err := rows.Scan(
			&user.ID, &user.Email, &user.FullName, &user.Mobile, &user.Role, &user.IsActive,
			&createdAt, &updatedAt, &lastLogin, &user.TOTPEnabled,
			&user.MustChangePassword, &user.MustEnrollMFA,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan user: %w", err)
		}

		user.CreatedAt = createdAt
		user.UpdatedAt = updatedAt
		if lastLogin.Valid {
			user.LastLogin = &lastLogin.Time
		}

		users = append(users, user)
	}

	return users, total, nil
}

// Update updates a user
func (s *UserService) Update(ctx context.Context, id uuid.UUID, input models.UpdateUserInput, updatedBy uuid.UUID, updaterRole string) (*models.User, error) {
	// Get current user
	user, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Build update query dynamically
	updates := []string{}
	args := []interface{}{}
	argIndex := 1

	if input.FullName != nil {
		updates = append(updates, fmt.Sprintf("full_name = $%d", argIndex))
		args = append(args, *input.FullName)
		user.FullName = *input.FullName
		argIndex++
	}
	if input.Mobile != nil {
		updates = append(updates, fmt.Sprintf("mobile = $%d", argIndex))
		args = append(args, *input.Mobile)
		// Check for uniqueness if changing mobile
		if *input.Mobile != "" && *input.Mobile != user.Mobile {
			var mobileCount int
			err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM srams.users WHERE mobile = $1 AND id != $2", *input.Mobile, id).Scan(&mobileCount)
			if err != nil {
				return nil, fmt.Errorf("failed to check mobile uniqueness: %w", err)
			}
			if mobileCount > 0 {
				return nil, errors.New("mobile number already exists")
			}
		}
		user.Mobile = *input.Mobile
		argIndex++
	}
	if input.Role != nil {
		// Check permission to change role
		if *input.Role == models.RoleSuperAdmin && updaterRole != models.RoleSuperAdmin {
			return nil, errors.New("only super admin can assign super admin role")
		}
		if user.Role == models.RoleSuperAdmin && updaterRole != models.RoleSuperAdmin {
			return nil, errors.New("only super admin can modify super admin")
		}
		updates = append(updates, fmt.Sprintf("role = $%d", argIndex))
		args = append(args, *input.Role)
		user.Role = *input.Role
		argIndex++
	}
	if input.IsActive != nil {
		updates = append(updates, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, *input.IsActive)
		user.IsActive = *input.IsActive
		argIndex++
	}

	if len(updates) == 0 {
		return user, nil
	}

	// Add updated_at
	updates = append(updates, "updated_at = NOW()")

	// Add WHERE clause
	args = append(args, id)
	query := fmt.Sprintf("UPDATE srams.users SET %s WHERE id = $%d",
		joinStrings(updates, ", "), argIndex)

	_, err = s.pool.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return user, nil
}

// Delete deletes a user
func (s *UserService) Delete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error {
	// Cannot delete self
	if id == deletedBy {
		return ErrCannotDeleteSelf
	}

	// Check if user exists and is not super admin
	user, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if user.Role == models.RoleSuperAdmin {
		return ErrCannotDeleteSuperAdmin
	}

	// Delete user (cascades to sessions, access, etc.)
	_, err = s.pool.Exec(ctx, "DELETE FROM srams.users WHERE id = $1", id)
	return err
}

// ChangePassword changes a user's password and clears must_change_password flag
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
	_, err = s.pool.Exec(ctx, `
		UPDATE srams.users 
		SET password_hash = $1, must_change_password = false, updated_at = NOW() 
		WHERE id = $2
	`, passwordHash, id)
	return err
}

// UpdateLastLogin updates the last login timestamp
func (s *UserService) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE srams.users SET last_login = NOW() WHERE id = $1",
		id,
	)
	return err
}

// Session Management

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

	_, err := s.pool.Exec(ctx, `
		INSERT INTO auth.sessions (
			id, user_id, token_hash, refresh_token_hash, ip_address,
			device_fingerprint, user_agent, expires_at, is_active
		) VALUES ($1, $2, $3, $4, $5::inet, $6, $7, $8, $9)
	`, session.ID, session.UserID, tokenHash, refreshTokenHash,
		ipAddress, deviceFingerprint, userAgent, expiresAt, true)

	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

// InvalidateSession invalidates a session
func (s *UserService) InvalidateSession(ctx context.Context, sessionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE auth.sessions SET is_active = false WHERE id = $1",
		sessionID,
	)
	return err
}

// InvalidateAllSessions invalidates all sessions for a user
func (s *UserService) InvalidateAllSessions(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE auth.sessions SET is_active = false WHERE user_id = $1",
		userID,
	)
	return err
}

// InvalidateAllSessionsExcept invalidates all sessions for a user except one
func (s *UserService) InvalidateAllSessionsExcept(ctx context.Context, userID uuid.UUID, exceptSessionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE auth.sessions SET is_active = false WHERE user_id = $1 AND id != $2",
		userID,
		exceptSessionID,
	)
	return err
}

// IsSessionValid checks if a session is valid
func (s *UserService) IsSessionValid(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	var isActive bool
	var expiresAt time.Time

	err := s.pool.QueryRow(ctx, `
		SELECT is_active, expires_at FROM auth.sessions 
		WHERE id = $1
	`, sessionID).Scan(&isActive, &expiresAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	return isActive && expiresAt.After(time.Now()), nil
}

// UpdateSessionActivity updates the last activity timestamp
func (s *UserService) UpdateSessionActivity(ctx context.Context, sessionID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE auth.sessions SET last_activity = NOW() WHERE id = $1",
		sessionID,
	)
	return err
}

// RotateRefreshToken updates the refresh token hash and extends session
func (s *UserService) RotateRefreshToken(ctx context.Context, sessionID uuid.UUID, newTokenHash string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE auth.sessions 
		SET refresh_token_hash = $1, last_activity = NOW(), expires_at = NOW() + INTERVAL '7 days'
		WHERE id = $2
	`, newTokenHash, sessionID)
	return err
}

// ValidateRefreshTokenHash validates the refresh token hash
func (s *UserService) ValidateRefreshTokenHash(ctx context.Context, sessionID uuid.UUID, hash string) (bool, *models.Session, error) {
	session := &models.Session{}
	var expiresAt time.Time

	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, refresh_token_hash, ip_address, device_fingerprint, user_agent, expires_at, is_active 
		FROM auth.sessions WHERE id = $1
	`, sessionID).Scan(
		&session.ID, &session.UserID, &session.RefreshTokenHash,
		&session.IPAddress, &session.DeviceFingerprint, &session.UserAgent, &expiresAt, &session.IsActive,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil, nil
		}
		return false, nil, err
	}

	session.ExpiresAt = expiresAt

	if session.RefreshTokenHash != hash {
		return false, session, nil
	}
	return true, session, nil
}

// TOTP Functions

// SetTOTPSecret sets the TOTP secret for a user
func (s *UserService) SetTOTPSecret(ctx context.Context, userID uuid.UUID, secret string) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE srams.users SET totp_secret = $1, updated_at = NOW() WHERE id = $2",
		secret, userID,
	)
	return err
}

// EnableTOTP enables TOTP and clears must_enroll_mfa flag
func (s *UserService) EnableTOTP(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE srams.users 
		SET totp_enabled = true, must_enroll_mfa = false, updated_at = NOW() 
		WHERE id = $1
	`, userID)
	return err
}

// DisableTOTP disables TOTP for a user
func (s *UserService) DisableTOTP(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE srams.users 
		SET totp_enabled = false, totp_secret = NULL, updated_at = NOW() 
		WHERE id = $1
	`, userID)
	return err
}

// StoreBackupCodes stores backup codes for a user
func (s *UserService) StoreBackupCodes(ctx context.Context, userID uuid.UUID, codes []string) error {
	_, err := s.pool.Exec(ctx,
		"UPDATE srams.users SET backup_codes = $1, updated_at = NOW() WHERE id = $2",
		codes, userID,
	)
	return err
}

// Security Functions

// IncrementFailedLoginAttempts increments failed login counter
func (s *UserService) IncrementFailedLoginAttempts(ctx context.Context, userID uuid.UUID, maxAttempts, lockoutMinutes int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE srams.users SET 
			failed_login_attempts = failed_login_attempts + 1,
			locked_until = CASE 
				WHEN failed_login_attempts + 1 >= $2 
				THEN NOW() + INTERVAL '1 minute' * $3
				ELSE locked_until 
			END,
			updated_at = NOW()
		WHERE id = $1
	`, userID, maxAttempts, lockoutMinutes)
	return err
}

// ResetFailedLoginAttempts resets the counter on successful login
func (s *UserService) ResetFailedLoginAttempts(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE srams.users 
		SET failed_login_attempts = 0, locked_until = NULL, updated_at = NOW() 
		WHERE id = $1
	`, userID)
	return err
}

// CreateSuperAdmin creates the initial Super Admin (only if none exists)
func (s *UserService) CreateSuperAdmin(ctx context.Context, email, password, fullName, mobile string) (*models.User, error) {
	// Check if super admin already exists
	var count int
	err := s.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM srams.users WHERE role = $1",
		models.RoleSuperAdmin,
	).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to check super admin: %w", err)
	}
	if count > 0 {
		return nil, errors.New("super admin already exists")
	}

	// Hash password
	passwordHash, err := s.authService.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		ID:       uuid.New(),
		Email:    email,
		FullName: fullName,
		Mobile:   mobile,
		Role:     models.RoleSuperAdmin,
		IsActive: true,
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO srams.users (
			id, email, password_hash, full_name, mobile, role, is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, user.ID, user.Email, passwordHash, user.FullName, user.Mobile, user.Role, true)

	if err != nil {
		return nil, fmt.Errorf("failed to create super admin: %w", err)
	}

	return user, nil
}

// GetDashboardStats returns statistics for the dashboard
func (s *UserService) GetDashboardStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total users
	var totalUsers int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM srams.users").Scan(&totalUsers)
	if err != nil {
		return nil, err
	}
	stats["total_users"] = totalUsers

	// Active users
	var activeUsers int
	err = s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM srams.users WHERE is_active = true").Scan(&activeUsers)
	if err != nil {
		return nil, err
	}
	stats["active_users"] = activeUsers

	// Users by role
	rows, err := s.pool.Query(ctx, `
		SELECT role, COUNT(*) FROM srams.users WHERE is_active = true GROUP BY role
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roleStats := make(map[string]int)
	for rows.Next() {
		var role string
		var count int
		if err := rows.Scan(&role, &count); err != nil {
			return nil, err
		}
		roleStats[role] = count
	}
	stats["users_by_role"] = roleStats

	// Total documents
	var totalDocs int
	err = s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM srams.documents WHERE is_active = true").Scan(&totalDocs)
	if err != nil {
		return nil, err
	}
	stats["total_documents"] = totalDocs

	// Active sessions
	var activeSessions int
	err = s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM auth.sessions 
		WHERE is_active = true AND expires_at > NOW()
	`).Scan(&activeSessions)
	if err != nil {
		return nil, err
	}
	stats["active_sessions"] = activeSessions

	return stats, nil
}

// GetActiveSessions returns all active sessions for a user
func (s *UserService) GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*models.Session, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, ip_address, device_fingerprint, user_agent, 
			created_at, last_activity, expires_at, is_active
		FROM auth.sessions 
		WHERE user_id = $1 AND is_active = true AND expires_at > NOW()
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*models.Session
	for rows.Next() {
		session := &models.Session{}
		var lastActivity sql.NullTime
		var ipAddress sql.NullString
		err := rows.Scan(
			&session.ID, &session.UserID, &ipAddress, &session.DeviceFingerprint,
			&session.UserAgent, &session.CreatedAt, &lastActivity, &session.ExpiresAt, &session.IsActive,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		if lastActivity.Valid {
			session.LastActivity = lastActivity.Time
		}
		if ipAddress.Valid {
			session.IPAddress = ipAddress.String
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// VerifyDeviceCertificate verifies or registers a device certificate for Super Admin
// Implements Trust-On-First-Use (TOFU): Registers the first device seen for the user
func (s *UserService) VerifyDeviceCertificate(ctx context.Context, userID uuid.UUID, fingerprint, deviceID string) (bool, error) {
	if fingerprint == "" {
		return false, errors.New("device fingerprint is required for super admin login")
	}

	// Check if any device certificate exists for this user
	var count int
	err := s.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM auth.device_certificates WHERE user_id = $1 AND revoked_at IS NULL",
		userID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check device certificates: %w", err)
	}

	// TOFU: If no certificate exists, register this one
	if count == 0 {
		machineID := deviceID
		if machineID == "" {
			machineID = fingerprint[:min(64, len(fingerprint))] // Use fingerprint prefix as machine ID
		}

		_, err = s.pool.Exec(ctx, `
			INSERT INTO auth.device_certificates (user_id, fingerprint, machine_id, os_info)
			VALUES ($1, $2, $3, $4)
		`, userID, fingerprint, machineID, "Registered via TOFU")
		if err != nil {
			return false, fmt.Errorf("failed to register device certificate: %w", err)
		}
		return true, nil
	}

	// Check if fingerprint matches any registered certificate
	var exists bool
	err = s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM auth.device_certificates 
			WHERE user_id = $1 AND fingerprint = $2 AND revoked_at IS NULL
		)
	`, userID, fingerprint).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to verify device certificate: %w", err)
	}

	if exists {
		// Update last_used_at
		_, err = s.pool.Exec(ctx,
			"UPDATE auth.device_certificates SET last_used_at = NOW() WHERE user_id = $1 AND fingerprint = $2",
			userID, fingerprint,
		)
		if err != nil {
			return false, fmt.Errorf("failed to update device certificate: %w", err)
		}
		return true, nil
	}

	return false, errors.New("device certificate mismatch - login from unregistered device")
}

// Helper function
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
