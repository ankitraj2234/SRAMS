package models

import (
	"io"
	"time"

	"github.com/google/uuid"
)

// User Service Inputs

type CreateUserInput struct {
	Email              string `json:"email" binding:"required,email"`
	Password           string `json:"password" binding:"required,min=8"`
	FullName           string `json:"full_name" binding:"required"`
	Mobile             string `json:"mobile"`
	Role               string `json:"role" binding:"required"`
	MustChangePassword bool   `json:"must_change_password"`
	MustEnrollMFA      bool   `json:"must_enroll_mfa"`
}

type UpdateUserInput struct {
	FullName *string `json:"full_name"`
	Mobile   *string `json:"mobile"`
	Role     *string `json:"role"`
	IsActive *bool   `json:"is_active"`
}

// Audit Service Inputs

type CreateAuditLogInput struct {
	ActorID    *uuid.UUID
	ActorRole  string
	ActionType string
	TargetType string
	TargetID   *uuid.UUID
	Metadata   map[string]interface{}
	IPAddress  string
	DeviceID   string
	UserAgent  string
}

type AuditLogFilter struct {
	ActorID        *uuid.UUID
	UserID         *uuid.UUID // often alias for ActorID or specific user context
	ActionType     string
	TargetType     string
	TargetID       *uuid.UUID
	StartDate      *time.Time
	EndDate        *time.Time
	IncludeDeleted bool
	Offset         int
	Limit          int
}

// Document Service Inputs

type UploadDocumentInput struct {
	Title    string
	Filename string
	Content  io.Reader
	FileSize int64
}

// Document Access User DTO
type DocumentAccessUser struct {
	ID             uuid.UUID `json:"id"`
	FullName       string    `json:"full_name"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	GrantedByName  string    `json:"granted_by_name,omitempty"`
	GrantedByEmail string    `json:"granted_by_email,omitempty"`
	GrantedByRole  string    `json:"granted_by_role,omitempty"`
	CanReassign    bool      `json:"can_reassign"`
}
