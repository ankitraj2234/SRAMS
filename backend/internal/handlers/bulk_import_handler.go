package handlers

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/srams/backend/internal/db/postgres"
	"github.com/srams/backend/internal/models"
)

// BulkImportHandler handles bulk user import/export operations
type BulkImportHandler struct {
	excelService *postgres.ExcelService
}

// NewBulkImportHandler creates a new bulk import handler
func NewBulkImportHandler(excelService *postgres.ExcelService) *BulkImportHandler {
	return &BulkImportHandler{
		excelService: excelService,
	}
}

// GetTemplate returns a downloadable Excel template for user import
// GET /api/v1/bulk/template
func (h *BulkImportHandler) GetTemplate(c *gin.Context) {
	buf, err := h.excelService.GenerateTemplate()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate template"})
		return
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=srams_user_import_template.xlsx")
	c.Header("Content-Length", strconv.Itoa(buf.Len()))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

// Preview parses an uploaded file and returns preview data
// POST /api/v1/bulk/preview
func (h *BulkImportHandler) Preview(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read file"})
		return
	}

	rows, err := h.excelService.ParseImportFile(data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Count valid vs invalid rows
	valid := 0
	invalid := 0
	for _, row := range rows {
		if row.Error == "" {
			valid++
		} else {
			invalid++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total":   len(rows),
		"valid":   valid,
		"invalid": invalid,
		"rows":    rows,
	})
}

// Import imports users from an uploaded Excel file
// POST /api/v1/bulk/import
func (h *BulkImportHandler) Import(c *gin.Context) {
	// Get current user
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser := userVal.(*models.User)

	// Get file
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read file"})
		return
	}

	// Parse file
	rows, err := h.excelService.ParseImportFile(data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get options from form
	opts := postgres.ExcelImportOptions{
		ForcePasswordChange: c.PostForm("force_password_change") == "true",
		ForceMFAEnrollment:  c.PostForm("force_mfa_enrollment") == "true",
		SkipDuplicates:      c.PostForm("skip_duplicates") != "false", // Default true
		DefaultRole:         c.PostForm("default_role"),
		DefaultPassword:     c.PostForm("default_password"),
	}

	// Import users
	result, err := h.excelService.ImportUsers(c.Request.Context(), rows, opts, currentUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Export exports all users to an Excel file
// GET /api/v1/bulk/export
func (h *BulkImportHandler) Export(c *gin.Context) {
	opts := postgres.ExcelExportOptions{
		IncludeInactive: c.Query("include_inactive") == "true",
	}

	buf, err := h.excelService.ExportUsers(c.Request.Context(), opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export users"})
		return
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=srams_users_export.xlsx")
	c.Header("Content-Length", strconv.Itoa(buf.Len()))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}
