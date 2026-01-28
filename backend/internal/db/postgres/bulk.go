package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BulkImportUser represents a user to be imported
type BulkImportUser struct {
	Email              string
	PasswordHash       string
	FullName           string
	Mobile             string
	Role               string
	MustChangePassword bool
	MustEnrollMFA      bool
}

// BulkImportError represents an error during bulk import
type BulkImportError struct {
	Row   int
	Email string
	Error string
}

// BulkImportResult contains the results of a bulk import operation
type BulkImportResult struct {
	Imported int
	Failed   int
	Errors   []BulkImportError
}

// BulkImportUsers imports multiple users in a single transaction
// All users succeed or all fail (transactional integrity)
func (db *Database) BulkImportUsers(ctx context.Context, users []BulkImportUser, createdBy uuid.UUID) (*BulkImportResult, error) {
	result := &BulkImportResult{
		Errors: make([]BulkImportError, 0),
	}

	// Use batch for performance
	batch := &pgx.Batch{}

	for _, u := range users {
		batch.Queue(`
			INSERT INTO srams.users (
				email, password_hash, full_name, mobile, role, 
				must_change_password, must_enroll_mfa, created_by
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (email) DO NOTHING
			RETURNING id
		`, u.Email, u.PasswordHash, u.FullName, u.Mobile, u.Role,
			u.MustChangePassword, u.MustEnrollMFA, createdBy)
	}

	// Execute in transaction
	err := db.WithTransaction(ctx, func(tx pgx.Tx) error {
		br := tx.SendBatch(ctx, batch)
		defer br.Close()

		for i, u := range users {
			var id uuid.UUID
			err := br.QueryRow().Scan(&id)
			if err != nil {
				if err == pgx.ErrNoRows {
					// ON CONFLICT DO NOTHING - email already exists
					result.Errors = append(result.Errors, BulkImportError{
						Row:   i + 1,
						Email: u.Email,
						Error: "Email already exists",
					})
					result.Failed++
				} else {
					result.Errors = append(result.Errors, BulkImportError{
						Row:   i + 1,
						Email: u.Email,
						Error: err.Error(),
					})
					result.Failed++
				}
			} else {
				result.Imported++
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// BulkExportUsers exports all users for Excel export
func (db *Database) BulkExportUsers(ctx context.Context) ([]map[string]interface{}, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT 
			email,
			full_name,
			mobile,
			role,
			is_active,
			totp_enabled,
			created_at,
			last_login
		FROM srams.users
		WHERE role != 'super_admin'
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var email, fullName, mobile, role string
		var isActive, totpEnabled bool
		var createdAt, lastLogin interface{}

		err := rows.Scan(&email, &fullName, &mobile, &role, &isActive, &totpEnabled, &createdAt, &lastLogin)
		if err != nil {
			return nil, err
		}

		users = append(users, map[string]interface{}{
			"email":        email,
			"full_name":    fullName,
			"mobile":       mobile,
			"role":         role,
			"is_active":    isActive,
			"totp_enabled": totpEnabled,
			"created_at":   createdAt,
			"last_login":   lastLogin,
		})
	}

	return users, rows.Err()
}

// SessionContextMiddleware is a helper to set session context before each query
type SessionContextMiddleware struct {
	pool *pgxpool.Pool
}

// NewSessionContextMiddleware creates a new session context middleware
func NewSessionContextMiddleware(pool *pgxpool.Pool) *SessionContextMiddleware {
	return &SessionContextMiddleware{pool: pool}
}

// Acquire acquires a connection with session context set
func (m *SessionContextMiddleware) Acquire(ctx context.Context, userID, sessionID uuid.UUID, role string) (*pgxpool.Conn, error) {
	conn, err := m.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	// Set session context
	_, err = conn.Exec(ctx,
		"SELECT srams.set_session_context($1, $2, $3)",
		userID, role, sessionID,
	)
	if err != nil {
		conn.Release()
		return nil, err
	}

	return conn, nil
}
