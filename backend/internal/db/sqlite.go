package db

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
	_ "modernc.org/sqlite"
)

// SQLiteConfig holds SQLite database configuration
type SQLiteConfig struct {
	// FilePath is the path to the database file
	FilePath string
	// EncryptionKey is the master password for database encryption
	EncryptionKey string
	// MaxSizeMB is the maximum database size in megabytes (default 5120 = 5GB)
	MaxSizeMB int64
	// WALMode enables Write-Ahead Logging for better concurrency
	WALMode bool
	// BusyTimeoutMs is the timeout for busy connections
	BusyTimeoutMs int
}

// SQLiteDatabase wraps the SQL database with encryption support
type SQLiteDatabase struct {
	DB            *sql.DB
	config        SQLiteConfig
	encryptionKey []byte
	mu            sync.RWMutex
}

// NewSQLite creates a new encrypted SQLite database connection
func NewSQLite(cfg SQLiteConfig) (*SQLiteDatabase, error) {
	// Ensure data directory exists
	dataDir := filepath.Dir(cfg.FilePath)
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Derive encryption key from password using Argon2id
	var encKey []byte
	if cfg.EncryptionKey != "" {
		salt := []byte("SRAMS-SQLite-v1") // Static salt, combined with data-specific IVs
		encKey = argon2.IDKey([]byte(cfg.EncryptionKey), salt, 3, 64*1024, 4, 32)
	}

	// Set defaults
	if cfg.MaxSizeMB == 0 {
		cfg.MaxSizeMB = 5120 // 5GB default
	}
	if cfg.BusyTimeoutMs == 0 {
		cfg.BusyTimeoutMs = 10000 // 10 seconds
	}

	// Open SQLite database
	db, err := sql.Open("sqlite", cfg.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool for SQLite
	db.SetMaxOpenConns(1) // SQLite only supports one writer
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	// Initialize database with optimizations
	pragmas := []string{
		fmt.Sprintf("PRAGMA busy_timeout = %d", cfg.BusyTimeoutMs),
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -64000", // 64MB cache
		"PRAGMA temp_store = MEMORY",
		"PRAGMA mmap_size = 268435456", // 256MB memory-mapped I/O
	}

	if cfg.WALMode {
		pragmas = append(pragmas, "PRAGMA journal_mode = WAL")
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma %s: %w", pragma, err)
		}
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	sqliteDB := &SQLiteDatabase{
		DB:            db,
		config:        cfg,
		encryptionKey: encKey,
	}

	// Run migrations if database is new
	if err := sqliteDB.RunMigrations(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return sqliteDB, nil
}

// Close closes the database connection
func (s *SQLiteDatabase) Close() error {
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}

// Health checks database connectivity
func (s *SQLiteDatabase) Health(ctx context.Context) error {
	return s.DB.PingContext(ctx)
}

// GetDB returns the underlying sql.DB for query operations
func (s *SQLiteDatabase) GetDB() *sql.DB {
	return s.DB
}

// CheckSize checks if database is approaching size limit
func (s *SQLiteDatabase) CheckSize() (currentMB int64, maxMB int64, err error) {
	info, err := os.Stat(s.config.FilePath)
	if err != nil {
		return 0, s.config.MaxSizeMB, err
	}
	currentMB = info.Size() / (1024 * 1024)
	return currentMB, s.config.MaxSizeMB, nil
}

// Encrypt encrypts data using AES-256-GCM
func (s *SQLiteDatabase) Encrypt(plaintext []byte) ([]byte, error) {
	if s.encryptionKey == nil {
		return plaintext, nil // No encryption configured
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts data using AES-256-GCM
func (s *SQLiteDatabase) Decrypt(ciphertext []byte) ([]byte, error) {
	if s.encryptionKey == nil {
		return ciphertext, nil // No encryption configured
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// HashForStorage creates a deterministic hash for storage (for indexed encrypted fields)
func (s *SQLiteDatabase) HashForStorage(data string) string {
	h := sha256.New()
	h.Write([]byte(data))
	if s.encryptionKey != nil {
		h.Write(s.encryptionKey)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Backup creates a backup of the database
func (s *SQLiteDatabase) Backup(destPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Checkpoint WAL if in WAL mode
	if s.config.WALMode {
		if _, err := s.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			return fmt.Errorf("failed to checkpoint WAL: %w", err)
		}
	}

	// Copy the database file
	srcFile, err := os.Open(s.config.FilePath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return err
	}

	return destFile.Sync()
}

// RunMigrations applies database schema
func (s *SQLiteDatabase) RunMigrations() error {
	// Create version table
	_, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			applied_at TEXT DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return err
	}

	// Check current version
	var currentVersion int
	err = s.DB.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&currentVersion)
	if err != nil {
		return err
	}

	// Apply migrations
	migrations := []struct {
		version int
		sql     string
	}{
		{1, schemaMigration001},
	}

	for _, m := range migrations {
		if m.version > currentVersion {
			if _, err := s.DB.Exec(m.sql); err != nil {
				return fmt.Errorf("migration %d failed: %w", m.version, err)
			}
			if _, err := s.DB.Exec("INSERT INTO schema_version (version) VALUES (?)", m.version); err != nil {
				return err
			}
		}
	}

	return nil
}

// Initial schema migration for SQLite
const schemaMigration001 = `
-- SRAMS SQLite Database Schema
-- Version: 1.0.0

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    full_name TEXT NOT NULL,
    mobile TEXT,
    role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('super_admin', 'admin', 'user')),
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    last_login TEXT,
    totp_secret TEXT,
    totp_enabled INTEGER NOT NULL DEFAULT 0,
    failed_login_attempts INTEGER DEFAULT 0,
    locked_until TEXT,
    must_change_password INTEGER NOT NULL DEFAULT 0,
    must_enroll_mfa INTEGER NOT NULL DEFAULT 0
);

-- Sessions table
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    refresh_token_hash TEXT NOT NULL,
    ip_address TEXT,
    device_fingerprint TEXT,
    user_agent TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    expires_at TEXT NOT NULL,
    last_activity TEXT DEFAULT (datetime('now')),
    is_active INTEGER NOT NULL DEFAULT 1
);

-- Documents table
CREATE TABLE IF NOT EXISTS documents (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    filename TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_hash TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    uploaded_by TEXT NOT NULL REFERENCES users(id),
    created_at TEXT DEFAULT (datetime('now')),
    is_active INTEGER NOT NULL DEFAULT 1
);

-- Document access table
CREATE TABLE IF NOT EXISTS document_access (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    granted_by TEXT NOT NULL REFERENCES users(id),
    granted_at TEXT DEFAULT (datetime('now')),
    revoked_at TEXT,
    is_active INTEGER NOT NULL DEFAULT 1,
    UNIQUE(document_id, user_id)
);

-- User requests table
CREATE TABLE IF NOT EXISTS user_requests (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    request_type TEXT NOT NULL DEFAULT 'access',
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    reason TEXT,
    reviewed_by TEXT REFERENCES users(id),
    reviewed_at TEXT,
    review_note TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);

-- Audit logs table
CREATE TABLE IF NOT EXISTS audit_logs (
    id TEXT PRIMARY KEY,
    actor_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    actor_role TEXT,
    action_type TEXT NOT NULL,
    target_type TEXT,
    target_id TEXT,
    metadata TEXT DEFAULT '{}',
    ip_address TEXT,
    device_id TEXT,
    user_agent TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    deleted_at TEXT,
    deleted_by TEXT REFERENCES users(id),
    deletion_reason TEXT
);

-- Admin actions table
CREATE TABLE IF NOT EXISTS admin_actions (
    id TEXT PRIMARY KEY,
    admin_id TEXT NOT NULL REFERENCES users(id),
    action_type TEXT NOT NULL,
    target_user_id TEXT REFERENCES users(id),
    details TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);

-- System configuration table
CREATE TABLE IF NOT EXISTS system_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_by TEXT REFERENCES users(id),
    updated_at TEXT DEFAULT (datetime('now'))
);

-- Device certificates table (for Super Admin device verification)
CREATE TABLE IF NOT EXISTS device_certificates (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    fingerprint TEXT NOT NULL,
    machine_id TEXT,
    os_info TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    last_used_at TEXT,
    revoked_at TEXT,
    revoked_by TEXT REFERENCES users(id),
    UNIQUE(user_id, fingerprint)
);

-- Desktop sessions table (for Super Admin desktop app gating)
CREATE TABLE IF NOT EXISTS desktop_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    session_token TEXT NOT NULL UNIQUE,
    ip_address TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    expires_at TEXT NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 1
);

-- Document views table
CREATE TABLE IF NOT EXISTS document_views (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    started_at TEXT DEFAULT (datetime('now')),
    ended_at TEXT,
    pages_viewed TEXT DEFAULT '[]',
    total_seconds INTEGER DEFAULT 0
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_is_active ON sessions(is_active);
CREATE INDEX IF NOT EXISTS idx_documents_uploaded_by ON documents(uploaded_by);
CREATE INDEX IF NOT EXISTS idx_document_access_user_id ON document_access(user_id);
CREATE INDEX IF NOT EXISTS idx_document_access_document_id ON document_access(document_id);
CREATE INDEX IF NOT EXISTS idx_user_requests_user_id ON user_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_user_requests_status ON user_requests(status);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_id ON audit_logs(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action_type ON audit_logs(action_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_document_views_document_id ON document_views(document_id);
CREATE INDEX IF NOT EXISTS idx_document_views_user_id ON document_views(user_id);

-- Insert default system config
INSERT OR IGNORE INTO system_config (key, value, updated_at) VALUES
    ('audit_retention_days', '365', datetime('now')),
    ('max_upload_size_mb', '50', datetime('now')),
    ('session_timeout_minutes', '30', datetime('now')),
    ('max_login_attempts', '5', datetime('now')),
    ('lockout_duration_minutes', '15', datetime('now')),
    ('require_2fa_admin', 'false', datetime('now')),
    ('require_2fa_super_admin', 'false', datetime('now')),
    ('database_max_size_mb', '5120', datetime('now'));
`
