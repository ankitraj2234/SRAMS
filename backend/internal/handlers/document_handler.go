package handlers

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/srams/backend/internal/interfaces"
	"github.com/srams/backend/internal/middleware"
	"github.com/srams/backend/internal/models"
	"github.com/srams/backend/internal/services"
)

type DocumentHandler struct {
	docService   interfaces.DocumentService
	auditService interfaces.AuditService
}

func NewDocumentHandler(docService interfaces.DocumentService, auditService interfaces.AuditService) *DocumentHandler {
	return &DocumentHandler{
		docService:   docService,
		auditService: auditService,
	}
}

func (h *DocumentHandler) Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	title := c.PostForm("title")
	if title == "" {
		title = header.Filename
	}

	currentUser := middleware.GetUser(c)
	meta := middleware.GetRequestMetadata(c)

	doc, err := h.docService.Upload(c.Request.Context(), models.UploadDocumentInput{
		Title:    title,
		Filename: header.Filename,
		Content:  file,
		FileSize: header.Size,
	}, currentUser.ID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload document"})
		return
	}

	// Log action
	h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
		ActorID:    &currentUser.ID,
		ActorRole:  currentUser.Role,
		ActionType: models.ActionDocUpload,
		TargetType: "document",
		TargetID:   &doc.ID,
		Metadata:   map[string]interface{}{"filename": doc.Filename, "size": doc.FileSize},
		IPAddress:  meta.IP,
		DeviceID:   meta.DeviceID,
		UserAgent:  meta.UserAgent,
	})

	c.JSON(http.StatusCreated, doc)
}

func (h *DocumentHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document ID"})
		return
	}

	doc, err := h.docService.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == services.ErrDocumentNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get document"})
		return
	}

	c.JSON(http.StatusOK, doc)
}

func (h *DocumentHandler) List(c *gin.Context) {
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if limit > 100 {
		limit = 100
	}

	docs, total, err := h.docService.List(c.Request.Context(), offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list documents"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"documents": docs,
		"total":     total,
		"offset":    offset,
		"limit":     limit,
	})
}

func (h *DocumentHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document ID"})
		return
	}

	currentUser := middleware.GetUser(c)
	meta := middleware.GetRequestMetadata(c)

	err = h.docService.Delete(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete document"})
		return
	}

	// Log action
	h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
		ActorID:    &currentUser.ID,
		ActorRole:  currentUser.Role,
		ActionType: models.ActionDocDelete,
		TargetType: "document",
		TargetID:   &id,
		IPAddress:  meta.IP,
		DeviceID:   meta.DeviceID,
		UserAgent:  meta.UserAgent,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Document deleted"})
}

// User's assigned documents
func (h *DocumentHandler) MyDocuments(c *gin.Context) {
	currentUser := middleware.GetUser(c)
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	docs, total, err := h.docService.GetUserDocuments(c.Request.Context(), currentUser.ID, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get documents"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"documents": docs,
		"total":     total,
		"offset":    offset,
		"limit":     limit,
	})
}

// Secure document viewing - returns PDF data for internal viewer
func (h *DocumentHandler) View(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document ID"})
		return
	}

	currentUser := middleware.GetUser(c)

	// Check access
	hasAccess, err := h.docService.HasAccess(c.Request.Context(), id, currentUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check access"})
		return
	}
	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Start view session
	viewID, err := h.docService.StartView(c.Request.Context(), id, currentUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start view session"})
		return
	}

	// Log document open
	h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
		ActorID:    &currentUser.ID,
		ActorRole:  currentUser.Role,
		ActionType: models.ActionDocumentOpen,
		TargetType: "document",
		TargetID:   &id,
		Metadata:   map[string]interface{}{"view_id": viewID.String()},
		IPAddress:  c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
	})

	// Get file content
	content, err := h.docService.GetFileContent(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read document"})
		return
	}
	defer content.Close()

	// Set headers
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "inline")
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
	c.Header("X-View-ID", viewID.String())

	// Stream file
	io.Copy(c.Writer, content)
}

