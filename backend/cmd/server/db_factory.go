package main

// Database factory for PostgreSQL
// PostgreSQL is the only supported database backend

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/srams/backend/internal/auth"
	"github.com/srams/backend/internal/config"
	"github.com/srams/backend/internal/db/postgres"
	"github.com/srams/backend/internal/interfaces"
)

// App holds all application services
type App struct {
	// PostgreSQL resources
	PostgresDB *postgres.Database

	// Shared services
	AuthService *auth.Service
	Config      *config.Config

	// Service interfaces
	UserService     interfaces.UserService
	DocumentService interfaces.DocumentService
	AuditService    interfaces.AuditService
	SystemService   interfaces.SystemService
	ExcelService    *postgres.ExcelService
}

// InitApp initializes the application with PostgreSQL
func InitApp(cfg *config.Config) (*App, error) {
	app := &App{
		Config: cfg,
	}

	// Initialize auth service
	app.AuthService = auth.NewService(&cfg.JWT)

	// Initialize PostgreSQL
	if err := app.initPostgres(); err != nil {
		return nil, fmt.Errorf("postgres init failed: %w", err)
	}

	log.Println("Database type: PostgreSQL")
	return app, nil
}

// initPostgres initializes PostgreSQL database and services
func (app *App) initPostgres() error {
	pgCfg := postgres.Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnvAsInt("DB_PORT", 5432),
		Database: getEnv("DB_NAME", "srams"),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", ""),
		SSLMode:  getEnv("DB_SSLMODE", "prefer"),
		MaxConns: int32(getEnvAsInt("DB_MAX_CONNS", 25)),
		MinConns: int32(getEnvAsInt("DB_MIN_CONNS", 5)),
	}

	database, err := postgres.NewDatabase(pgCfg)
	if err != nil {
		return err
	}
	app.PostgresDB = database

	// ALWAYS Run migrations
	// In installer context, we must ensure schema exists.
	// The overhead is minimal as it tracks applied migrations.
	log.Println("Verifying database schema...")
	if err := database.RunMigrations(context.Background()); err != nil {
		log.Printf("[WARNING] Migration check failed: %v", err)
		// Don't crash - maybe it's partially set up or permissions issue
		// Let the app try to run, but log heavily
	} else {
		log.Println("Database schema verified")
	}

	// Initialize PostgreSQL services
	app.UserService = postgres.NewUserService(database.Pool, app.AuthService)
	app.AuditService = postgres.NewAuditService(database.Pool)
	app.DocumentService = postgres.NewDocumentService(database.Pool, getEnv("DOCUMENTS_PATH", "./documents"))
	app.SystemService = postgres.NewSystemService(database.Pool, getEnv("DOCUMENTS_PATH", "./documents"))
	app.ExcelService = postgres.NewExcelService(app.UserService, app.AuthService)

	// Log pool stats
	stats := database.GetStats()
	log.Printf("PostgreSQL pool: max=%d, idle=%d, total=%d",
		stats.MaxConns(), stats.IdleConns(), stats.TotalConns())

	return nil
}

// Close closes the database connection
func (app *App) Close() {
	if app.PostgresDB != nil {
		app.PostgresDB.Close()
	}
}

// GetPostgresDB returns the PostgreSQL database
func (app *App) GetPostgresDB() *postgres.Database {
	return app.PostgresDB
}

// Helper functions
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		var i int
		if _, err := fmt.Sscanf(val, "%d", &i); err == nil {
			return i
		}
	}
	return defaultVal
}
