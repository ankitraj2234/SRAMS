package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/srams/backend/internal/models"
)

var (
	ErrDocumentNotFound  = errors.New("document not found")
	ErrNoAccess          = errors.New("no access to document")
	ErrRequestNotFound   = errors.New("request not found")
	ErrRequestNotPending = errors.New("request is not pending")
)

type DocumentService struct {
	db          *sql.DB
	storagePath string
}

func NewDocumentService(db *sql.DB, storagePath string) *DocumentService {
	os.MkdirAll(storagePath, 0755)

	return &DocumentService{
		db:          db,
		storagePath: storagePath,
	}
}

func (s *DocumentService) Upload(ctx context.Context, input models.UploadDocumentInput, uploadedBy uuid.UUID) (*models.Document, error) {
	docID := uuid.New()
	ext := filepath.Ext(input.Filename)
	storedFilename := docID.String() + ext
	filePath := filepath.Join(s.storagePath, storedFilename)

	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	writer := io.MultiWriter(file, hasher)

	bytesWritten, err := io.Copy(writer, input.Content)
	if err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	fileHash := hex.EncodeToString(hasher.Sum(nil))
	now := time.Now().UTC().Format(time.RFC3339)

	doc := &models.Document{
		ID:         docID,
		Title:      input.Title,
		Filename:   input.Filename,
		FilePath:   filePath,
		FileHash:   fileHash,
		FileSize:   bytesWritten,
		UploadedBy: uploadedBy,
		CreatedAt:  time.Now(),
		IsActive:   true,
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO documents (id, title, filename, file_path, file_hash, file_size, uploaded_by, created_at, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, doc.ID.String(), doc.Title, doc.Filename, doc.FilePath, doc.FileHash, doc.FileSize, doc.UploadedBy.String(), now, 1)

	if err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to insert document: %w", err)
	}

	return doc, nil
}

func (s *DocumentService) GetByID(ctx context.Context, id uuid.UUID) (*models.Document, error) {
	doc := &models.Document{}
	var isActive int
	var createdAt sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, title, filename, file_path, file_hash, file_size, uploaded_by, created_at, is_active
		FROM documents WHERE id = ? AND is_active = 1
	`, id.String()).Scan(&doc.ID, &doc.Title, &doc.Filename, &doc.FilePath, &doc.FileHash, &doc.FileSize, &doc.UploadedBy, &createdAt, &isActive)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	doc.IsActive = isActive == 1
	if createdAt.Valid {
		if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
			doc.CreatedAt = t
		}
	}

	return doc, nil
}

func (s *DocumentService) List(ctx context.Context, offset, limit int) ([]*models.Document, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM documents WHERE is_active = 1").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, filename, file_hash, file_size, uploaded_by, created_at
		FROM documents WHERE is_active = 1
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var docs []*models.Document
	for rows.Next() {
		doc := &models.Document{}
		var createdAt sql.NullString
		err := rows.Scan(&doc.ID, &doc.Title, &doc.Filename, &doc.FileHash, &doc.FileSize, &doc.UploadedBy, &createdAt)
		if err != nil {
			return nil, 0, err
		}
		if createdAt.Valid {
			if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
				doc.CreatedAt = t
			}
		}
		docs = append(docs, doc)
	}

	return docs, total, nil
}

func (s *DocumentService) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, "UPDATE documents SET is_active = 0 WHERE id = ?", id.String())
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	return nil
}

func (s *DocumentService) GrantAccess(ctx context.Context, documentID, userID, grantedBy uuid.UUID, canReassign bool) error {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM document_access WHERE document_id = ? AND user_id = ? AND is_active = 1
	`, documentID.String(), userID.String()).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil // Already has access
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO document_access (id, document_id, user_id, granted_by, granted_at, is_active)
		VALUES (?, ?, ?, ?, ?, 1)
	`, uuid.New().String(), documentID.String(), userID.String(), grantedBy.String(), now)

	return err
}

func (s *DocumentService) RevokeAccess(ctx context.Context, documentID, userID, revokedBy uuid.UUID) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		UPDATE document_access SET is_active = 0, revoked_at = ?
		WHERE document_id = ? AND user_id = ? AND is_active = 1
	`, now, documentID.String(), userID.String())
	return err
}

