package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/service"
)

type FeatureFlagHandler struct {
	service *service.FeatureFlagService
}

func NewFeatureFlagHandler(s *service.FeatureFlagService) *FeatureFlagHandler {
	return &FeatureFlagHandler{service: s}
}

// No POST/DELETE here on purpose — see FeatureFlagRepository's doc comment.
// The catalog of possible flags is fixed by the system (domain.KnownFeatureFlags);
// a tenant can only read the current state or flip one, never create or
// remove a flag.
func (h *FeatureFlagHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/feature-flags", h.List)
	rg.PATCH("/feature-flags/:key", h.Set)
}

func (h *FeatureFlagHandler) List(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	flags, err := h.service.List(c.Request.Context(), tID)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"flags": flags})
}

type setFeatureFlagRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *FeatureFlagHandler) Set(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	key := domain.FeatureFlagKey(c.Param("key"))

	var req setFeatureFlagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	flag, err := h.service.Set(c.Request.Context(), tID, key, req.Enabled)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, flag)
}
