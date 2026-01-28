package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/srams/backend/internal/auth"
	"github.com/srams/backend/internal/models"
)

type contextKey string

const (
	UserContextKey    contextKey = "user"
	SessionContextKey contextKey = "session"
	ClaimsContextKey  contextKey = "claims"
)

// RateLimiter manages per-IP rate limiting
type RateLimiter struct {
	visitors map[string]*rate.Limiter
	mu       sync.RWMutex
	r        rate.Limit
	b        int
}

func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*rate.Limiter),
		r:        r,
		b:        b,
	}

	// Cleanup old entries every minute
	go func() {
		for range time.Tick(time.Minute) {
			rl.mu.Lock()
			rl.visitors = make(map[string]*rate.Limiter)
			rl.mu.Unlock()
		}
	}()

	return rl
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.visitors[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.r, rl.b)
		rl.visitors[ip] = limiter
	}

	return limiter
}

// RateLimit middleware
func (rl *RateLimiter) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := rl.getLimiter(ip)

		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please try again later.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AuthMiddleware validates JWT tokens and sets user context
type AuthMiddleware struct {
	authService *auth.Service
	db          SessionChecker
}

type SessionChecker interface {
	IsSessionValid(ctx context.Context, sessionID uuid.UUID) (bool, error)
	UpdateSessionActivity(ctx context.Context, sessionID uuid.UUID) error
	GetByID(ctx context.Context, userID uuid.UUID) (*models.User, error)
}

func NewAuthMiddleware(authService *auth.Service, db SessionChecker) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
		db:          db,
	}
}

// CORSMiddleware adds CORS headers
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Device-ID, X-Desktop-Session")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "X-View-ID")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// RequestID adds a unique ID to the request context and headers
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("RequestID", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Next()
	}
}

// Logger provides a simple logger middleware (wrapping Gin's or custom)
func Logger() gin.HandlerFunc {
	return gin.Logger()
}

func (m *AuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := ""
		authHeader := c.GetHeader("Authorization")

		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString = parts[1]
			}
		}

		// Fallback to query parameter (needed for SSE)
		if tokenString == "" {
			tokenString = c.Query("token")
		}

		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header or token query parameter required"})
			c.Abort()
			return
		}

		claims, err := m.authService.ValidateAccessToken(tokenString)
		if err != nil {
			if err == auth.ErrTokenExpired {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Token expired", "code": "TOKEN_EXPIRED"})
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			}
			c.Abort()
			return
		}

		// Verify session is still active
		valid, err := m.db.IsSessionValid(c.Request.Context(), claims.SessionID)
		if err != nil || !valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired or invalid"})
			c.Abort()
			return
		}

		// Update session activity
		_ = m.db.UpdateSessionActivity(c.Request.Context(), claims.SessionID)

		// Get user
		user, err := m.db.GetByID(c.Request.Context(), claims.UserID)
		if err != nil || !user.IsActive {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found or inactive"})
			c.Abort()
			return
		}

		// Set context values
		c.Set(string(UserContextKey), user)
		c.Set(string(ClaimsContextKey), claims)

		c.Next()
	}
}

