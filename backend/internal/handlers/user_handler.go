package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/srams/backend/internal/interfaces"
	"github.com/srams/backend/internal/middleware"
	"github.com/srams/backend/internal/models"
)

type UserHandler struct {
	userService  interfaces.UserService
	auditService interfaces.AuditService
}

func NewUserHandler(userService interfaces.UserService, auditService interfaces.AuditService) *UserHandler {
	return &UserHandler{
		userService:  userService,
		auditService: auditService,
	}
}

func (h *UserHandler) Create(c *gin.Context) {
	var req models.CreateUserInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	currentUser := middleware.GetUser(c)
	meta := middleware.GetRequestMetadata(c)

	// Only admins can create users
	if currentUser.Role == models.RoleUser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	// Only super admin can create admins
	if req.Role == models.RoleAdmin && currentUser.Role != models.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only super admin can create admins"})
		return
	}

	user, err := h.userService.Create(c.Request.Context(), req, currentUser.ID)
	if err != nil {
		if err.Error() == "email already exists" {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		if err.Error() == "mobile number already exists" {
			c.JSON(http.StatusConflict, gin.H{"error": "Mobile number already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Log action
	h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
		ActorID:    &currentUser.ID,
		ActorRole:  currentUser.Role,
		ActionType: models.ActionUserCreate,
		TargetType: "user",
		TargetID:   &user.ID,
		Metadata:   map[string]interface{}{"email": user.Email, "role": user.Role},
		IPAddress:  meta.IP,
		DeviceID:   meta.DeviceID,
		UserAgent:  meta.UserAgent,
	})

	c.JSON(http.StatusCreated, user)
}

func (h *UserHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := h.userService.GetByID(c.Request.Context(), id)
	if err != nil {
		if err.Error() == "user not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) GetDashboardStats(c *gin.Context) {
	stats, err := h.userService.GetDashboardStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get dashboard stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) List(c *gin.Context) {
	role := c.Query("role")
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if limit > 100 {
		limit = 100
	}

	users, total, err := h.userService.List(c.Request.Context(), role, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"total": total,
	})
}

func (h *UserHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req models.UpdateUserInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	currentUser := middleware.GetUser(c)
	meta := middleware.GetRequestMetadata(c)

	// Only admin/super_admin can update users
	if currentUser.Role == models.RoleUser {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	user, err := h.userService.Update(c.Request.Context(), id, req, currentUser.ID, currentUser.Role)
	if err != nil {
		if err.Error() == "mobile number already exists" {
			c.JSON(http.StatusConflict, gin.H{"error": "Mobile number already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	// Log action
	h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
		ActorID:    &currentUser.ID,
		ActorRole:  currentUser.Role,
		ActionType: models.ActionUserUpdate,
		TargetType: "user",
		TargetID:   &user.ID,
		Metadata:   map[string]interface{}{"updated_user_email": user.Email},
		IPAddress:  meta.IP,
		DeviceID:   meta.DeviceID,
		UserAgent:  meta.UserAgent,
	})

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	currentUser := middleware.GetUser(c)
	meta := middleware.GetRequestMetadata(c)

	// Only super admin can delete users
	if currentUser.Role != models.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only super admin can delete users"})
		return
	}

	err = h.userService.Delete(c.Request.Context(), id, currentUser.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Log action
	h.auditService.Create(c.Request.Context(), models.CreateAuditLogInput{
		ActorID:    &currentUser.ID,
		ActorRole:  currentUser.Role,
		ActionType: models.ActionUserDelete,
		TargetType: "user",
		TargetID:   &id, // Use the ID variable directly
		Metadata:   map[string]interface{}{"deleted_user_id": id.String()},
		IPAddress:  meta.IP,
		DeviceID:   meta.DeviceID,
		UserAgent:  meta.UserAgent,
	})

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}
