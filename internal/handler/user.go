package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/repository"
	"github.com/MykolaShev/fleet-management-saas/internal/service"
)

type UserHandler struct {
	service *service.UserService
}

func NewUserHandler(s *service.UserService) *UserHandler {
	return &UserHandler{service: s}
}

func (h *UserHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/users", h.Create)
	rg.GET("/users", h.List)
	rg.GET("/users/:id", h.Get)
	rg.PATCH("/users/:id", h.Update)
	rg.DELETE("/users/:id", h.Delete)
}

type createUserRequest struct {
	Email string          `json:"email" binding:"required"`
	Name  string          `json:"name"`
	Role  domain.UserRole `json:"role" binding:"required"`
}

func (h *UserHandler) Create(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, err := h.service.Create(c.Request.Context(), service.CreateUserInput{
		TenantID: tID,
		Email:    req.Email,
		Name:     req.Name,
		Role:     req.Role,
	})
	if err != nil {
		respondUserServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, u)
}

func (h *UserHandler) Get(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	u, err := h.service.Get(c.Request.Context(), tID, id)
	if err != nil {
		respondUserServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, u)
}

func (h *UserHandler) List(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.service.List(c.Request.Context(), tID, page, pageSize)
	if err != nil {
		respondUserServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// updateUserRequest intentionally has no Email field: email doubles as the
// per-tenant login identity, and changing it is left out of scope for now
// (it would need re-checking the uniqueness constraint and, later, syncing
// with the OAuth2 identity).
type updateUserRequest struct {
	Name *string          `json:"name"`
	Role *domain.UserRole `json:"role"`
}

func (h *UserHandler) Update(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, err := h.service.Update(c.Request.Context(), tID, id, service.UpdateUserInput{
		Name: req.Name,
		Role: req.Role,
	})
	if err != nil {
		respondUserServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, u)
}

func (h *UserHandler) Delete(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), tID, id); err != nil {
		respondUserServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// respondUserServiceError additionally maps ErrConflict to 409, which the
// Vehicle flow never produces (vehicles have no uniqueness constraint).
func respondUserServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "a user with this email already exists in this tenant"})
	default:
		respondServiceError(c, err)
	}
}
