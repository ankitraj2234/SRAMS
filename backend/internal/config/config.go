package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Known insecure default values that MUST be changed in production
var insecureDefaults = []string{
	"your-super-secret-access-key-change-in-production",
	"your-super-secret-refresh-key-change-in-production",
	"changeme",
	"password",
	"secret",
}

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	Security SecurityConfig
	TLS      TLSConfig
	Mode     string // "development" or "production"
}

type DatabaseConfig struct {
	Type     string // "sqlite" or "postgres"
	SQLite   SQLiteConfig
	Postgres PostgresConfig
}

type PostgresConfig struct {
	Host        string
	Port        int
	Database    string
	User        string
	Password    string
	SSLMode     string
	MaxConns    int
	MinConns    int
	MaxConnLife time.Duration
	MaxConnIdle time.Duration
}

type ServerConfig struct {
	Port         string
	Host         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// SQLiteConfig holds embedded SQLite database configuration
type SQLiteConfig struct {
	// FilePath is the path to the database file
	FilePath string
	// EncryptionKey is the master password for database encryption (AES-256)
	EncryptionKey string
	// MaxSizeMB is the maximum database size in megabytes
	MaxSizeMB int64
	// WALMode enables Write-Ahead Logging for better concurrency
	WALMode bool
	// BusyTimeoutMs is the timeout for busy connections
	BusyTimeoutMs int
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

type SecurityConfig struct {
	RateLimitRequests int
	RateLimitWindow   time.Duration
	SessionTimeout    time.Duration
	MaxLoginAttempts  int
	LockoutDuration   time.Duration
}

type TLSConfig struct {
	Enabled  bool
	CertFile string
	KeyFile  string
}

// ValidationError contains details about configuration problems
type ValidationError struct {
	Field   string
	Message string
	Fatal   bool
}

// ValidationResult contains all validation errors
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []ValidationError
}

func (v *ValidationResult) HasFatalErrors() bool {
	for _, e := range v.Errors {
		if e.Fatal {
			return true
		}
	}
	return false
}

func (v *ValidationResult) String() string {
	var sb strings.Builder
	if len(v.Errors) > 0 {
		sb.WriteString("CONFIGURATION ERRORS:\n")
		for _, e := range v.Errors {
			severity := "ERROR"
			if e.Fatal {
				severity = "FATAL"
			}
			sb.WriteString(fmt.Sprintf("  [%s] %s: %s\n", severity, e.Field, e.Message))
		}
	}
	if len(v.Warnings) > 0 {
		sb.WriteString("CONFIGURATION WARNINGS:\n")
		for _, w := range v.Warnings {
			sb.WriteString(fmt.Sprintf("  [WARN] %s: %s\n", w.Field, w.Message))
		}
	}
	return sb.String()
}

func Load() *Config {
	// Try loading .env from multiple locations (professional Windows pattern)
	envPaths := []string{
		".env",                                 // Current working directory
		"C:\\ProgramData\\SRAMS\\config\\.env", // Professional Windows pattern
		filepath.Join(getExecutableDir(), ".env"),           // Next to executable
		filepath.Join(getExecutableDir(), "config", ".env"), // config/ next to executable
	}

	for _, envPath := range envPaths {
		if _, err := os.Stat(envPath); err == nil {
			_ = godotenv.Load(envPath)
			break
		}
	}

	mode := getEnv("SRAMS_MODE", "development")

	// Determine data directory
	dataDir := getEnv("SRAMS_DATA_DIR", "")
	if dataDir == "" {
		// Default to executable directory / data
		exePath, err := os.Executable()
		if err == nil {
			dataDir = filepath.Join(filepath.Dir(exePath), "data")
		} else {
			dataDir = "./data"
		}
	}

	// Ensure data directory exists
	os.MkdirAll(dataDir, 0700)

	// Defaults for Postgres
	pgPort := 5432
	if p := os.Getenv("DB_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &pgPort)
	}

	return &Config{
		Mode: mode,
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			ReadTimeout:  getDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
		},
		Database: DatabaseConfig{
			Type: getEnv("DB_TYPE", "sqlite"),
			SQLite: SQLiteConfig{
				FilePath:      getEnv("DB_FILE_PATH", filepath.Join(dataDir, "srams.db")),
				EncryptionKey: getEnv("DB_ENCRYPTION_KEY", ""),
				MaxSizeMB:     getEnvInt64("DB_MAX_SIZE_MB", 5120),
				WALMode:       getEnvBool("DB_WAL_MODE", true),
				BusyTimeoutMs: getEnvInt("DB_BUSY_TIMEOUT_MS", 10000),
			},
			Postgres: PostgresConfig{
				Host:        getEnv("DB_HOST", "localhost"),
				Port:        pgPort,
				Database:    getEnv("DB_NAME", "srams"),
				User:        getEnv("DB_USER", "postgres"),
				Password:    getEnv("DB_PASSWORD", "postgres"),
				SSLMode:     getEnv("DB_SSLMODE", "disable"),
				MaxConns:    getEnvInt("DB_MAX_CONNS", 25),
				MinConns:    getEnvInt("DB_MIN_CONNS", 5),
				MaxConnLife: time.Hour,
				MaxConnIdle: 30 * time.Minute,
			},
		},
		JWT: JWTConfig{
			AccessSecret:  getEnv("JWT_ACCESS_SECRET", "your-super-secret-access-key-change-in-production"),
			RefreshSecret: getEnv("JWT_REFRESH_SECRET", "your-super-secret-refresh-key-change-in-production"),
			AccessExpiry:  getDuration("JWT_ACCESS_EXPIRY", 15*time.Minute),
			RefreshExpiry: getDuration("JWT_REFRESH_EXPIRY", 7*24*time.Hour),
		},
		Security: SecurityConfig{
			RateLimitRequests: getEnvInt("RATE_LIMIT_REQUESTS", 100),
			RateLimitWindow:   getDuration("RATE_LIMIT_WINDOW", time.Minute),
			SessionTimeout:    getDuration("SESSION_TIMEOUT", 30*time.Minute),
			MaxLoginAttempts:  getEnvInt("MAX_LOGIN_ATTEMPTS", 5),
			LockoutDuration:   getDuration("LOCKOUT_DURATION", 15*time.Minute),
		},
		TLS: TLSConfig{
			Enabled:  getEnvBool("TLS_ENABLED", false),
			CertFile: getEnv("TLS_CERT_FILE", ""),
			KeyFile:  getEnv("TLS_KEY_FILE", ""),
		},
	}
}

