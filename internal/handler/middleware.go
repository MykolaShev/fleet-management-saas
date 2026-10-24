package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MykolaShev/fleet-management-saas/internal/auth"
)

const (
	tenantContextKey = "tenant_id"
	userContextKey   = "user_id"
	roleContextKey   = "role"
)

// Authenticate validates the session JWT issued by /auth/callback (the
// real Google login) or, locally, /auth/dev-token, and makes the caller's
// identity available to handlers via tenantID(c).
//
// This replaces the earlier EnsureTenant stand-in, which trusted a raw
// X-Tenant-ID header and had to provision a Tenant row on every request to
// guard against a bad/unknown ID. Authenticate does zero database work per
// request — tenant/user provisioning now happens exactly once, at login —
// because the JWT itself is the proof that provisioning already happened.
func Authenticate(issuer *auth.JWTIssuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractToken(c)
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid Authorization header"})
			return
		}

		claims, err := issuer.Parse(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set(tenantContextKey, claims.TenantID)
		c.Set(userContextKey, claims.UserID)
		c.Set(roleContextKey, claims.Role)
		c.Next()
	}
}

// extractToken reads the bearer token from the Authorization header, or
// falls back to a ?token= query parameter. The fallback exists only for
// Server-Sent Events endpoints (see AgentHandler.Chat): a browser's native
// EventSource API can't set custom headers, so there's no other way for
// it to authenticate. A query-param token ends up in more places (server
// access logs, browser history) than a header does, so this fallback
// should only ever be relied on where it's genuinely unavoidable — not
// treated as an equally safe alternative everywhere.
func extractToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if trimmed := strings.TrimPrefix(header, "Bearer "); trimmed != "" && trimmed != header {
		return trimmed
	}
	return c.Query("token")
}

// tenantID retrieves the tenant id that Authenticate already validated and
// stored on the context. Handlers never parse a header or token
// themselves — this keeps "what counts as a valid caller" defined in
// exactly one place.
func tenantID(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(tenantContextKey)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant context missing (Authenticate middleware not applied)"})
		return uuid.Nil, false
	}
	return v.(uuid.UUID), true
}
