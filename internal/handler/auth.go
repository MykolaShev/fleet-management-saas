package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/oauth2"

	authpkg "github.com/MykolaShev/fleet-management-saas/internal/auth"
	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/service"
)

// devTenantProvisioner is the narrow dependency DevToken needs: a brand
// new tenant_id must have a real Tenant row before anything referencing
// it (Vehicle/User/Delivery/FeatureFlag all have a real FK to tenants.id)
// can be inserted.
type devTenantProvisioner interface {
	FindOrCreate(ctx context.Context, id uuid.UUID) error
}

type AuthHandler struct {
	oauthConfig *oauth2.Config
	authService *service.AuthService
	jwtIssuer   *authpkg.JWTIssuer
	tenants     devTenantProvisioner
}

func NewAuthHandler(oauthConfig *oauth2.Config, authService *service.AuthService, jwtIssuer *authpkg.JWTIssuer, tenants devTenantProvisioner) *AuthHandler {
	return &AuthHandler{oauthConfig: oauthConfig, authService: authService, jwtIssuer: jwtIssuer, tenants: tenants}
}

// RegisterRoutes adds the real OAuth2 routes. These must NOT sit behind
// the Authenticate middleware — they're how a client gets a token in the
// first place.
func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/auth/login", h.Login)
	rg.GET("/auth/callback", h.Callback)
}

// RegisterDevRoutes adds the dev-only token-minting endpoint. Call this
// only when APP_ENV=local (see cmd/api/main.go) — it lets Postman/CI get a
// valid session token without a real Google account, by issuing one
// directly. It must never be wired up outside local development.
func (h *AuthHandler) RegisterDevRoutes(rg *gin.RouterGroup) {
	rg.POST("/auth/dev-token", h.DevToken)
}

// Login redirects the browser to Google's sign-in page.
//
// Known simplification: a real implementation would generate a random
// per-request "state" value, stash it in a short-lived session, and
// verify it matches on callback (CSRF protection for the auth flow).
// There's no session store in this project yet to stash it in, so this is
// flagged here rather than silently skipped.
func (h *AuthHandler) Login(c *gin.Context) {
	url := h.oauthConfig.AuthCodeURL("state")
	c.Redirect(http.StatusFound, url)
}

func (h *AuthHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing code"})
		return
	}

	tok, err := h.oauthConfig.Exchange(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "token exchange with Google failed"})
		return
	}

	rawIDToken, err := authpkg.ExtractIDToken(tok)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	claims, err := authpkg.ParseIDToken(rawIDToken)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid id_token from Google"})
		return
	}

	sessionToken, err := h.authService.HandleGoogleLogin(c.Request.Context(), *claims)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": sessionToken})
}

type devTokenRequest struct {
	TenantID string          `json:"tenant_id"` // optional; a new tenant is provisioned if omitted
	Role     domain.UserRole `json:"role" binding:"required"`
}

// DevToken mints a session JWT directly, bypassing Google entirely.
// Role isn't validated against the known-roles list here the way the real
// User-creation path does — this is a test-only shortcut, not production
// surface, so that extra guard isn't worth the added code for what it
// protects.
func (h *AuthHandler) DevToken(c *gin.Context) {
	var req devTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := uuid.New()
	if req.TenantID != "" {
		parsed, err := uuid.Parse(req.TenantID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
			return
		}
		tenantID = parsed
	}

	if err := h.tenants.FindOrCreate(c.Request.Context(), tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	token, err := h.jwtIssuer.Issue(uuid.New(), tenantID, req.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token, "tenant_id": tenantID})
}
