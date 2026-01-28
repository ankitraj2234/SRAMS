package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/srams/backend/internal/interfaces"
	"github.com/srams/backend/internal/middleware"
	"github.com/srams/backend/internal/models"
)

type AuditHandler struct {
	auditService interfaces.AuditService
}

func NewAuditHandler(auditService interfaces.AuditService) *AuditHandler {
	return &AuditHandler{auditService: auditService}
}

func (h *AuditHandler) List(c *gin.Context) {
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	if limit > 100 {
		limit = 100
	}

	filter := models.AuditLogFilter{
		Offset: offset,
		Limit:  limit,
	}

	// Parse filters
	if actorID := c.Query("actor_id"); actorID != "" {
		if id, err := uuid.Parse(actorID); err == nil {
			filter.ActorID = &id
		}
	}

	filter.ActionType = c.Query("action_type")
	filter.TargetType = c.Query("target_type")

	if startDate := c.Query("start_date"); startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			filter.StartDate = &t
		}
	}

	if endDate := c.Query("end_date"); endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			t = t.Add(24*time.Hour - time.Second) // End of day
			filter.EndDate = &t
		}
	}

	// Only super admin can see deleted logs
	currentUser := middleware.GetUser(c)
	if currentUser.Role == models.RoleSuperAdmin && c.Query("include_deleted") == "true" {
		filter.IncludeDeleted = true
	}

	logs, total, err := h.auditService.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list audit logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":   logs,
		"total":  total,
		"offset": offset,
		"limit":  limit,
	})
}

func (h *AuditHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log ID"})
		return
	}

	log, err := h.auditService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Log not found"})
		return
	}

	c.JSON(http.StatusOK, log)
}

// Super Admin only
func (h *AuditHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log ID"})
		return
	}

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Deletion reason is required"})
		return
	}

	currentUser := middleware.GetUser(c)
	meta := middleware.GetRequestMetadata(c)

	err = h.auditService.Delete(c.Request.Context(), id, currentUser.ID, req.Reason, currentUser.Role)
	if err != nil {
		if err.Error() == "only super admin can perform this action" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only super admin can delete logs"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Log the deletion itself
	h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
		ActorID:    &currentUser.ID,
		ActorRole:  currentUser.Role,
		ActionType: models.ActionLogDelete,
		TargetType: "audit_log",
		TargetID:   &id,
		Metadata:   map[string]interface{}{"reason": req.Reason},
		IPAddress:  meta.IP,
		DeviceID:   meta.DeviceID,
		UserAgent:  meta.UserAgent,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Log deleted"})
}

func (h *AuditHandler) BulkDelete(c *gin.Context) {
	var req struct {
		LogIDs []string `json:"log_ids" binding:"required"`
		Reason string   `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var ids []uuid.UUID
	for _, idStr := range req.LogIDs {
		if id, err := uuid.Parse(idStr); err == nil {
			ids = append(ids, id)
		}
	}

	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid log IDs"})
		return
	}

	currentUser := middleware.GetUser(c)
	meta := middleware.GetRequestMetadata(c)

	count, err := h.auditService.BulkDelete(c.Request.Context(), ids, currentUser.ID, req.Reason, currentUser.Role)
	if err != nil {
		if err.Error() == "only super admin can perform this action" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only super admin can delete logs"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Log the bulk deletion
	h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
		ActorID:    &currentUser.ID,
		ActorRole:  currentUser.Role,
		ActionType: models.ActionLogDelete,
		TargetType: "audit_logs",
		Metadata:   map[string]interface{}{"count": count, "reason": req.Reason},
		IPAddress:  meta.IP,
		DeviceID:   meta.DeviceID,
		UserAgent:  meta.UserAgent,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Logs deleted", "count": count})
}

func (h *AuditHandler) GetActionTypes(c *gin.Context) {
	types, err := h.auditService.GetActionTypes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get action types"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"action_types": types})
}

func (h *AuditHandler) GetStats(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	since := time.Now().AddDate(0, 0, -days)

	stats, err := h.auditService.GetStatsByAction(c.Request.Context(), since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stats": stats, "since": since})
}

func (h *AuditHandler) GetUserTimeline(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	timeline, err := h.auditService.GetUserActivityTimeline(c.Request.Context(), userID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get timeline"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"timeline": timeline})
}

// Log client-side events
func (h *AuditHandler) LogEvent(c *gin.Context) {
	var req struct {
		ActionType string                 `json:"action_type" binding:"required"`
		TargetType string                 `json:"target_type"`
		TargetID   string                 `json:"target_id"`
		Metadata   map[string]interface{} `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	currentUser := middleware.GetUser(c)
	meta := middleware.GetRequestMetadata(c)

	input := models.CreateAuditLogInput{
		ActorID:    &currentUser.ID,
		ActorRole:  currentUser.Role,
		ActionType: req.ActionType,
		TargetType: req.TargetType,
		Metadata:   req.Metadata,
		IPAddress:  meta.IP,
		DeviceID:   meta.DeviceID,
		UserAgent:  meta.UserAgent,
	}

	if req.TargetID != "" {
		if id, err := uuid.Parse(req.TargetID); err == nil {
			input.TargetID = &id
		}
	}

	_, err := h.auditService.Create(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log event"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Event logged"})
}
func (h *AuditHandler) Export(c *gin.Context) {
	// Simple CSV export implementation based on List
	c.Header("Content-Disposition", "attachment; filename=audit_logs.csv")
	c.Header("Content-Type", "text/csv")

	// Default limit for export
	limit := 1000
	filter := models.AuditLogFilter{
		Limit: limit,
	}

	logs, _, err := h.auditService.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch logs"})
		return
	}

	// Write CSV header
	c.Writer.Write([]byte("ID,Action,Actor,Target,IP,Date\n"))
	for _, l := range logs {
		role := l.ActorRole
		if l.ActorRole == "" {
			role = "system"
		}
		line := fmt.Sprintf("%s,%s,%s,%s,%s,%s\n",
			l.ID, l.ActionType, role, l.TargetType, l.IPAddress, l.CreatedAt.Format(time.RFC3339))
		c.Writer.Write([]byte(line))
	}
}
