package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// AdminSetup represents the admin credentials from the setup file
type AdminSetup struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

// seedSuperAdminFromConfig reads admin credentials from config and seeds the super admin
func seedSuperAdminFromConfig(app *App) error {
	// Look for admin setup file in various locations
	// The backend service runs with CWD set to {install}/config/
	// So we check both relative to CWD and relative to executable
	configPaths := []string{
		// Direct in CWD (when CWD is config folder)
		"needs-admin-setup",
		"admin-setup.json",
		// Relative paths from backend folder
		"../config/needs-admin-setup",
		"../config/admin-setup.json",
		"config/needs-admin-setup",
		"config/admin-setup.json",
	}

	// Get executable directory and add those paths
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		configPaths = append(configPaths,
			// Installed layout: backend/srams-server.exe -> ../config/
			filepath.Join(execDir, "..", "config", "needs-admin-setup"),
			filepath.Join(execDir, "..", "config", "admin-setup.json"),
		)
	}

	var adminSetup AdminSetup
	var foundPath string

	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if err := json.Unmarshal(data, &adminSetup); err != nil {
				continue
			}
			if adminSetup.Email != "" && adminSetup.Password != "" {
				foundPath = path
				break
			}
		}
	}

	if foundPath == "" {
		// No admin setup file found - this is normal after first run
		log.Println("No admin setup file found, skipping auto-seed")
		return nil
	}

	log.Printf("Found admin setup file: %s", foundPath)

	// Check if super admin already exists
	ctx := context.Background()
	existingAdmin, err := app.UserService.GetByEmail(ctx, adminSetup.Email)
	if err == nil && existingAdmin != nil {
		log.Printf("Super admin %s already exists, skipping seed", adminSetup.Email)
		// Clean up the setup file
		os.Remove(foundPath)
		return nil
	}

	// Create super admin using the interface method
	log.Printf("Creating super admin: %s", adminSetup.Email)

	_, err = app.UserService.CreateSuperAdmin(ctx, adminSetup.Email, adminSetup.Password, adminSetup.FullName, "")
	if err != nil {
		log.Printf("Failed to create super admin: %v", err)
		return err
	}

	log.Printf("Super admin created successfully: %s", adminSetup.Email)

	// Clean up the setup file (contains password)
	if err := os.Remove(foundPath); err != nil {
		log.Printf("Warning: Could not remove admin setup file: %v", err)
	} else {
		log.Println("Cleaned up admin setup file")
	}

	return nil
}
