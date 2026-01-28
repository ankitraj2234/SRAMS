package interfaces

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/srams/backend/internal/models"
)

// UserService defines the contract for user management operations
type UserService interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Create(ctx context.Context, input models.CreateUserInput, createdBy uuid.UUID) (*models.User, error)
	Update(ctx context.Context, id uuid.UUID, input models.UpdateUserInput, updatedBy uuid.UUID, updaterRole string) (*models.User, error)
	Delete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error
	ChangePassword(ctx context.Context, id uuid.UUID, oldPassword, newPassword string) error

	// Session Management
	CreateSession(ctx context.Context, sessionID, userID uuid.UUID, tokenHash, refreshTokenHash, ipAddress, deviceFingerprint, userAgent string, expiresAt time.Time) (*models.Session, error)
	InvalidateSession(ctx context.Context, sessionID uuid.UUID) error
	InvalidateAllSessions(ctx context.Context, userID uuid.UUID) error
	InvalidateAllSessionsExcept(ctx context.Context, userID uuid.UUID, exceptSessionID uuid.UUID) error
	GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*models.Session, error)
	UpdateSessionActivity(ctx context.Context, sessionID uuid.UUID) error
	IsSessionValid(ctx context.Context, sessionID uuid.UUID) (bool, error)
	RotateRefreshToken(ctx context.Context, sessionID uuid.UUID, newTokenHash string) error
	ValidateRefreshTokenHash(ctx context.Context, sessionID uuid.UUID, hash string) (bool, *models.Session, error)
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error

	// Admin
	List(ctx context.Context, role string, offset, limit int) ([]*models.User, int, error)
	CreateSuperAdmin(ctx context.Context, email, password, fullName, mobile string) (*models.User, error)
	GetDashboardStats(ctx context.Context) (map[string]interface{}, error)

	// Security
	IncrementFailedLoginAttempts(ctx context.Context, userID uuid.UUID, maxAttempts, lockoutMinutes int) error
	ResetFailedLoginAttempts(ctx context.Context, userID uuid.UUID) error
	VerifyDeviceCertificate(ctx context.Context, userID uuid.UUID, fingerprint, deviceID string) (bool, error)

	// MFA
	SetTOTPSecret(ctx context.Context, userID uuid.UUID, secret string) error
	EnableTOTP(ctx context.Context, userID uuid.UUID) error
	DisableTOTP(ctx context.Context, userID uuid.UUID) error
	StoreBackupCodes(ctx context.Context, userID uuid.UUID, codes []string) error
}

// AuditService defines the contract for audit logging operations
type AuditService interface {
	Create(ctx context.Context, input models.CreateAuditLogInput) (*models.AuditLog, error)
	List(ctx context.Context, filter models.AuditLogFilter) ([]*models.AuditLog, int, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error)
	Delete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID, reason string, role string) error
	BulkDelete(ctx context.Context, ids []uuid.UUID, deletedBy uuid.UUID, reason string, role string) (int64, error)

	// Helper methods
	LogLogin(ctx context.Context, userID uuid.UUID, role, ip, deviceID, userAgent string, success bool) error
	LogLogout(ctx context.Context, userID uuid.UUID, role, ip, deviceID, userAgent string) error

	// Stats
	GetActionTypes(ctx context.Context) ([]string, error)
	GetStats(ctx context.Context, period string) (map[string]interface{}, error) // period: "day", "week", "month"
	GetStatsByAction(ctx context.Context, since time.Time) (map[string]interface{}, error)
	GetUserTimeline(ctx context.Context, userID uuid.UUID, limit int) ([]*models.AuditLog, error)
	GetUserActivityTimeline(ctx context.Context, userID uuid.UUID, days int) ([]map[string]interface{}, error)
}

// DocumentService defines the contract for document operations
type DocumentService interface {
	Upload(ctx context.Context, input models.UploadDocumentInput, uploadedBy uuid.UUID) (*models.Document, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Document, error)
	List(ctx context.Context, offset, limit int) ([]*models.Document, int, error)
	Delete(ctx context.Context, id uuid.UUID) error
	GetUserDocuments(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*models.Document, int, error)

	// Access Control
	GrantAccess(ctx context.Context, documentID, userID, grantedBy uuid.UUID, canReassign bool) error
	RevokeAccess(ctx context.Context, documentID, userID, revokedBy uuid.UUID) error
	HasAccess(ctx context.Context, documentID, userID uuid.UUID) (bool, error)
	CanReassign(ctx context.Context, documentID, userID uuid.UUID) (bool, error)
	GetDocumentAccessList(ctx context.Context, documentID uuid.UUID) ([]models.DocumentAccessUser, error)

	// Requests
	CreateRequest(ctx context.Context, userID uuid.UUID, documentID *uuid.UUID, documentName, reason string) (*models.UserRequest, error)
	GetPendingRequests(ctx context.Context, offset, limit int) ([]*models.UserRequest, int, error)
	ApproveRequest(ctx context.Context, requestID, reviewedBy uuid.UUID, note string) error
	RejectRequest(ctx context.Context, requestID, reviewedBy uuid.UUID, note string) error
	GetUserRequests(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*models.UserRequest, int, error)

	// Viewing
	StartView(ctx context.Context, documentID, userID uuid.UUID) (uuid.UUID, error)
	EndView(ctx context.Context, viewID uuid.UUID, totalSeconds int) error
	RecordPageView(ctx context.Context, viewID uuid.UUID, pageNumber int) error
	GetFileContent(ctx context.Context, id uuid.UUID) (io.ReadCloser, error)
}

// SystemService defines the contract for system configuration and monitoring
type SystemService interface {
	GetHealth(ctx context.Context) (map[string]interface{}, error)
	GetServerStatus(ctx context.Context) (map[string]interface{}, error)

	// Desktop Sessions
	CreateDesktopSession(ctx context.Context, ip string) (string, error)
	EndDesktopSession(ctx context.Context, token string) error
	ValidateDesktopSession(ctx context.Context, token string) (bool, error)

	// Config
	GetConfig(ctx context.Context) (map[string]string, error)
	SetConfig(ctx context.Context, key, value string) error
	GetAllConfigs(ctx context.Context) (map[string]string, error)

	// Analytics
	GetDatabaseStats(ctx context.Context) (map[string]interface{}, error)
	GetSessionAnalytics(ctx context.Context) (map[string]interface{}, error)
	CleanupExpiredSessions(ctx context.Context) (int64, error)

	// Logo
	SaveLogo(ctx context.Context, content io.Reader, filename string, size int64) error
	GetLogoPath(ctx context.Context) (string, error)
	DeleteLogo(ctx context.Context) error
}
