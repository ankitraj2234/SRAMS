package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/srams/backend/internal/db/postgres"
)

// PostgresContextKey is the context key for the PostgreSQL database
const PostgresContextKey = "postgres_db"

// DBContextMiddleware sets the PostgreSQL session context for RLS
// This middleware MUST run after authentication middleware
func DBContextMiddleware(db *postgres.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user from context (set by auth middleware)
		userVal, exists := c.Get("user")
		if !exists {
			// No authenticated user, continue without setting context
			c.Next()
			return
		}

		// Extract user info
		user, ok := userVal.(interface {
			GetID() uuid.UUID
			GetRole() string
		})
		if !ok {
			c.Next()
			return
		}

		// Get session ID (optional)
		var sessionID uuid.UUID
		if sid, exists := c.Get("session_id"); exists {
			if s, ok := sid.(uuid.UUID); ok {
				sessionID = s
			}
		}

		// Set PostgreSQL session context for RLS
		err := db.SetSessionContext(c.Request.Context(), user.GetID(), sessionID, user.GetRole())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to set database context",
			})
			return
		}

		// Store database in context for handlers
		c.Set(PostgresContextKey, db)

		c.Next()

		// Clear session context after request (optional, helps with connection pooling)
		_ = db.ClearSessionContext(c.Request.Context())
	}
}

// GetPostgresDB retrieves the PostgreSQL database from the Gin context
func GetPostgresDB(c *gin.Context) *postgres.Database {
	if db, exists := c.Get(PostgresContextKey); exists {
		if pgdb, ok := db.(*postgres.Database); ok {
			return pgdb
		}
	}
	return nil
}
