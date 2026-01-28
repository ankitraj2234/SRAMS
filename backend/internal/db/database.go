package db

// This file is deprecated - using sqlite.go instead
// Keeping for reference during migration only

import (
	"context"
	"database/sql"
)

// Database wraps the SQL database
type Database struct {
	DB *sql.DB
}

// Close closes the database connection
func (db *Database) Close() {
	if db.DB != nil {
		db.DB.Close()
	}
}

// Health checks database connectivity
func (db *Database) Health(ctx context.Context) error {
	return db.DB.PingContext(ctx)
}
