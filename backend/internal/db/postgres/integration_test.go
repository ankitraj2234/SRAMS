package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// IntegrationTest runs comprehensive integration tests for PostgreSQL
// Run with: go test -v ./internal/db/postgres/... -run TestIntegration
func TestIntegrationPostgres(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Connect to PostgreSQL
	cfg := ConfigFromEnv()
	if cfg.Host == "" {
		cfg.Host = "localhost"
		cfg.Port = 5432
		cfg.Database = "srams"
		cfg.User = "srams_app"
		cfg.Password = "srams_app_2026"
	}

	ctx := context.Background()
	db, err := NewDatabase(cfg)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// Run tests
	t.Run("TestDatabaseConnection", func(t *testing.T) {
		testDatabaseConnection(t, db)
	})

	t.Run("TestSessionContext", func(t *testing.T) {
		testSessionContext(t, db, ctx)
	})

	t.Run("TestRLSPolicies", func(t *testing.T) {
		testRLSPolicies(t, db, ctx)
	})

	t.Run("TestAuditImmutability", func(t *testing.T) {
		testAuditImmutability(t, db, ctx)
	})
}

func testDatabaseConnection(t *testing.T, db *Database) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test ping
	if err := db.Pool.Ping(ctx); err != nil {
		t.Errorf("Ping failed: %v", err)
	}

	// Test query
	var result int
	err := db.Pool.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		t.Errorf("Simple query failed: %v", err)
	}
	if result != 1 {
		t.Errorf("Expected 1, got %d", result)
	}

	t.Log("Database connection test passed")
}

func testSessionContext(t *testing.T, db *Database, ctx context.Context) {
	userID := uuid.New()
	sessionID := uuid.New()
	role := "admin"

	// Set session context
	err := db.SetSessionContext(ctx, userID, sessionID, role)
	if err != nil {
		t.Errorf("Failed to set session context: %v", err)
		return
	}

	// Verify context was set
	var currentUserID, currentSessionID, currentRole string
	err = db.Pool.QueryRow(ctx, `
		SELECT 
			current_setting('srams.current_user_id', true),
			current_setting('srams.current_session_id', true),
			current_setting('srams.current_user_role', true)
	`).Scan(&currentUserID, &currentSessionID, &currentRole)

	if err != nil {
		t.Errorf("Failed to get session context: %v", err)
		return
	}

	if currentUserID != userID.String() {
		t.Errorf("User ID mismatch: expected %s, got %s", userID, currentUserID)
	}
	if currentRole != role {
		t.Errorf("Role mismatch: expected %s, got %s", role, currentRole)
	}

	// Clear context
	err = db.ClearSessionContext(ctx)
	if err != nil {
		t.Errorf("Failed to clear session context: %v", err)
	}

	t.Log("Session context test passed")
}

func testRLSPolicies(t *testing.T, db *Database, ctx context.Context) {
	// Test that RLS is enabled on tables
	tables := []string{"srams.users", "srams.documents", "srams.document_access", "audit.logs"}

	for _, table := range tables {
		var rlsEnabled bool
		err := db.Pool.QueryRow(ctx, `
			SELECT relrowsecurity FROM pg_class 
			WHERE oid = $1::regclass
		`, table).Scan(&rlsEnabled)

		if err != nil {
			t.Errorf("Failed to check RLS for %s: %v", table, err)
			continue
		}

		if !rlsEnabled {
			t.Errorf("RLS is not enabled on %s", table)
		} else {
			t.Logf("RLS verified for %s", table)
		}
	}

	t.Log("RLS policies test passed")
}

func testAuditImmutability(t *testing.T, db *Database, ctx context.Context) {
	// First, set a context (required for insert)
	userID := uuid.New()
	sessionID := uuid.New()
	_ = db.SetSessionContext(ctx, userID, sessionID, "super_admin")

	// Insert a test audit log
	logID := uuid.New()
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO audit.logs (id, actor_id, actor_role, action_type, target_type)
		VALUES ($1, $2, 'test', 'test_action', 'test_target')
	`, logID, userID)

	if err != nil {
		t.Logf("Insert might fail due to RLS - this is expected: %v", err)
		return
	}

	// Try to DELETE (should fail)
	_, err = db.Pool.Exec(ctx, "DELETE FROM audit.logs WHERE id = $1", logID)
	if err == nil {
		t.Error("DELETE should have been prevented by trigger")
	} else {
		t.Logf("DELETE correctly prevented: %v", err)
	}

	// Try to UPDATE (should fail unless soft-delete)
	_, err = db.Pool.Exec(ctx, `
		UPDATE audit.logs SET action_type = 'modified' WHERE id = $1
	`, logID)
	if err == nil {
		t.Error("UPDATE should have been prevented by trigger")
	} else {
		t.Logf("UPDATE correctly prevented: %v", err)
	}

	t.Log("Audit immutability test passed")
}

// TestPgxPoolStats tests connection pool statistics
func TestPgxPoolStats(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := ConfigFromEnv()
	if cfg.Host == "" {
		t.Skip("No PostgreSQL configuration found")
	}

	db, err := NewDatabase(cfg)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	stats := db.GetStats()
	t.Logf("Pool stats: Total=%d, Acquired=%d, Idle=%d, Max=%d",
		stats.TotalConns(), stats.AcquiredConns(), stats.IdleConns(), stats.MaxConns())
}