// RequireRole middleware ensures user has required role
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get(string(UserContextKey))
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
			c.Abort()
			return
		}

		userModel := user.(*models.User)

		// Check if user has one of the required roles
		hasRole := false
		for _, role := range roles {
			if userModel.Role == role {
				hasRole = true
				break
			}
		}

		// Super admin has access to everything
		if userModel.Role == models.RoleSuperAdmin {
			hasRole = true
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireSuperAdmin middleware ensures user is super admin
func RequireSuperAdmin() gin.HandlerFunc {
	return RequireRole(models.RoleSuperAdmin)
}

// RequireAdmin middleware ensures user is admin or super admin
func RequireAdmin() gin.HandlerFunc {
	return RequireRole(models.RoleAdmin, models.RoleSuperAdmin)
}

// CSRFMiddleware provides CSRF protection
func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip for safe methods
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// Check CSRF token
		csrfToken := c.GetHeader("X-CSRF-Token")
		cookieToken, err := c.Cookie("csrf_token")

		if err != nil || csrfToken == "" || csrfToken != cookieToken {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid CSRF token"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AuditLogger logs all requests for auditing
type AuditLogger struct {
	logFunc func(ctx context.Context, log *AuditLogEntry) error
}

type AuditLogEntry struct {
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

func NewAuditLogger(logFunc func(ctx context.Context, log *AuditLogEntry) error) *AuditLogger {
	return &AuditLogger{logFunc: logFunc}
}

func (a *AuditLogger) LogRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Log after request completes
		entry := &AuditLogEntry{
			ActionType: "api_request",
			TargetType: "endpoint",
			Metadata: map[string]interface{}{
				"method":   c.Request.Method,
				"path":     c.Request.URL.Path,
				"status":   c.Writer.Status(),
				"duration": time.Since(start).Milliseconds(),
				"query":    c.Request.URL.RawQuery,
			},
			IPAddress: c.ClientIP(),
			DeviceID:  c.GetHeader("X-Device-ID"),
			UserAgent: c.Request.UserAgent(),
		}

		// Add user info if authenticated
		if user, exists := c.Get(string(UserContextKey)); exists {
			userModel := user.(*models.User)
			entry.ActorID = &userModel.ID
			entry.ActorRole = userModel.Role
		}

		// Log asynchronously
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = a.logFunc(ctx, entry)
		}()
	}
}

// SecurityHeaders adds security headers to responses
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; object-src 'none'")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		c.Next()
	}
}

// GetUser extracts user from context
func GetUser(c *gin.Context) *models.User {
	if user, exists := c.Get(string(UserContextKey)); exists {
		return user.(*models.User)
	}
	return nil
}

// GetClaims extracts claims from context
func GetClaims(c *gin.Context) *auth.TokenClaims {
	if claims, exists := c.Get(string(ClaimsContextKey)); exists {
		return claims.(*auth.TokenClaims)
	}
	return nil
}

// RequestMetadata captures request info for logging
type RequestMetadata struct {
	IP        string `json:"ip"`
	DeviceID  string `json:"device_id"`
	UserAgent string `json:"user_agent"`
}

func GetRequestMetadata(c *gin.Context) RequestMetadata {
	return RequestMetadata{
		IP:        c.ClientIP(),
		DeviceID:  c.GetHeader("X-Device-ID"),
		UserAgent: c.Request.UserAgent(),
	}
}

// ToJSON converts metadata to JSON
func (m RequestMetadata) ToJSON() json.RawMessage {
	data, _ := json.Marshal(m)
	return data
}

// DesktopSessionManager manages desktop app sessions for Super Admin access gating
type DesktopSessionManager struct {
	activeSession string
	mu            sync.RWMutex
}

var desktopSession = &DesktopSessionManager{}

// CreateSession creates a new desktop session and returns the token
func (d *DesktopSessionManager) CreateSession() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.activeSession = uuid.New().String()
	return d.activeSession
}

// EndSession ends the current desktop session
func (d *DesktopSessionManager) EndSession() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.activeSession = ""
}

// SetSession sets the active session token (used by SystemHandler)
func (d *DesktopSessionManager) SetSession(token string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.activeSession = token
}

// ValidateSession checks if the provided token matches the active session
func (d *DesktopSessionManager) ValidateSession(token string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return token != "" && token == d.activeSession
}

// HasActiveSession checks if there's an active desktop session
func (d *DesktopSessionManager) HasActiveSession() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.activeSession != ""
}

// GetDesktopSession returns the global desktop session manager
func GetDesktopSession() *DesktopSessionManager {
	return desktopSession
}

