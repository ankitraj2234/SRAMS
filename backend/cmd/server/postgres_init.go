//go:build postgres
// +build postgres

package main

// This file provides PostgreSQL-specific initialization
// Build with: go build -tags postgres -o srams-server-pg.exe ./cmd/server
// Or use DB_TYPE=postgres environment variable with the unified build

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/srams/backend/internal/auth"
	"github.com/srams/backend/internal/db/postgres"
)

// PostgresApp holds all PostgreSQL services
type PostgresApp struct {
	DB              *postgres.Database
	AuthService     *auth.Service
	UserService     *postgres.UserService
	DocumentService *postgres.DocumentService
	AuditService    *postgres.AuditService
	SystemService   *postgres.SystemService
}

// InitPostgres initializes the PostgreSQL database and all services
func InitPostgres(jwtConfig interface{}) (*PostgresApp, error) {
	// Load PostgreSQL config from environment
	cfg := postgres.Config{
		Host:     getEnvOrDefault("DB_HOST", "localhost"),
		Port:     getEnvInt("DB_PORT", 5432),
		Database: getEnvOrDefault("DB_NAME", "srams"),
		User:     getEnvOrDefault("DB_USER", "srams_app"),
		Password: getEnvOrDefault("DB_PASSWORD", ""),
		SSLMode:  getEnvOrDefault("DB_SSLMODE", "prefer"),
		MaxConns: int32(getEnvInt("DB_MAX_CONNS", 25)),
		MinConns: int32(getEnvInt("DB_MIN_CONNS", 5)),
	}

	// Connect to database
	db, err := postgres.NewDatabase(cfg)
	if err != nil {
		return nil, err
	}

	// Run migrations if requested
	if os.Getenv("DB_RUN_MIGRATIONS") == "true" {
		log.Println("Running database migrations...")
		if err := db.RunMigrations(context.Background()); err != nil {
			db.Close()
			return nil, err
		}
		log.Println("Migrations completed successfully")
	}

	// Initialize auth service
	authService := auth.NewService(jwtConfig.(*auth.JWTConfig))

	// Initialize PostgreSQL services
	app := &PostgresApp{
		DB:              db,
		AuthService:     authService,
		UserService:     postgres.NewUserService(db.Pool, authService),
		DocumentService: postgres.NewDocumentService(db.Pool, getEnvOrDefault("DOCUMENTS_PATH", "./documents")),
		AuditService:    postgres.NewAuditService(db.Pool),
		SystemService:   postgres.NewSystemService(db.Pool, getEnvOrDefault("DOCUMENTS_PATH", "./documents")),
	}

	log.Printf("Connected to PostgreSQL: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	// Log pool stats
	stats := db.GetStats()
	log.Printf("Connection pool: max=%d, idle=%d, total=%d",
		stats.MaxConns(), stats.IdleConns(), stats.TotalConns())

	return app, nil
}

// Close closes the PostgreSQL connection
func (app *PostgresApp) Close() {
	if app.DB != nil {
		app.DB.Close()
	}
}

// Helper functions
func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		var i int
		if _, err := fmt.Sscanf(val, "%d", &i); err == nil {
			return i
		}
	}
	return defaultVal
}
