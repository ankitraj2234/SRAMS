package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Role constants
const (
	RoleSuperAdmin = "super_admin"
	RoleAdmin      = "admin"
	RoleUser       = "user"
)

// User represents a system user
type User struct {
	ID                  uuid.UUID  `json:"id" db:"id"`
	Email               string     `json:"email" db:"email"`
	PasswordHash        string     `json:"-" db:"password_hash"`
	FullName            string     `json:"full_name" db:"full_name"`
	Mobile              string     `json:"mobile" db:"mobile"`
	Role                string     `json:"role" db:"role"`
	IsActive            bool       `json:"is_active" db:"is_active"`
	CreatedAt           time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at" db:"updated_at"`
	LastLogin           *time.Time `json:"last_login,omitempty" db:"last_login"`
	TOTPSecret          *string    `json:"-" db:"totp_secret"`
	TOTPEnabled         bool       `json:"totp_enabled" db:"totp_enabled"`
	BackupCodes         []string   `json:"-" db:"backup_codes"` // Stored as array, not exposed in profile
	FailedLoginAttempts int        `json:"-" db:"failed_login_attempts"`
	LockedUntil         *time.Time `json:"locked_until,omitempty" db:"locked_until"`
	// Phase 8: Enterprise Security Fields
	MustChangePassword bool `json:"must_change_password" db:"must_change_password"`
	MustEnrollMFA      bool `json:"must_enroll_mfa" db:"must_enroll_mfa"`
}

// Session represents an active user session
type Session struct {
	ID                uuid.UUID `json:"id" db:"id"`
	UserID            uuid.UUID `json:"user_id" db:"user_id"`
	TokenHash         string    `json:"-" db:"token_hash"`
	RefreshTokenHash  string    `json:"-" db:"refresh_token_hash"`
	IPAddress         string    `json:"ip_address" db:"ip_address"`
	DeviceFingerprint string    `json:"device_fingerprint" db:"device_fingerprint"`
	UserAgent         string    `json:"user_agent" db:"user_agent"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	ExpiresAt         time.Time `json:"expires_at" db:"expires_at"`
	LastActivity      time.Time `json:"last_activity" db:"last_activity"`
	IsActive          bool      `json:"is_active" db:"is_active"`
}

// Document represents a PDF document in the system
type Document struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Title       string    `json:"title" db:"title"`
	Filename    string    `json:"filename" db:"filename"`
	FilePath    string    `json:"-" db:"file_path"`
	FileHash    string    `json:"file_hash" db:"file_hash"`
	FileSize    int64     `json:"file_size" db:"file_size"`
	UploadedBy  uuid.UUID `json:"uploaded_by" db:"uploaded_by"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	CanReassign bool      `json:"can_reassign,omitempty" db:"-"` // Computed field for permissions
}

// DocumentAccess represents document access grants
type DocumentAccess struct {
	ID                 uuid.UUID  `json:"id" db:"id"`
	DocumentID         uuid.UUID  `json:"document_id" db:"document_id"`
	UserID             uuid.UUID  `json:"user_id" db:"user_id"`
	GrantedBy          uuid.UUID  `json:"granted_by" db:"granted_by"`
	GrantedAt          time.Time  `json:"granted_at" db:"granted_at"`
	RevokedAt          *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
	IsActive           bool       `json:"is_active" db:"is_active"`
	CanReassign        bool       `json:"can_reassign" db:"can_reassign"`
	LockedBySuperAdmin bool       `json:"locked_by_super_admin" db:"locked_by_super_admin"`
}

// RequestStatus constants
const (
	RequestStatusPending  = "pending"
	RequestStatusApproved = "approved"
	RequestStatusRejected = "rejected"
)

// UserRequest represents a document access request
type UserRequest struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	UserID       uuid.UUID  `json:"user_id" db:"user_id"`
	DocumentID   *uuid.UUID `json:"document_id,omitempty" db:"document_id"`
	DocumentName string     `json:"document_name,omitempty" db:"document_name"`
	RequestType  string     `json:"request_type" db:"request_type"`
	Status       string     `json:"status" db:"status"`
	Reason       string     `json:"reason" db:"reason"`
	ReviewedBy   *uuid.UUID `json:"reviewed_by,omitempty" db:"reviewed_by"`
	ReviewedAt   *time.Time `json:"reviewed_at,omitempty" db:"reviewed_at"`
	ReviewNote   *string    `json:"review_note,omitempty" db:"review_note"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
}

// AuditLog action types
const (
	ActionLogin          = "login"
	ActionLogout         = "logout"
	ActionLoginFailed    = "login_failed"
	ActionPasswordChange = "password_change"
	ActionSessionTimeout = "session_timeout"
	ActionPageView       = "page_view"
	ActionButtonClick    = "button_click"
	ActionDocumentOpen   = "document_open"
	ActionDocumentClose  = "document_close"
	ActionDocumentPage   = "document_page_view"
	ActionAdminApproval  = "admin_approval"
	ActionRoleChange     = "role_change"
	ActionConfigChange   = "config_change"
	ActionUserCreate     = "user_create"
	ActionUserUpdate     = "user_update"
	ActionUserDelete     = "user_delete"
	ActionDocUpload      = "document_upload"
	ActionDocDelete      = "document_delete"
	ActionAccessGrant    = "access_grant"
	ActionAccessRevoke   = "access_revoke"
	ActionLogDelete      = "log_delete"
)

// AuditLog represents an audit log entry (append-only)
type AuditLog struct {
	ID             uuid.UUID       `json:"id" db:"id"`
	ActorID        *uuid.UUID      `json:"actor_id,omitempty" db:"actor_id"`
	ActorRole      string          `json:"actor_role" db:"actor_role"`
	ActionType     string          `json:"action_type" db:"action_type"`
	TargetType     string          `json:"target_type" db:"target_type"`
	TargetID       *uuid.UUID      `json:"target_id,omitempty" db:"target_id"`
	Metadata       json.RawMessage `json:"metadata" db:"metadata"`
	IPAddress      string          `json:"ip_address" db:"ip_address"`
	DeviceID       string          `json:"device_id" db:"device_id"`
	UserAgent      string          `json:"user_agent" db:"user_agent"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	DeletedAt      *time.Time      `json:"deleted_at,omitempty" db:"deleted_at"`
	DeletedBy      *uuid.UUID      `json:"deleted_by,omitempty" db:"deleted_by"`
	DeletionReason *string         `json:"deletion_reason,omitempty" db:"deletion_reason"`
}

// AdminAction represents admin-specific actions
type AdminAction struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	AdminID      uuid.UUID  `json:"admin_id" db:"admin_id"`
	ActionType   string     `json:"action_type" db:"action_type"`
	TargetUserID *uuid.UUID `json:"target_user_id,omitempty" db:"target_user_id"`
	Details      string     `json:"details" db:"details"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
}

// SystemConfig represents system configuration
type SystemConfig struct {
	Key       string    `json:"key" db:"key"`
	Value     string    `json:"value" db:"value"`
	UpdatedBy uuid.UUID `json:"updated_by" db:"updated_by"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// DocumentView tracks document viewing sessions
type DocumentView struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	DocumentID   uuid.UUID  `json:"document_id" db:"document_id"`
	UserID       uuid.UUID  `json:"user_id" db:"user_id"`
	StartedAt    time.Time  `json:"started_at" db:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty" db:"ended_at"`
	PagesViewed  []int      `json:"pages_viewed" db:"pages_viewed"`
	TotalSeconds int        `json:"total_seconds" db:"total_seconds"`
}
