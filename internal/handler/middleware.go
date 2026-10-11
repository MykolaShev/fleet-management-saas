package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MykolaShev/fleet-management-saas/internal/repository"
)

const tenantContextKey = "tenant_id"

// EnsureTenant is a stand-in until OAuth2 (C16) supplies the tenant from a
// verified JWT. Reading it from a header keeps the API testable
// end-to-end right now; B5 (tenant isolation) will replace this with
// middleware that derives the tenant from the authenticated session
// instead of trusting a client header.
//
// It also guarantees a matching Tenant row exists before any handler
// runs — Vehicle/User/Delivery all have a real foreign key to
// tenants.id, so without this, the very first insert for a brand-new
// tenant ID would fail with a foreign-key violation (as it did before
// this middleware existed).
func EnsureTenant(repo *repository.TenantRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("X-Tenant-ID")
		id, err := uuid.Parse(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing or invalid X-Tenant-ID header"})
			return
		}

		if err := repo.FindOrCreate(c.Request.Context(), id); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.Set(tenantContextKey, id)
		c.Next()
	}
}

// tenantID retrieves the tenant id that EnsureTenant already validated,
// provisioned, and stored on the context. Handlers no longer parse the
// header themselves — this keeps "what counts as a valid tenant" defined
// in exactly one place.
func tenantID(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(tenantContextKey)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant context missing (EnsureTenant middleware not applied)"})
		return uuid.Nil, false
	}
	return v.(uuid.UUID), true
}
