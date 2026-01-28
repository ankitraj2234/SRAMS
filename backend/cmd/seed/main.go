package main

import (
	"context"
	"log"
	"os"

	"github.com/srams/backend/internal/auth"
	"github.com/srams/backend/internal/config"
	"github.com/srams/backend/internal/db/postgres"
)

// We need to access InitApp from main package.
// However, main package is "package main" in cmd/server/main.go and db_factory.go.
// We cannot import "package main".
// We must duplicate InitApp logic or move db_factory to an internal package.
// Refactoring db_factory to internal/app (or similar) is cleaner but might be too much change now.
// I will copy the minimal Init logic here to connect to Postgres.

func main() {
	log.Println("Starting database seeder...")

	// Load config manually or via internal/config
	// Assuming .env is present
	cfg := config.Load()

	// Force Postgres config from env or defaults matching .env
	pgCfg := postgres.Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnvAsInt("DB_PORT", 5432),
		Database: getEnv("DB_NAME", "srams"),
		User:     getEnv("DB_USER", "srams_app"),
		Password: getEnv("DB_PASSWORD", "srams_app_2026"),
		SSLMode:  getEnv("DB_SSLMODE", "prefer"),
		MaxConns: 5,
		MinConns: 1,
	}

	log.Printf("Connecting to PostgreSQL at %s:%d/%s as %s", pgCfg.Host, pgCfg.Port, pgCfg.Database, pgCfg.User)

	db, err := postgres.NewDatabase(pgCfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize Auth Service (needed for hashing)
	// We need internal/auth
	// But auth.NewService requires *config.JWTConfig
	// internal/auth is available.
	// Wait, I need to look at imports.

	/*
	   Imports will be:
	   "github.com/srams/backend/internal/auth"
	   "github.com/srams/backend/internal/config"
	   "github.com/srams/backend/internal/db/postgres"
	*/

	authService := auth.NewService(&cfg.JWT)
	userService := postgres.NewUserService(db.Pool, authService)

	// Seed Super Admin
	email := "admin@srams.local"
	password := "Admin@123"
	fullName := "System Administrator"
	mobile := "+1234567890"

	log.Printf("Seeding Super Admin: %s", email)

	user, err := userService.CreateSuperAdmin(context.Background(), email, password, fullName, mobile)
	if err != nil {
		if err.Error() == "super admin already exists" {
			log.Println("Super Admin already exists. Skipping.")
			return
		}
		log.Fatalf("Failed to create super admin: %v", err)
	}

	log.Printf("Successfully created Super Admin (ID: %s)", user.ID)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	// simplified for brevity
	return fallback
}