// End document view session
func (h *DocumentHandler) EndView(c *gin.Context) {
	viewID, err := uuid.Parse(c.Param("viewId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid view ID"})
		return
	}

	var req struct {
		TotalSeconds int `json:"total_seconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Use EndView (Postgres/Interface style)
	err = h.docService.EndView(c.Request.Context(), viewID, req.TotalSeconds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to end view session"})
		return
	}

	currentUser := middleware.GetUser(c)
	// Log action
	h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
		ActorID:    &currentUser.ID,
		ActorRole:  currentUser.Role,
		ActionType: models.ActionDocumentClose,
		TargetType: "document_view",
		TargetID:   &viewID,
		Metadata:   map[string]interface{}{"total_seconds": req.TotalSeconds},
	})

	c.JSON(http.StatusOK, gin.H{"message": "View session ended"})
}

// Record page view
func (h *DocumentHandler) RecordPageView(c *gin.Context) {
	viewID, err := uuid.Parse(c.Param("viewId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid view ID"})
		return
	}

	pageNumber, err := strconv.Atoi(c.Param("page"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page number"})
		return
	}

	h.docService.RecordPageView(c.Request.Context(), viewID, pageNumber)
	c.JSON(http.StatusOK, gin.H{"message": "Page view recorded"})
}

// GetDocumentAccess returns list of users with access to a document
func (h *DocumentHandler) GetDocumentAccess(c *gin.Context) {
	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document ID"})
		return
	}

	users, err := h.docService.GetDocumentAccessList(c.Request.Context(), docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get document access"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

// Access control
func (h *DocumentHandler) GrantAccess(c *gin.Context) {
	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document ID"})
		return
	}

	var req struct {
		UserID      string `json:"user_id" binding:"required"`
		CanReassign bool   `json:"can_reassign"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	currentUser := middleware.GetUser(c)
	meta := middleware.GetRequestMetadata(c)

	// Check permission to re-assign
	// If currentUser is NOT super_admin, they must have CanReassign permission on this doc
	if currentUser.Role != models.RoleSuperAdmin {
		canReassign, err := h.docService.CanReassign(c.Request.Context(), docID, currentUser.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
			return
		}
		if !canReassign {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to assign users to this document"})
			return
		}
	}

	err = h.docService.GrantAccess(c.Request.Context(), docID, userID, currentUser.ID, req.CanReassign)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to grant access"})
		return
	}

	// Log action
	h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
		ActorID:    &currentUser.ID,
		ActorRole:  currentUser.Role,
		ActionType: models.ActionAccessGrant,
		TargetType: "document",
		TargetID:   &docID,
		Metadata:   map[string]interface{}{"granted_to": userID.String()},
		IPAddress:  meta.IP,
		DeviceID:   meta.DeviceID,
		UserAgent:  meta.UserAgent,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Access granted"})
}

func (h *DocumentHandler) RevokeAccess(c *gin.Context) {
	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document ID"})
		return
	}

	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	currentUser := middleware.GetUser(c)
	meta := middleware.GetRequestMetadata(c)

	err = h.docService.RevokeAccess(c.Request.Context(), docID, userID, currentUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke access"})
		return
	}

	// Log action
	h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
		ActorID:    &currentUser.ID,
		ActorRole:  currentUser.Role,
		ActionType: models.ActionAccessRevoke,
		TargetType: "document",
		TargetID:   &docID,
		Metadata:   map[string]interface{}{"revoked_from": userID.String()},
		IPAddress:  meta.IP,
		DeviceID:   meta.DeviceID,
		UserAgent:  meta.UserAgent,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Access revoked"})
}

// Document requests
func (h *DocumentHandler) CreateRequest(c *gin.Context) {
	var req struct {
		DocumentID   *string `json:"document_id"`
		DocumentName string  `json:"document_name"`
		Reason       string  `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var docID *uuid.UUID
	if req.DocumentID != nil && *req.DocumentID != "" {
		id, err := uuid.Parse(*req.DocumentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document ID"})
			return
		}
		docID = &id
	}

	if docID == nil && req.DocumentName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Document ID or Name is required"})
		return
	}

	currentUser := middleware.GetUser(c)

	request, err := h.docService.CreateRequest(c.Request.Context(), currentUser.ID, docID, req.DocumentName, req.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, request)
}

func (h *DocumentHandler) GetMyRequests(c *gin.Context) {
	currentUser := middleware.GetUser(c)
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	requests, total, err := h.docService.GetUserRequests(c.Request.Context(), currentUser.ID, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get requests"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"requests": requests,
		"total":    total,
		"offset":   offset,
		"limit":    limit,
	})
}

func (h *DocumentHandler) GetPendingRequests(c *gin.Context) {
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	requests, total, err := h.docService.GetPendingRequests(c.Request.Context(), offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get requests"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"requests": requests,
		"total":    total,
		"offset":   offset,
		"limit":    limit,
	})
}

func (h *DocumentHandler) ApproveRequest(c *gin.Context) {
	requestID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	var req struct {
		Note string `json:"note"`
	}
	c.ShouldBindJSON(&req)

	currentUser := middleware.GetUser(c)
	meta := middleware.GetRequestMetadata(c)

	err = h.docService.ApproveRequest(c.Request.Context(), requestID, currentUser.ID, req.Note)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Log action
	h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
		ActorID:    &currentUser.ID,
		ActorRole:  currentUser.Role,
		ActionType: models.ActionAdminApproval,
		TargetType: "request",
		TargetID:   &requestID,
		Metadata:   map[string]interface{}{"action": "approved", "note": req.Note},
		IPAddress:  meta.IP,
		DeviceID:   meta.DeviceID,
		UserAgent:  meta.UserAgent,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Request approved"})
}

func (h *DocumentHandler) RejectRequest(c *gin.Context) {
	requestID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	var req struct {
		Note string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rejection reason required"})
		return
	}

	currentUser := middleware.GetUser(c)
	meta := middleware.GetRequestMetadata(c)

	err = h.docService.RejectRequest(c.Request.Context(), requestID, currentUser.ID, req.Note)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Log action
	h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
		ActorID:    &currentUser.ID,
		ActorRole:  currentUser.Role,
		ActionType: models.ActionAdminApproval,
		TargetType: "request",
		TargetID:   &requestID,
		Metadata:   map[string]interface{}{"action": "rejected", "note": req.Note},
		IPAddress:  meta.IP,
		DeviceID:   meta.DeviceID,
		UserAgent:  meta.UserAgent,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Request rejected"})
}