func (s *DocumentService) HasAccess(ctx context.Context, documentID, userID uuid.UUID) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM document_access 
		WHERE document_id = ? AND user_id = ? AND is_active = 1
	`, documentID.String(), userID.String()).Scan(&count)
	return count > 0, err
}

// CanReassign checks if a user has permission to re-assign a document (SQLite Stub)
func (s *DocumentService) CanReassign(ctx context.Context, documentID, userID uuid.UUID) (bool, error) {
	// For SQLite, we assume only super admins or maybe simple owner check?
	// Given this is legacy/fallback, we just return false for now unless super admin logic is implemented.
	// Or we can query user role.
	// Let's implement basic super admin check if possible, or just stub default FALSE/TRUE.
	// Stub: Return false.
	return false, nil
}

// GetDocumentAccessList returns all users who have access to a document
func (s *DocumentService) GetDocumentAccessList(ctx context.Context, documentID uuid.UUID) ([]models.DocumentAccessUser, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.full_name, u.email
		FROM document_access da
		JOIN users u ON da.user_id = u.id
		WHERE da.document_id = ? AND da.is_active = 1 AND u.is_active = 1
		ORDER BY u.full_name
	`, documentID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.DocumentAccessUser
	for rows.Next() {
		var user models.DocumentAccessUser
		if err := rows.Scan(&user.ID, &user.FullName, &user.Email); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

func (s *DocumentService) GetUserDocuments(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*models.Document, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM documents d
		JOIN document_access da ON d.id = da.document_id
		WHERE da.user_id = ? AND da.is_active = 1 AND d.is_active = 1
	`, userID.String()).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id, d.title, d.filename, d.file_hash, d.file_size, d.uploaded_by, d.created_at
		FROM documents d
		JOIN document_access da ON d.id = da.document_id
		WHERE da.user_id = ? AND da.is_active = 1 AND d.is_active = 1
		ORDER BY da.granted_at DESC
		LIMIT ? OFFSET ?
	`, userID.String(), limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var docs []*models.Document
	for rows.Next() {
		doc := &models.Document{}
		var createdAt sql.NullString
		err := rows.Scan(&doc.ID, &doc.Title, &doc.Filename, &doc.FileHash, &doc.FileSize, &doc.UploadedBy, &createdAt)
		if err != nil {
			return nil, 0, err
		}
		if createdAt.Valid {
			if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
				doc.CreatedAt = t
			}
		}
		docs = append(docs, doc)
	}

	return docs, total, nil
}

func (s *DocumentService) GetFileContent(ctx context.Context, id uuid.UUID) (io.ReadCloser, error) {
	doc, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return os.Open(doc.FilePath)
}

func (s *DocumentService) GetDocumentPath(ctx context.Context, documentID, userID uuid.UUID, role string) (string, error) {
	doc, err := s.GetByID(ctx, documentID)
	if err != nil {
		return "", err
	}

	if role == models.RoleAdmin || role == models.RoleSuperAdmin {
		return doc.FilePath, nil
	}

	hasAccess, err := s.HasAccess(ctx, documentID, userID)
	if err != nil {
		return "", err
	}
	if !hasAccess {
		return "", ErrNoAccess
	}

	return doc.FilePath, nil
}

func (s *DocumentService) CreateRequest(ctx context.Context, userID uuid.UUID, documentID *uuid.UUID, documentName, reason string) (*models.UserRequest, error) {
	if documentID != nil {
		_, err := s.GetByID(ctx, *documentID)
		if err != nil {
			return nil, err
		}

		hasAccess, err := s.HasAccess(ctx, *documentID, userID)
		if err != nil {
			return nil, err
		}
		if hasAccess {
			return nil, errors.New("already has access to document")
		}

		var count int
		err = s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM document_requests WHERE user_id = ? AND document_id = ? AND status = 'pending'
		`, userID.String(), documentID.String()).Scan(&count)
		if err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New("pending request already exists for this document")
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	request := &models.UserRequest{
		ID:           uuid.New(),
		UserID:       userID,
		DocumentID:   documentID,
		DocumentName: documentName,
		RequestType:  "access",
		Status:       models.RequestStatusPending,
		Reason:       reason,
		CreatedAt:    time.Now(),
	}

	docIDStr := sql.NullString{}
	if documentID != nil {
		docIDStr.String = documentID.String()
		docIDStr.Valid = true
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO document_requests (id, user_id, document_id, document_name, request_type, status, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, request.ID.String(), request.UserID.String(), docIDStr, request.DocumentName, request.RequestType, request.Status, request.Reason, now)

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return request, nil
}

func (s *DocumentService) GetPendingRequests(ctx context.Context, offset, limit int) ([]*models.UserRequest, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM document_requests WHERE status = 'pending'").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, document_id, request_type, status, reason, created_at
		FROM document_requests WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var requests []*models.UserRequest
	for rows.Next() {
		req := &models.UserRequest{}
		var createdAt sql.NullString
		err := rows.Scan(&req.ID, &req.UserID, &req.DocumentID, &req.RequestType, &req.Status, &req.Reason, &createdAt)
		if err != nil {
			return nil, 0, err
		}
		if createdAt.Valid {
			if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
				req.CreatedAt = t
			}
		}
		requests = append(requests, req)
	}

	return requests, total, nil
}

func (s *DocumentService) ApproveRequest(ctx context.Context, requestID, reviewedBy uuid.UUID, note string) error {
	var request models.UserRequest
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, document_id, status FROM document_requests WHERE id = ?
	`, requestID.String()).Scan(&request.ID, &request.UserID, &request.DocumentID, &request.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRequestNotFound
		}
		return err
	}

	if request.Status != models.RequestStatusPending {
		return ErrRequestNotPending
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx, `
		UPDATE document_requests SET status = 'approved', reviewed_by = ?, reviewed_at = ?, review_note = ?
		WHERE id = ?
	`, reviewedBy.String(), now, note, requestID.String())
	if err != nil {
		return err
	}

	if request.DocumentID == nil {
		return errors.New("cannot approve request without document ID")
	}
	return s.GrantAccess(ctx, *request.DocumentID, request.UserID, reviewedBy, false)
}

