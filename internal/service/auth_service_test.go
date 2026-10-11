package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MykolaShev/fleet-management-saas/internal/auth"
	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/repository"
	"github.com/MykolaShev/fleet-management-saas/internal/service"
)

type fakeTenantProvisioner struct {
	created []uuid.UUID
}

func (f *fakeTenantProvisioner) FindOrCreate(_ context.Context, id uuid.UUID) error {
	f.created = append(f.created, id)
	return nil
}

// fakeAuthUserRepo is keyed by OAuth subject alone (no tenant), matching
// the global-lookup interface Google's flow needs.
type fakeAuthUserRepo struct {
	users map[string]domain.User // key: subject
}

func newFakeAuthUserRepo() *fakeAuthUserRepo {
	return &fakeAuthUserRepo{users: map[string]domain.User{}}
}

func (f *fakeAuthUserRepo) FindByOAuthSubject(_ context.Context, subject string) (*domain.User, error) {
	u, ok := f.users[subject]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &u, nil
}

func (f *fakeAuthUserRepo) Create(_ context.Context, u *domain.User) error {
	u.ID = uuid.New()
	f.users[*u.OAuthSubject] = *u
	return nil
}

func TestAuthService_HandleGoogleLogin_FirstTimeUser_CreatesOwnTenantAsManager(t *testing.T) {
	tenants := &fakeTenantProvisioner{}
	users := newFakeAuthUserRepo()
	jwtIssuer := auth.NewJWTIssuer("test-secret", time.Hour)
	svc := service.NewAuthService(tenants, users, jwtIssuer)

	claims := auth.GoogleClaims{
		Subject: "google-sub-123",
		Email:   "ada@gmail.com",
		Name:    "Ada",
	}

	token, err := svc.HandleGoogleLogin(context.Background(), claims)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	parsed, err := jwtIssuer.Parse(token)
	require.NoError(t, err)
	assert.Equal(t, domain.RoleManager, parsed.Role)

	require.Len(t, tenants.created, 1, "exactly one new tenant should be provisioned for a first-time user")
	require.Len(t, users.users, 1)
	assert.Equal(t, tenants.created[0], parsed.TenantID)
}

func TestAuthService_HandleGoogleLogin_ReturningUser_ReusesExistingTenantAndUser(t *testing.T) {
	tenants := &fakeTenantProvisioner{}
	users := newFakeAuthUserRepo()
	jwtIssuer := auth.NewJWTIssuer("test-secret", time.Hour)
	svc := service.NewAuthService(tenants, users, jwtIssuer)

	claims := auth.GoogleClaims{
		Subject: "google-sub-123",
		Email:   "ada@gmail.com",
		Name:    "Ada",
	}

	firstToken, err := svc.HandleGoogleLogin(context.Background(), claims)
	require.NoError(t, err)
	firstClaims, err := jwtIssuer.Parse(firstToken)
	require.NoError(t, err)

	// Logging in again with the same Google sub must reuse the same
	// tenant and user, not create a second one.
	secondToken, err := svc.HandleGoogleLogin(context.Background(), claims)
	require.NoError(t, err)
	secondClaims, err := jwtIssuer.Parse(secondToken)
	require.NoError(t, err)

	assert.Equal(t, firstClaims.TenantID, secondClaims.TenantID)
	assert.Equal(t, firstClaims.UserID, secondClaims.UserID)
	assert.Len(t, tenants.created, 1, "second login must not provision another tenant")
	assert.Len(t, users.users, 1, "second login must not create a second user")
}

func TestAuthService_HandleGoogleLogin_DifferentUsers_GetDifferentTenants(t *testing.T) {
	tenants := &fakeTenantProvisioner{}
	users := newFakeAuthUserRepo()
	jwtIssuer := auth.NewJWTIssuer("test-secret", time.Hour)
	svc := service.NewAuthService(tenants, users, jwtIssuer)

	tokenA, err := svc.HandleGoogleLogin(context.Background(), auth.GoogleClaims{Subject: "sub-a", Email: "a@gmail.com", Name: "A"})
	require.NoError(t, err)
	tokenB, err := svc.HandleGoogleLogin(context.Background(), auth.GoogleClaims{Subject: "sub-b", Email: "b@gmail.com", Name: "B"})
	require.NoError(t, err)

	claimsA, _ := jwtIssuer.Parse(tokenA)
	claimsB, _ := jwtIssuer.Parse(tokenB)

	assert.NotEqual(t, claimsA.TenantID, claimsB.TenantID)
	assert.Len(t, tenants.created, 2)
}
