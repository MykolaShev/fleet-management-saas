package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/service"
)

type DeliveryHandler struct {
	service *service.DeliveryService
}

func NewDeliveryHandler(s *service.DeliveryService) *DeliveryHandler {
	return &DeliveryHandler{service: s}
}

func (h *DeliveryHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/deliveries", h.Create)
	rg.GET("/deliveries", h.List)
	rg.GET("/deliveries/:id", h.Get)
	rg.PATCH("/deliveries/:id", h.Update)
	rg.DELETE("/deliveries/:id", h.Delete)
}

type createDeliveryRequest struct {
	PickupAddress  string     `json:"pickup_address" binding:"required"`
	DropoffAddress string     `json:"dropoff_address" binding:"required"`
	VehicleID      *uuid.UUID `json:"vehicle_id"`
	DriverID       *uuid.UUID `json:"driver_id"`
}

func (h *DeliveryHandler) Create(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	var req createDeliveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	d, err := h.service.Create(c.Request.Context(), service.CreateDeliveryInput{
		TenantID:       tID,
		VehicleID:      req.VehicleID,
		DriverID:       req.DriverID,
		PickupAddress:  req.PickupAddress,
		DropoffAddress: req.DropoffAddress,
	})
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, d)
}

func (h *DeliveryHandler) Get(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	d, err := h.service.Get(c.Request.Context(), tID, id)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, d)
}

func (h *DeliveryHandler) List(c *gin.Context) {
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

// updateDeliveryRequest exposes explicit unassign_vehicle/unassign_driver
// flags (mirroring UpdateDeliveryInput) so a client can distinguish "leave
// this relation as-is" from "clear it" instead of overloading a null vs
// missing-field JSON convention.
type updateDeliveryRequest struct {
	Status          *domain.DeliveryStatus `json:"status"`
	VehicleID       *uuid.UUID             `json:"vehicle_id"`
	UnassignVehicle bool                   `json:"unassign_vehicle"`
	DriverID        *uuid.UUID             `json:"driver_id"`
	UnassignDriver  bool                   `json:"unassign_driver"`
	PickupAddress   *string                `json:"pickup_address"`
	DropoffAddress  *string                `json:"dropoff_address"`
}

func (h *DeliveryHandler) Update(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req updateDeliveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	d, err := h.service.Update(c.Request.Context(), tID, id, service.UpdateDeliveryInput{
		Status:          req.Status,
		VehicleID:       req.VehicleID,
		UnassignVehicle: req.UnassignVehicle,
		DriverID:        req.DriverID,
		UnassignDriver:  req.UnassignDriver,
		PickupAddress:   req.PickupAddress,
		DropoffAddress:  req.DropoffAddress,
	})
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, d)
}

func (h *DeliveryHandler) Delete(c *gin.Context) {
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
