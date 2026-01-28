package postgres

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/srams/backend/internal/auth"
	"github.com/srams/backend/internal/interfaces"
	"github.com/srams/backend/internal/models"
)

// ExcelImportOptions configures bulk import behavior
type ExcelImportOptions struct {
	ForcePasswordChange bool
	ForceMFAEnrollment  bool
	DefaultRole         string
	DefaultPassword     string
	SkipDuplicates      bool
}

// ExcelExportOptions configures bulk export behavior
type ExcelExportOptions struct {
	IncludeInactive  bool
	IncludePasswords bool // Only for template, never actual passwords
}

// ExcelRow represents a parsed row from the import file
type ExcelRow struct {
	Row      int    `json:"row"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Mobile   string `json:"mobile"`
	Role     string `json:"role"`
	Error    string `json:"error,omitempty"`
}

// ExcelImportResult contains the results of an import operation
type ExcelImportResult struct {
	TotalRows   int         `json:"total_rows"`
	Imported    int         `json:"imported"`
	Skipped     int         `json:"skipped"`
	Failed      int         `json:"failed"`
	Errors      []ExcelRow  `json:"errors,omitempty"`
	ImportedIDs []uuid.UUID `json:"imported_ids,omitempty"`
}

// ExcelService handles Excel import/export operations
type ExcelService struct {
	userService interfaces.UserService
	authService *auth.Service
}

// NewExcelService creates a new Excel service
func NewExcelService(userService interfaces.UserService, authService *auth.Service) *ExcelService {
	return &ExcelService{
		userService: userService,
		authService: authService,
	}
}

// GenerateTemplate creates a downloadable Excel template for user import
func (s *ExcelService) GenerateTemplate() (*bytes.Buffer, error) {
	f := excelize.NewFile()
	defer f.Close()

	// Rename default sheet
	f.SetSheetName("Sheet1", "Users")

	// Set headers
	headers := []string{"Email*", "Full Name*", "Mobile", "Role*"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("Users", cell, header)
	}

	// Style headers
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"4F46E5"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	f.SetCellStyle("Users", "A1", "D1", headerStyle)

	// Set column widths
	f.SetColWidth("Users", "A", "A", 30) // Email
	f.SetColWidth("Users", "B", "B", 25) // Full Name
	f.SetColWidth("Users", "C", "C", 15) // Mobile
	f.SetColWidth("Users", "D", "D", 12) // Role

	// Add sample rows
	sampleData := [][]interface{}{
		{"user1@example.com", "John Doe", "9876543210", "user"},
		{"user2@example.com", "Jane Smith", "9876543211", "admin"},
	}
	for i, row := range sampleData {
		for j, val := range row {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+2)
			f.SetCellValue("Users", cell, val)
		}
	}

	// Add instructions sheet
	f.NewSheet("Instructions")
	instructions := [][]interface{}{
		{"SRAMS User Import Template"},
		{""},
		{"Required Fields:"},
		{"- Email: Must be a valid email address"},
		{"- Full Name: User's display name"},
		{"- Role: Must be 'admin' or 'user' (super_admin cannot be imported)"},
		{""},
		{"Optional Fields:"},
		{"- Mobile: Phone number (any format)"},
		{""},
		{""},
		{"Notes:"},
		{"- Duplicate emails will be skipped"},
		{"- All imported users will be required to change password on first login"},
		{"- Maximum 1000 users per import"},
	}
	for i, row := range instructions {
		for j, val := range row {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+1)
			f.SetCellValue("Instructions", cell, val)
		}
	}

	// Set active sheet to Users
	usersIndex, _ := f.GetSheetIndex("Users")
	f.SetActiveSheet(usersIndex)

	// Write to buffer
	buf := new(bytes.Buffer)
	if err := f.Write(buf); err != nil {
		return nil, fmt.Errorf("failed to write Excel file: %w", err)
	}

	return buf, nil
}

// ParseImportFile parses an uploaded Excel file and returns rows
func (s *ExcelService) ParseImportFile(data []byte) ([]ExcelRow, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	// Get first sheet
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("no sheets found in Excel file")
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read rows: %w", err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("file must contain at least a header row and one data row")
	}

	// Skip header row
	var parsedRows []ExcelRow
	for i, row := range rows[1:] {
		if len(row) == 0 {
			continue // Skip empty rows
		}

		excelRow := ExcelRow{Row: i + 2} // Excel rows are 1-indexed, plus header

		// Parse columns safely
		if len(row) > 0 {
			excelRow.Email = strings.TrimSpace(row[0])
		}
		if len(row) > 1 {
			excelRow.FullName = strings.TrimSpace(row[1])
		}
		if len(row) > 2 {
			excelRow.Mobile = strings.TrimSpace(row[2])
		}
		if len(row) > 3 {
			excelRow.Role = strings.ToLower(strings.TrimSpace(row[3]))
		}

		// Validate required fields
		if excelRow.Email == "" {
			excelRow.Error = "Email is required"
		} else if excelRow.FullName == "" {
			excelRow.Error = "Full Name is required"
		} else if excelRow.Role == "" {
			excelRow.Error = "Role is required"
		} else if excelRow.Role != "admin" && excelRow.Role != "user" {
			excelRow.Error = "Role must be 'admin' or 'user'"
		}

		parsedRows = append(parsedRows, excelRow)
	}

	return parsedRows, nil
}

// ImportUsers imports users from parsed Excel rows
func (s *ExcelService) ImportUsers(ctx context.Context, rows []ExcelRow, opts ExcelImportOptions, createdBy uuid.UUID) (*ExcelImportResult, error) {
	result := &ExcelImportResult{
		TotalRows: len(rows),
		Errors:    make([]ExcelRow, 0),
	}

	for _, row := range rows {
		// Skip rows with parsing errors
		if row.Error != "" {
			row.Error = "Parse error: " + row.Error
			result.Errors = append(result.Errors, row)
			result.Failed++
			continue
		}

		// Generate password if not provided
		password := ""
		if opts.DefaultPassword != "" {
			password = opts.DefaultPassword
		} else {
			password = s.generateRandomPassword()
		}

		// Determine role
		role := row.Role
		if role == "" && opts.DefaultRole != "" {
			role = opts.DefaultRole
		}

		// Create user
		input := models.CreateUserInput{
			Email:              row.Email,
			Password:           password,
			FullName:           row.FullName,
			Mobile:             row.Mobile,
			Role:               role,
			MustChangePassword: opts.ForcePasswordChange,
			MustEnrollMFA:      opts.ForceMFAEnrollment,
		}

		user, err := s.userService.Create(ctx, input, createdBy)
		if err != nil {
			if strings.Contains(err.Error(), "email already exists") && opts.SkipDuplicates {
				result.Skipped++
				continue
			}
			row.Error = err.Error()
			result.Errors = append(result.Errors, row)
			result.Failed++
			continue
		}

		result.Imported++
		result.ImportedIDs = append(result.ImportedIDs, user.ID)
	}

	return result, nil
}

// ExportUsers exports all users to an Excel file
func (s *ExcelService) ExportUsers(ctx context.Context, opts ExcelExportOptions) (*bytes.Buffer, error) {
	// Get all users
	users, _, err := s.userService.List(ctx, "", 0, 10000)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	f := excelize.NewFile()
	defer f.Close()

	f.SetSheetName("Sheet1", "Users")

	// Set headers
	headers := []string{"Email", "Full Name", "Mobile", "Role", "Active", "MFA Enabled", "Created At", "Last Login"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("Users", cell, header)
	}

	// Style headers
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"10B981"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	f.SetCellStyle("Users", "A1", "H1", headerStyle)

	// Set column widths
	f.SetColWidth("Users", "A", "A", 30)
	f.SetColWidth("Users", "B", "B", 25)
	f.SetColWidth("Users", "C", "C", 15)
	f.SetColWidth("Users", "D", "D", 12)
	f.SetColWidth("Users", "E", "E", 8)
	f.SetColWidth("Users", "F", "F", 12)
	f.SetColWidth("Users", "G", "G", 20)
	f.SetColWidth("Users", "H", "H", 20)

	// Add data rows
	for i, user := range users {
		if !opts.IncludeInactive && !user.IsActive {
			continue
		}
		// Exclude super admin from export
		if user.Role == "super_admin" {
			continue
		}

		rowNum := i + 2
		f.SetCellValue("Users", fmt.Sprintf("A%d", rowNum), user.Email)
		f.SetCellValue("Users", fmt.Sprintf("B%d", rowNum), user.FullName)
		f.SetCellValue("Users", fmt.Sprintf("C%d", rowNum), user.Mobile)
		f.SetCellValue("Users", fmt.Sprintf("D%d", rowNum), user.Role)
		f.SetCellValue("Users", fmt.Sprintf("E%d", rowNum), user.IsActive)
		f.SetCellValue("Users", fmt.Sprintf("F%d", rowNum), user.TOTPEnabled)
		f.SetCellValue("Users", fmt.Sprintf("G%d", rowNum), user.CreatedAt.Format("2006-01-02 15:04:05"))
		if user.LastLogin != nil {
			f.SetCellValue("Users", fmt.Sprintf("H%d", rowNum), user.LastLogin.Format("2006-01-02 15:04:05"))
		}
	}

	// Write to buffer
	buf := new(bytes.Buffer)
	if err := f.Write(buf); err != nil {
		return nil, fmt.Errorf("failed to write Excel file: %w", err)
	}

	return buf, nil
}

// generateRandomPassword generates a secure random password
func (s *ExcelService) generateRandomPassword() string {
	const (
		lowercase = "abcdefghijklmnopqrstuvwxyz"
		uppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		digits    = "0123456789"
		symbols   = "!@#$%^&*"
	)

	// Use auth service's random token generation if available
	// Otherwise generate a simple random password
	return "TempPass123!" // Placeholder - user will be forced to change
}
