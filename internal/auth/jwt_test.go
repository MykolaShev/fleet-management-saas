package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MykolaShev/fleet-management-saas/internal/auth"
	"github.com/MykolaShev/fleet-management-saas/internal/domain"
)

func TestJWTIssuer_IssueAndParse_Roundtrip(t *testing.T) {
	issuer := auth.NewJWTIssuer("test-secret", time.Hour)
	userID := uuid.New()
	tenantID := uuid.New()

	token, err := issuer.Issue(userID, tenantID, domain.RoleManager)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := issuer.Parse(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, tenantID, claims.TenantID)
	assert.Equal(t, domain.RoleManager, claims.Role)
}

func TestJWTIssuer_Parse_RejectsExpiredToken(t *testing.T) {
	issuer := auth.NewJWTIssuer("test-secret", -time.Hour) // issues already-expired tokens
	token, err := issuer.Issue(uuid.New(), uuid.New(), domain.RoleDriver)
	require.NoError(t, err)

	_, err = issuer.Parse(token)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestJWTIssuer_Parse_RejectsTokenSignedWithDifferentSecret(t *testing.T) {
	issuerA := auth.NewJWTIssuer("secret-a", time.Hour)
	issuerB := auth.NewJWTIssuer("secret-b", time.Hour)

	token, err := issuerA.Issue(uuid.New(), uuid.New(), domain.RoleManager)
	require.NoError(t, err)

	_, err = issuerB.Parse(token)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestJWTIssuer_Parse_RejectsGarbage(t *testing.T) {
	issuer := auth.NewJWTIssuer("test-secret", time.Hour)

	_, err := issuer.Parse("not-a-real-token")
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}
