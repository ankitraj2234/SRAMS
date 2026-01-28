package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/srams/backend/internal/auth"
	"github.com/srams/backend/internal/interfaces"
	"github.com/srams/backend/internal/middleware"
	"github.com/srams/backend/internal/models"
)

type AuthHandler struct {
	userService  interfaces.UserService
	auditService interfaces.AuditService
	authService  *auth.Service
}

func NewAuthHandler(userService interfaces.UserService, auditService interfaces.AuditService, authService *auth.Service) *AuthHandler {
	return &AuthHandler{
		userService:  userService,
		auditService: auditService,
		authService:  authService,
	}
}

type LoginRequest struct {
	Email             string `json:"email" binding:"required,email"`
	Password          string `json:"password" binding:"required"`
	TOTPCode          string `json:"totp_code"`
	DeviceID          string `json:"device_id"`
	DeviceFingerprint string `json:"device_fingerprint"`
}

type LoginResponse struct {
	User                   *models.User `json:"user"`
	AccessToken            string       `json:"access_token"`
	RefreshToken           string       `json:"refresh_token"`
	ExpiresAt              time.Time    `json:"expires_at"`
	RequiresTOTP           bool         `json:"requires_totp,omitempty"`
	RequiresPasswordChange bool         `json:"requires_password_change,omitempty"`
	RequiresMFAEnrollment  bool         `json:"requires_mfa_enrollment,omitempty"`
	ClientIP               string       `json:"client_ip"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	meta := middleware.GetRequestMetadata(c)

	// Get user
	user, err := h.userService.GetByEmail(c.Request.Context(), req.Email)
	if err != nil {
		// Log failed attempt (unknown user)
		h.auditService.LogLogin(c.Request.Context(), uuid.Nil, "", meta.IP, meta.DeviceID, meta.UserAgent, false)
		// Use same error message to prevent user enumeration
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// P0 FIX: Check if account is locked due to brute-force attempts
	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
			ActorID:    &user.ID,
			ActorRole:  user.Role,
			ActionType: "login_blocked_locked",
			TargetType: "user",
			TargetID:   &user.ID,
			Metadata:   map[string]interface{}{"reason": "account_locked", "locked_until": user.LockedUntil},
			IPAddress:  meta.IP,
			DeviceID:   meta.DeviceID,
			UserAgent:  meta.UserAgent,
		})
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":        "Account temporarily locked due to too many failed attempts",
			"locked_until": user.LockedUntil,
		})
		return
	}

	// Check if user is active
	if !user.IsActive {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Account is disabled"})
		return
	}

	// Verify password
	if !h.authService.VerifyPassword(req.Password, user.PasswordHash) {
		// P0 FIX: Increment failed login attempts
		h.userService.IncrementFailedLoginAttempts(c.Request.Context(), user.ID, 5, 15)
		h.auditService.LogLogin(c.Request.Context(), user.ID, user.Role, meta.IP, meta.DeviceID, meta.UserAgent, false)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Check TOTP if enabled
	if user.TOTPEnabled {
		if req.TOTPCode == "" {
			c.JSON(http.StatusOK, LoginResponse{RequiresTOTP: true})
			return
		}
		if user.TOTPSecret == nil || !h.authService.ValidateTOTP(*user.TOTPSecret, req.TOTPCode) {
			// P0 FIX: Also increment on TOTP failure
			h.userService.IncrementFailedLoginAttempts(c.Request.Context(), user.ID, 5, 15)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid TOTP code"})
			return
		}
	}

	// SECURITY: Role-based login restrictions
	// IsServerMachine checks if client is on the same machine (localhost, ::1, or server's LAN IP)

	// Security Check: Super Admin Restrictions
	// Super Admin: MUST be on server machine (localhost/server IP) AND desktop session must be active
	// This prevents remote takeover even if credentials are compromised
	if user.Role == models.RoleSuperAdmin {
		isServerMachine := middleware.IsServerMachine(meta.IP)

		if !isServerMachine {
			h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
				ActorID:    &user.ID,
				ActorRole:  user.Role,
				ActionType: "login_blocked_superadmin_not_localhost",
				TargetType: "user",
				TargetID:   &user.ID,
				Metadata:   map[string]interface{}{"ip": meta.IP, "reason": "superadmin_requires_localhost"},
				IPAddress:  meta.IP,
				DeviceID:   meta.DeviceID,
				UserAgent:  meta.UserAgent,
			})
			c.JSON(http.StatusForbidden, gin.H{"error": "Login restricted: Super Admin requires local server access"})
			return
		}
		// RELAXED: Desktop session check removed to allow recovery after server restart
		// if !middleware.GetDesktopSession().HasActiveSession() {
		// 	h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
		// 		ActorID:    &user.ID,
		// 		ActorRole:  user.Role,
		// 		ActionType: "login_blocked_superadmin_no_desktop_session",
		// 		TargetType: "user",
		// 		TargetID:   &user.ID,
		// 		Metadata:   map[string]interface{}{"reason": "superadmin_requires_desktop_app"},
		// 		IPAddress:  meta.IP,
		// 		DeviceID:   meta.DeviceID,
		// 		UserAgent:  meta.UserAgent,
		// 	})
		// 	c.JSON(http.StatusForbidden, gin.H{"error": "Login restricted"})
		// 	return
		// }

		// DEVICE CERTIFICATE VERIFICATION
		// Super Admin must use registered device (Trust-On-First-Use)
		if req.DeviceFingerprint != "" {
			valid, err := h.userService.VerifyDeviceCertificate(c.Request.Context(), user.ID, req.DeviceFingerprint, req.DeviceID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Device verification failed"})
				return
			}
			if !valid {
				h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
					ActorID:    &user.ID,
					ActorRole:  user.Role,
					ActionType: "login_blocked_device_mismatch",
					TargetType: "user",
					TargetID:   &user.ID,
					Metadata:   map[string]interface{}{"reason": "device_fingerprint_mismatch", "fingerprint": req.DeviceFingerprint},
					IPAddress:  meta.IP,
					DeviceID:   meta.DeviceID,
					UserAgent:  meta.UserAgent,
				})
				c.JSON(http.StatusForbidden, gin.H{"error": "Device not authorized"})
				return
			}
		}
	}

	// Admin: No IP restrictions (allowed from any machine)
	// Regular User: No IP restrictions (allowed from any machine)

	// Regular User: Can login from any local network IP (already filtered by middleware)

	// P0 FIX: Reset failed login attempts on successful login
	h.userService.ResetFailedLoginAttempts(c.Request.Context(), user.ID)

	// Create session with IP/device binding for session validation
	sessionID := uuid.New()
	tokenPair, err := h.authService.GenerateTokenPair(user, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	// Store session with bound IP and device
	_, err = h.userService.CreateSession(
		c.Request.Context(),
		sessionID,
		user.ID,
		h.authService.HashToken(tokenPair.AccessToken),
		h.authService.HashToken(tokenPair.RefreshToken),
		meta.IP,
		req.DeviceID,
		meta.UserAgent,
		tokenPair.ExpiresAt.Add(7*24*time.Hour), // Refresh token expiry
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	// Update last login
	h.userService.UpdateLastLogin(c.Request.Context(), user.ID)

	// Log successful login
	h.auditService.LogLogin(c.Request.Context(), user.ID, user.Role, meta.IP, meta.DeviceID, meta.UserAgent, true)

	// Set CSRF cookie - HttpOnly must be false so JavaScript can read it
	// Secure is false in development (HTTP), should be true in production (HTTPS)
	csrfToken, _ := h.authService.GenerateCSRFToken()
	c.SetCookie("csrf_token", csrfToken, 3600*24, "/", "", false, false)

	c.JSON(http.StatusOK, LoginResponse{
		User:                   user,
		AccessToken:            tokenPair.AccessToken,
		RefreshToken:           tokenPair.RefreshToken,
		ExpiresAt:              tokenPair.ExpiresAt,
		RequiresPasswordChange: user.MustChangePassword,
		RequiresMFAEnrollment:  user.MustEnrollMFA && !user.TOTPEnabled,
		ClientIP:               meta.IP,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	claims := middleware.GetClaims(c)
	user := middleware.GetUser(c)
	meta := middleware.GetRequestMetadata(c)

	if claims != nil {
		h.userService.InvalidateSession(c.Request.Context(), claims.SessionID)
	}

	if user != nil {
		h.auditService.LogLogout(c.Request.Context(), user.ID, user.Role, meta.IP, meta.DeviceID, meta.UserAgent)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	meta := middleware.GetRequestMetadata(c)

	claims, err := h.authService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	// P0 FIX: Verify the refresh token hash matches stored hash (prevents replay)
	providedTokenHash := h.authService.HashToken(req.RefreshToken)
	valid, session, err := h.userService.ValidateRefreshTokenHash(c.Request.Context(), claims.SessionID, providedTokenHash)
	if err != nil || !valid {
		// P0 FIX: Token reuse detected - invalidate entire session family (potential theft)
		if session != nil {
			h.userService.InvalidateSession(c.Request.Context(), claims.SessionID)
			h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
				ActorID:    &claims.UserID,
				ActorRole:  "system",
				ActionType: "refresh_token_reuse_detected",
				TargetType: "session",
				TargetID:   &claims.SessionID,
				Metadata:   map[string]interface{}{"reason": "potential_token_theft"},
				IPAddress:  meta.IP,
				DeviceID:   meta.DeviceID,
				UserAgent:  meta.UserAgent,
			})
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired or token reused"})
		return
	}

	// P1 FIX: Validate IP/device binding (allow some tolerance for mobile networks)
	if session != nil && session.IPAddress != meta.IP {
		h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
			ActorID:    &claims.UserID,
			ActorRole:  "system",
			ActionType: "session_ip_mismatch",
			TargetType: "session",
			TargetID:   &claims.SessionID,
			Metadata:   map[string]interface{}{"original_ip": session.IPAddress, "current_ip": meta.IP},
			IPAddress:  meta.IP,
			DeviceID:   meta.DeviceID,
			UserAgent:  meta.UserAgent,
		})
		// For now, log but allow - could be mobile network. Device fingerprint is more reliable.
	}

	// P1 FIX: Device fingerprint MUST match
	// RELAXED: Device fingerprint check removed to allow browser usage (which doesn't share fingerprint with desktop)
	/*
		if session != nil && session.DeviceFingerprint != "" && session.DeviceFingerprint != meta.DeviceID {
			h.userService.InvalidateSession(c.Request.Context(), claims.SessionID)
			h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
				ActorID:    &claims.UserID,
				ActorRole:  "system",
				ActionType: "session_device_mismatch",
				TargetType: "session",
				TargetID:   &claims.SessionID,
				Metadata:   map[string]interface{}{"original_device": session.DeviceFingerprint, "current_device": meta.DeviceID, "action": "session_revoked"},
				IPAddress:  meta.IP,
				DeviceID:   meta.DeviceID,
				UserAgent:  meta.UserAgent,
			})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session invalidated due to device mismatch"})
			return
		}
	*/

	// Get user
	user, err := h.userService.GetByID(c.Request.Context(), claims.UserID)
	if err != nil || !user.IsActive {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	// Generate new token pair (session ID remains same for tracking)
	tokenPair, err := h.authService.GenerateTokenPair(user, claims.SessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	// P0 FIX: Update session with new refresh token hash (rotate the token)
	err = h.userService.RotateRefreshToken(c.Request.Context(), claims.SessionID, h.authService.HashToken(tokenPair.RefreshToken))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to rotate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"expires_at":    tokenPair.ExpiresAt,
	})
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	user := middleware.GetUser(c)
	meta := middleware.GetRequestMetadata(c)

	err := h.userService.ChangePassword(c.Request.Context(), user.ID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		if err == auth.ErrInvalidCredentials {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Current password is incorrect"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change password"})
		return
	}

	// Log password change
	h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
		ActorID:    &user.ID,
		ActorRole:  user.Role,
		ActionType: models.ActionPasswordChange,
		TargetType: "user",
		TargetID:   &user.ID,
		IPAddress:  meta.IP,
		DeviceID:   meta.DeviceID,
		UserAgent:  meta.UserAgent,
	})

	// Invalidate all other sessions (keep current one active)
	claims := middleware.GetClaims(c)
	if claims != nil {
		h.userService.InvalidateAllSessionsExcept(c.Request.Context(), user.ID, claims.SessionID)
	} else {
		h.userService.InvalidateAllSessions(c.Request.Context(), user.ID)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

func (h *AuthHandler) GetProfile(c *gin.Context) {
	user := middleware.GetUser(c)
	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) GetSessions(c *gin.Context) {
	user := middleware.GetUser(c)
	sessions, err := h.userService.GetActiveSessions(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get sessions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// TOTP endpoints
func (h *AuthHandler) SetupTOTP(c *gin.Context) {
	user := middleware.GetUser(c)

	if user.TOTPEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "TOTP already enabled"})
		return
	}

	secret, url, err := h.authService.GenerateTOTPSecret(user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate TOTP"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"secret": secret,
		"qr_url": url,
		"email":  user.Email,
	})
}

type EnableTOTPRequest struct {
	Secret string `json:"secret" binding:"required"`
	Code   string `json:"code" binding:"required"`
}

func (h *AuthHandler) EnableTOTP(c *gin.Context) {
	var req EnableTOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	user := middleware.GetUser(c)

	// Verify code
	if !h.authService.ValidateTOTP(req.Secret, req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid code"})
		return
	}

	// Generate backup codes
	backupCodes, err := h.authService.GenerateBackupCodes(10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate backup codes"})
		return
	}

	// Save TOTP secret first, then enable
	if err := h.userService.SetTOTPSecret(c.Request.Context(), user.ID, req.Secret); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save TOTP secret"})
		return
	}

	// Store backup codes
	if err := h.userService.StoreBackupCodes(c.Request.Context(), user.ID, backupCodes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store backup codes"})
		return
	}

	err = h.userService.EnableTOTP(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable TOTP"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "TOTP enabled",
		"backup_codes": backupCodes,
	})
}

func (h *AuthHandler) DisableTOTP(c *gin.Context) {
	user := middleware.GetUser(c)

	err := h.userService.DisableTOTP(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable TOTP"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "TOTP disabled"})
}
