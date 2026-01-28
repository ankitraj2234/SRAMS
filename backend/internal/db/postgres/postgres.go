package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Config holds PostgreSQL database configuration
type Config struct {
	Host        string
	Port        int
	Database    string
	User        string
	Password    string
	SSLMode     string // disable, require, verify-ca, verify-full
	MaxConns    int32
	MinConns    int32
	MaxConnLife time.Duration
	MaxConnIdle time.Duration
}

// Database wraps the PostgreSQL connection pool
type Database struct {
	Pool   *pgxpool.Pool
	config Config
}

// NewDatabase creates a new PostgreSQL database connection pool
func NewDatabase(cfg Config) (*Database, error) {
	// Set defaults
	if cfg.Port == 0 {
		cfg.Port = 5432
	}
	if cfg.SSLMode == "" {
		cfg.SSLMode = "prefer"
	}
	if cfg.MaxConns == 0 {
		cfg.MaxConns = 25
	}
	if cfg.MinConns == 0 {
		cfg.MinConns = 5
	}
	if cfg.MaxConnLife == 0 {
		cfg.MaxConnLife = time.Hour
	}
	if cfg.MaxConnIdle == 0 {
		cfg.MaxConnIdle = 30 * time.Minute
	}

	// Build connection string
	connString := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.Database, cfg.User, cfg.Password, cfg.SSLMode,
	)

	// Parse config
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Configure pool
	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLife
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdle
	poolConfig.HealthCheckPeriod = time.Minute

	// Create pool
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Printf("Connected to PostgreSQL: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	return &Database{
		Pool:   pool,
		config: cfg,
	}, nil
}

// Close closes the database connection pool
func (db *Database) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

// SetSessionContext sets the current user context for RLS
// This MUST be called at the start of each HTTP request
func (db *Database) SetSessionContext(ctx context.Context, userID, sessionID uuid.UUID, role string) error {
	_, err := db.Pool.Exec(ctx,
		"SELECT srams.set_session_context($1, $2, $3)",
		userID, role, sessionID,
	)
	return err
}

// ClearSessionContext clears the current session context
func (db *Database) ClearSessionContext(ctx context.Context) error {
	_, err := db.Pool.Exec(ctx,
		"SELECT srams.set_session_context(NULL, NULL, NULL)",
	)
	return err
}

// WithTransaction executes a function within a transaction
func (db *Database) WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// RunMigrations executes all embedded SQL migration files
func (db *Database) RunMigrations(ctx context.Context) error {
	// Get list of migration files
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations: %w", err)
	}

	// Sort files by name (001_, 002_, etc.)
	var migrationFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			migrationFiles = append(migrationFiles, entry.Name())
		}
	}
	sort.Strings(migrationFiles)

	// Execute each migration
	// Execute each migration
	for _, file := range migrationFiles {
		log.Printf("Running migration: %s", file)

		// embed.FS always uses forward slashes, even on Windows
		// filepath.Join uses backslashes on Windows, which causes "file does not exist"
		content, err := migrationsFS.ReadFile("migrations/" + file)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", file, err)
		}

		_, err = db.Pool.Exec(ctx, string(content))
		if err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", file, err)
		}

		log.Printf("Completed migration: %s", file)
	}

	return nil
}

// GetStats returns connection pool statistics
func (db *Database) GetStats() *pgxpool.Stat {
	return db.Pool.Stat()
}

// ConfigFromEnv creates a Config from environment variables
func ConfigFromEnv() Config {
	port := 5432
	if p := os.Getenv("DB_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}

	maxConns := int32(25)
	if m := os.Getenv("DB_MAX_CONNS"); m != "" {
		fmt.Sscanf(m, "%d", &maxConns)
	}

	minConns := int32(5)
	if m := os.Getenv("DB_MIN_CONNS"); m != "" {
		fmt.Sscanf(m, "%d", &minConns)
	}

	return Config{
		Host:        os.Getenv("DB_HOST"),
		Port:        port,
		Database:    os.Getenv("DB_NAME"),
		User:        os.Getenv("DB_USER"),
		Password:    os.Getenv("DB_PASSWORD"),
		SSLMode:     os.Getenv("DB_SSLMODE"),
		MaxConns:    maxConns,
		MinConns:    minConns,
		MaxConnLife: time.Hour,
		MaxConnIdle: 30 * time.Minute,
	}
}
