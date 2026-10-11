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

type VehicleHandler struct {
	service *service.VehicleService
}

func NewVehicleHandler(s *service.VehicleService) *VehicleHandler {
	return &VehicleHandler{service: s}
}

func (h *VehicleHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/vehicles", h.Create)
	rg.GET("/vehicles", h.List)
	rg.GET("/vehicles/:id", h.Get)
	rg.PATCH("/vehicles/:id", h.Update)
	rg.DELETE("/vehicles/:id", h.Delete)
}

// tenantID is a stand-in until OAuth2 (C16) supplies it from a verified JWT.
// Reading it from a header keeps the API testable end-to-end right now;
// B5 (tenant isolation) will replace this with middleware that derives the
// tenant from the authenticated session instead of trusting a client header.
func tenantID(c *gin.Context) (uuid.UUID, bool) {
	raw := c.GetHeader("X-Tenant-ID")
	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing or invalid X-Tenant-ID header"})
		return uuid.Nil, false
	}
	return id, true
}

type createVehicleRequest struct {
	PlateNumber string `json:"plate_number" binding:"required"`
	Model       string `json:"model"`
}

func (h *VehicleHandler) Create(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	var req createVehicleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	v, err := h.service.Create(c.Request.Context(), service.CreateVehicleInput{
		TenantID:    tID,
		PlateNumber: req.PlateNumber,
		Model:       req.Model,
	})
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, v)
}

func (h *VehicleHandler) Get(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	v, err := h.service.Get(c.Request.Context(), tID, id)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, v)
}

func (h *VehicleHandler) List(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.service.List(c.Request.Context(), tID, page, pageSize)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

type updateVehicleRequest struct {
	PlateNumber *string               `json:"plate_number"`
	Model       *string               `json:"model"`
	Status      *domain.VehicleStatus `json:"status"`
}

func (h *VehicleHandler) Update(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req updateVehicleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	v, err := h.service.Update(c.Request.Context(), tID, id, service.UpdateVehicleInput{
		PlateNumber: req.PlateNumber,
		Model:       req.Model,
		Status:      req.Status,
	})
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, v)
}

func (h *VehicleHandler) Delete(c *gin.Context) {
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
		respondServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// respondServiceError maps known service/repository sentinel errors to the
// right HTTP status. Anything unrecognized is a 500 — we don't leak
// internal error details to the client.
func respondServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, service.ErrValidation):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
