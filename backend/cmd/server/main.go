package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/srams/backend/internal/config"
	"github.com/srams/backend/internal/handlers"
	"github.com/srams/backend/internal/middleware"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Validate configuration - fail fast if insecure in production
	validation := cfg.Validate()
	if validation.HasFatalErrors() {
		log.Fatalf("CONFIGURATION ERROR - Cannot start:\n%s", validation.String())
	}
	if len(validation.Warnings) > 0 {
		log.Printf("Configuration warnings:\n%s", validation.String())
	}

	if cfg.IsProduction() {
		log.Println("Running in PRODUCTION mode")
		gin.SetMode(gin.ReleaseMode)
	} else {
		log.Println("Running in DEVELOPMENT mode - NOT FOR PRODUCTION USE")
	}

	// Initialize App (Database and Services)
	app, err := InitApp(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}
	defer app.Close()

	// Auto-seed Super Admin if config file exists
	if err := seedSuperAdminFromConfig(app); err != nil {
		log.Printf("Warning: Could not seed super admin: %v", err)
	}

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(app.UserService, app.AuditService, app.AuthService)
	userHandler := handlers.NewUserHandler(app.UserService, app.AuditService)
	docHandler := handlers.NewDocumentHandler(app.DocumentService, app.AuditService)
	auditHandler := handlers.NewAuditHandler(app.AuditService)
	systemHandler := handlers.NewSystemHandler(app.SystemService, app.AuditService)
	bulkHandler := handlers.NewBulkImportHandler(app.ExcelService)
	realtimeHandler := handlers.NewRealtimeHandler()

	// Initialize SSE hub for real-time events
	handlers.InitSSEHub()
	log.Println("SSE Hub initialized for real-time event broadcasting")

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(app.AuthService, app.UserService)
	rateLimiter := middleware.NewRateLimiter(rate.Limit(cfg.Security.RateLimitRequests), cfg.Security.RateLimitRequests)

	// Setup Gin
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// Initialize server IPs for same-machine detection
	middleware.InitServerIPs()
	log.Printf("Server IPs for access control: %v", middleware.GetServerIPs())

	// Global middleware
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.RequestID())
	router.Use(middleware.Logger())
	router.Use(rateLimiter.RateLimit())

	// CORS configuration (Optionally handled by middleware.CORSMiddleware or explicitly here)
	// We use middleware.CORSMiddleware() above, assuming it's sufficient.
	// If custom config needed:
	/*
		corsHandler := cors.New(cors.Options{...})
		router.Use(func(c *gin.Context) { corsHandler.ServeHTTP(c.Writer, c.Request, func(w http.ResponseWriter, r *http.Request){ c.Next() }) })
	*/

	// Route Registration Helper
	setupRoutes := func(rg *gin.RouterGroup) {
		// Public routes
		auth := rg.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/refresh", authHandler.Refresh)
			auth.POST("/logout", authHandler.Logout)
			// Change Password (Authenticated)
			auth.POST("/change-password", authMiddleware.Authenticate(), authHandler.ChangePassword)
			// Explicit compatibility for profile in auth group (legacy support)
			auth.GET("/profile", authMiddleware.Authenticate(), userHandler.GetProfile)

			// TOTP Routes
			auth.GET("/totp/setup", authMiddleware.Authenticate(), authHandler.SetupTOTP)
			auth.POST("/totp/enable", authMiddleware.Authenticate(), authHandler.EnableTOTP)
			auth.POST("/totp/disable", authMiddleware.Authenticate(), authHandler.DisableTOTP)
		}

		// System routes (some public/gated)
		system := rg.Group("/system")
		{
			system.GET("/health", systemHandler.GetHealth)
			system.POST("/desktop-session", systemHandler.CreateDesktopSession)
		}

		// Protected routes
		protected := rg.Group("/")
		protected.Use(authMiddleware.Authenticate())
		{
			// User routes
			users := protected.Group("/users")
			{
				users.GET("", middleware.RequireRole("admin", "super_admin"), userHandler.List)
				users.GET("/:id", userHandler.Get)
				users.POST("", middleware.RequireRole("admin", "super_admin"), userHandler.Create)
				users.PUT("/:id", middleware.RequireRole("admin", "super_admin"), userHandler.Update)
				users.DELETE("/:id", middleware.RequireRole("super_admin"), userHandler.Delete)
				users.GET("/me", userHandler.GetProfile)
				users.GET("/stats", middleware.RequireRole("admin", "super_admin"), userHandler.GetDashboardStats)

				// Bulk Import/Export routes
				bulk := users.Group("/bulk")
				bulk.Use(middleware.RequireRole("admin", "super_admin"))
				{
					bulk.GET("/template", bulkHandler.GetTemplate)
					bulk.POST("/preview", bulkHandler.Preview)
					bulk.POST("/import", bulkHandler.Import)
					bulk.GET("/export", bulkHandler.Export)
				}
			}

			// Document routes
			docs := protected.Group("/documents")
			{
				docs.GET("", docHandler.List)
				docs.POST("/upload", docHandler.Upload)
				docs.GET("/my", docHandler.MyDocuments)

				// Document ID routes
				docs.GET("/id/:id", docHandler.Get)
				docs.DELETE("/id/:id", docHandler.Delete)
				docs.GET("/id/:id/view", docHandler.View)

				// View Session routes
				docs.POST("/view/:viewId/end", docHandler.EndView)
				docs.POST("/view/:viewId/page/:page", docHandler.RecordPageView)

				// Access control
				docs.GET("/id/:id/access", docHandler.GetDocumentAccess)
				docs.POST("/id/:id/access", docHandler.GrantAccess)
				docs.DELETE("/id/:id/access/:userId", docHandler.RevokeAccess)

				// Requests
				docs.POST("/request", docHandler.CreateRequest)
				docs.GET("/requests/my", docHandler.GetMyRequests)
				docs.GET("/requests/pending", middleware.RequireRole("admin", "super_admin"), docHandler.GetPendingRequests)
				docs.POST("/requests/:id/approve", middleware.RequireRole("admin", "super_admin"), docHandler.ApproveRequest)
				docs.POST("/requests/:id/reject", middleware.RequireRole("admin", "super_admin"), docHandler.RejectRequest)
			}

			// Audit routes
			audit := protected.Group("/audit")
			audit.Use(middleware.RequireRole("admin", "super_admin"))
			{
				audit.GET("", auditHandler.List)
				audit.GET("/:id", auditHandler.Get)
				audit.DELETE("/:id", middleware.RequireRole("super_admin"), auditHandler.Delete)
				audit.POST("/bulk-delete", middleware.RequireRole("super_admin"), auditHandler.BulkDelete)
				audit.GET("/stats", auditHandler.GetStats)
				audit.GET("/export", auditHandler.Export)
				audit.GET("/timeline/user/:userId", auditHandler.GetUserTimeline)

				// Fix for /audit/log (POST) which was missing in original but requested by frontend
				audit.POST("/log", auditHandler.LogEvent)
			}

			// System Public (Authenticated) routes
			system := protected.Group("/system")
			{
				system.GET("/config", systemHandler.GetConfig)
				system.GET("/logo", systemHandler.GetLogo)
			}

			// System Admin routes (Super Admin only)
			sysAdmin := protected.Group("/system")
			sysAdmin.Use(middleware.RequireRole("super_admin"))
			{
				// Config update
				sysAdmin.PUT("/config", systemHandler.UpdateConfig)

				// Analytics & Maintenance
				sysAdmin.GET("/db-stats", systemHandler.GetDatabaseStats)
				sysAdmin.GET("/sessions", systemHandler.GetSessionAnalytics)
				sysAdmin.POST("/sessions/cleanup", systemHandler.CleanupSessions)

				// Logo management (write/delete)
				sysAdmin.POST("/logo", systemHandler.UploadLogo)
				sysAdmin.DELETE("/logo", systemHandler.DeleteLogo)
			}

			// Real-time
			protected.GET("/events", realtimeHandler.ServeSSE)
			// Fix for /realtime/subscribe if it exists, mapping to ServeSSE
			rg.GET("/realtime/subscribe", authMiddleware.Authenticate(), realtimeHandler.ServeSSE)

			// Compatibility Routes (Frontend expectation mismatches)
			rg.GET("/dashboard/stats", authMiddleware.Authenticate(), userHandler.GetDashboardStats)
			rg.GET("/requests/my", authMiddleware.Authenticate(), docHandler.GetMyRequests)

			// Documents root POST mapping to Upload
			// Fix for POST /api/v1/documents (mapped to Upload)
			rg.POST("/documents", authMiddleware.Authenticate(), docHandler.Upload)
		}
	}

	// Register routes for both /api and /api/v1 (Compatibility)
	api := router.Group("/api")
	setupRoutes(api)
	setupRoutes(api.Group("/v1"))

	// SERVE FRONTEND (SPA)
	// Try to find frontend dist directory
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	// Search paths:
	// 1. ../frontend (Production: srams-server.exe is in backend/, frontend is sibling)
	// 2. ./frontend (Alternative)
	// 3. ./dist (Dev)
	frontendPaths := []string{
		filepath.Join(exeDir, "..", "frontend"),
		filepath.Join(exeDir, "frontend"),
		filepath.Join(exeDir, "dist"),
		"./frontend",
		"../frontend/dist",
	}

	var frontendDir string
	for _, p := range frontendPaths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			// Check if index.html exists
			if _, err := os.Stat(filepath.Join(p, "index.html")); err == nil {
				frontendDir = p
				break
			}
		}
	}

	if frontendDir != "" {
		log.Printf("Serving frontend from: %s", frontendDir)

		// Serve static files
		router.Use(static.Serve("/", static.LocalFile(frontendDir, true)))

		// SPA Fallback: If no route matched (404), serve index.html
		router.NoRoute(func(c *gin.Context) {
			if !strings.HasPrefix(c.Request.URL.Path, "/api") {
				c.File(filepath.Join(frontendDir, "index.html"))
			} else {
				c.JSON(http.StatusNotFound, gin.H{"error": "API route not found"})
			}
		})
	} else {
		log.Println("[WARNING] Frontend directory not found! Running in API-only mode.")
	}

	// Start server
	srv := &http.Server{
		Addr:         cfg.Server.Host + ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	log.Printf("Server starting on %s:%s", cfg.Server.Host, cfg.Server.Port)

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}

	log.Println("Server exiting")
}