// LocalhostOnly middleware restricts access to localhost and local network only
func LocalhostOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		// Check if IP is allowed (localhost or private network)
		isAllowed := IsLocalOrPrivateIP(ip)

		if !isAllowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Login restricted",
			})
			return
		}

		c.Next()
	}
}

// IsLocalOrPrivateIP checks if IP is localhost or in private network ranges
func IsLocalOrPrivateIP(ip string) bool {
	// Localhost variations
	if ip == "127.0.0.1" || ip == "::1" || ip == "localhost" || ip == "[::1]" {
		return true
	}

	// Check private network ranges
	// 10.0.0.0 - 10.255.255.255 (Class A)
	if strings.HasPrefix(ip, "10.") {
		return true
	}

	// 192.168.0.0 - 192.168.255.255 (Class C)
	if strings.HasPrefix(ip, "192.168.") {
		return true
	}

	// 172.16.0.0 - 172.31.255.255 (Class B)
	if strings.HasPrefix(ip, "172.") {
		parts := strings.Split(ip, ".")
		if len(parts) >= 2 {
			// Parse second octet
			secondOctet := 0
			for _, ch := range parts[1] {
				if ch >= '0' && ch <= '9' {
					secondOctet = secondOctet*10 + int(ch-'0')
				} else {
					break
				}
			}
			if secondOctet >= 16 && secondOctet <= 31 {
				return true
			}
		}
	}

	return false
}

// DesktopAppGate middleware ensures Super Admin access is gated by desktop app
func DesktopAppGate() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetUser(c)
		if user != nil && user.Role == models.RoleSuperAdmin {
			// Check for desktop app session header
			desktopToken := c.GetHeader("X-Desktop-Session")
			if !desktopSession.ValidateSession(desktopToken) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "Super Admin access requires Desktop App authentication. Please login via the SRAMS Desktop App.",
					"code":  "DESKTOP_APP_REQUIRED",
				})
				return
			}
		}
		c.Next()
	}
}

// serverIPs holds all IP addresses of the server machine
var serverIPs []string
var serverIPsOnce sync.Once

// InitServerIPs initializes the list of server's own IP addresses
func InitServerIPs() {
	serverIPsOnce.Do(func() {
		serverIPs = getLocalIPs()
	})
}

// getLocalIPs returns all IP addresses of the local machine
func getLocalIPs() []string {
	ips := []string{
		"127.0.0.1",
		"::1",
		"localhost",
		"[::1]",
	}

	// Get all network interfaces
	interfaces, err := getNetworkInterfaces()
	if err == nil {
		ips = append(ips, interfaces...)
	}

	return ips
}

// getNetworkInterfaces returns all non-loopback IP addresses
func getNetworkInterfaces() ([]string, error) {
	var ips []string

	// Import net package inline to get interfaces
	// This is a simplified version - in production you'd use net.Interfaces()
	// For now, we'll read the SERVER_IP environment variable which is set during install
	if serverIP := strings.TrimSpace(readEnv("SERVER_IP")); serverIP != "" {
		ips = append(ips, serverIP)
	}

	// Also check HOST environment variable
	if host := strings.TrimSpace(readEnv("HOST")); host != "" && host != "0.0.0.0" && host != "127.0.0.1" {
		ips = append(ips, host)
	}

	return ips, nil
}

// readEnv reads an environment variable
func readEnv(key string) string {
	return os.Getenv(key)
}

// IsServerMachine checks if the given IP address belongs to this server machine
func IsServerMachine(clientIP string) bool {
	// Always allow localhost variants
	if clientIP == "127.0.0.1" || clientIP == "::1" || clientIP == "localhost" || clientIP == "[::1]" {
		return true
	}

	// Check against all known server IPs
	for _, ip := range serverIPs {
		if clientIP == ip {
			return true
		}
	}

	return false
}

// GetServerIPs returns the list of server IP addresses
func GetServerIPs() []string {
	return serverIPs
}