// Validate performs security validation on the configuration
func (c *Config) Validate() *ValidationResult {
	result := &ValidationResult{}
	isProduction := c.Mode == "production"

	// JWT Secret Validation
	if isInsecureDefault(c.JWT.AccessSecret) {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "JWT_ACCESS_SECRET",
			Message: "Using insecure default value. Set a secure random secret (min 32 bytes).",
			Fatal:   isProduction,
		})
	} else if len(c.JWT.AccessSecret) < 32 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "JWT_ACCESS_SECRET",
			Message: fmt.Sprintf("Secret too short (%d chars). Minimum 32 characters required.", len(c.JWT.AccessSecret)),
			Fatal:   isProduction,
		})
	}

	if isInsecureDefault(c.JWT.RefreshSecret) {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "JWT_REFRESH_SECRET",
			Message: "Using insecure default value. Set a secure random secret (min 32 bytes).",
			Fatal:   isProduction,
		})
	} else if len(c.JWT.RefreshSecret) < 32 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "JWT_REFRESH_SECRET",
			Message: fmt.Sprintf("Secret too short (%d chars). Minimum 32 characters required.", len(c.JWT.RefreshSecret)),
			Fatal:   isProduction,
		})
	}

	// Database encryption validation for production (SQLite only)
	if isProduction && c.Database.Type == "sqlite" && c.Database.SQLite.EncryptionKey == "" {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "DB_ENCRYPTION_KEY",
			Message: "Database encryption not configured. Recommend setting DB_ENCRYPTION_KEY for production.",
		})
	}

	// TLS Validation for Production - warning only
	if isProduction && !c.TLS.Enabled {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "TLS_ENABLED",
			Message: "TLS is not enabled. For production, use TLS or deploy behind a reverse proxy.",
		})
	}

	if c.TLS.Enabled {
		if c.TLS.CertFile == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "TLS_CERT_FILE",
				Message: "TLS enabled but no certificate file specified.",
				Fatal:   true,
			})
		}
		if c.TLS.KeyFile == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "TLS_KEY_FILE",
				Message: "TLS enabled but no key file specified.",
				Fatal:   true,
			})
		}
	}

	return result
}

// MustValidate panics if configuration has fatal errors
func (c *Config) MustValidate() {
	result := c.Validate()
	if result.HasFatalErrors() {
		panic(fmt.Sprintf("Configuration validation failed:\n%s", result.String()))
	}
	if len(result.Warnings) > 0 {
		fmt.Printf("Configuration warnings:\n%s", result.String())
	}
}

// GenerateSecureSecret generates a cryptographically secure random secret
func GenerateSecureSecret(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Mode == "production"
}

func isInsecureDefault(value string) bool {
	valueLower := strings.ToLower(value)
	for _, insecure := range insecureDefaults {
		if strings.Contains(valueLower, strings.ToLower(insecure)) {
			return true
		}
	}
	return false
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return strings.ToLower(value) == "true" || value == "1"
	}
	return defaultValue
}

func getDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// RequiredEnvError is returned when a required environment variable is missing
var ErrMissingRequiredEnv = errors.New("required environment variable not set")

// getExecutableDir returns the directory containing the executable
func getExecutableDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exePath)
}