func (s *DocumentService) RejectRequest(ctx context.Context, requestID, reviewedBy uuid.UUID, note string) error {
	var status string
	err := s.db.QueryRowContext(ctx, "SELECT status FROM document_requests WHERE id = ?", requestID.String()).Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRequestNotFound
		}
		return err
	}

	if status != models.RequestStatusPending {
		return ErrRequestNotPending
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx, `
		UPDATE document_requests SET status = 'rejected', reviewed_by = ?, reviewed_at = ?, review_note = ?
		WHERE id = ?
	`, reviewedBy.String(), now, note, requestID.String())

	return err
}

func (s *DocumentService) GetUserRequests(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*models.UserRequest, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM document_requests WHERE user_id = ?", userID.String()).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, document_id, request_type, status, reason, reviewed_by, reviewed_at, review_note, created_at
		FROM document_requests WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, userID.String(), limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var requests []*models.UserRequest
	for rows.Next() {
		req := &models.UserRequest{}
		var createdAt, reviewedAt sql.NullString
		var reviewedBy, reviewNote sql.NullString
		err := rows.Scan(&req.ID, &req.UserID, &req.DocumentID, &req.RequestType, &req.Status, &req.Reason,
			&reviewedBy, &reviewedAt, &reviewNote, &createdAt)
		if err != nil {
			return nil, 0, err
		}
		if createdAt.Valid {
			if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
				req.CreatedAt = t
			}
		}
		if reviewedAt.Valid {
			if t, err := time.Parse(time.RFC3339, reviewedAt.String); err == nil {
				req.ReviewedAt = &t
			}
		}
		if reviewedBy.Valid {
			if id, err := uuid.Parse(reviewedBy.String); err == nil {
				req.ReviewedBy = &id
			}
		}
		if reviewNote.Valid {
			req.ReviewNote = &reviewNote.String
		}
		requests = append(requests, req)
	}

	return requests, total, nil
}

func (s *DocumentService) StartView(ctx context.Context, documentID, userID uuid.UUID) (uuid.UUID, error) {
	viewID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO document_views (id, document_id, user_id, started_at, pages_viewed)
		VALUES (?, ?, ?, ?, '[]')
	`, viewID.String(), documentID.String(), userID.String(), now)
	return viewID, err
}

func (s *DocumentService) EndView(ctx context.Context, viewID uuid.UUID, totalSeconds int) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx, `
		UPDATE document_views 
		SET ended_at = ?, total_seconds = ?
		WHERE id = ?
	`, now, totalSeconds, viewID.String())

	return err
}

func (s *DocumentService) RecordPageView(ctx context.Context, viewID uuid.UUID, pageNumber int) error {
	// Get current pages_viewed
	var pagesJSON sql.NullString
	err := s.db.QueryRowContext(ctx, "SELECT pages_viewed FROM document_views WHERE id = ?", viewID.String()).Scan(&pagesJSON)
	if err != nil {
		return err
	}

	var pages []int
	if pagesJSON.Valid && pagesJSON.String != "" {
		json.Unmarshal([]byte(pagesJSON.String), &pages)
	}
	pages = append(pages, pageNumber)

	newPagesJSON, _ := json.Marshal(pages)
	_, err = s.db.ExecContext(ctx, `
		UPDATE document_views SET pages_viewed = ? WHERE id = ?
	`, string(newPagesJSON), viewID.String())
	return err
}
