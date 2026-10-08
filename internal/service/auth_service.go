package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/MykolaShev/fleet-management-saas/internal/auth"
	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/repository"
)

// tenantProvisioner and authUserRepository are narrow interfaces so
// AuthService can be unit-tested with fakes — the same pattern every
// other service in this codebase follows.
type tenantProvisioner interface {
	FindOrCreate(ctx context.Context, id uuid.UUID) error
}

type authUserRepository interface {
	// FindByOAuthSubject looks up a user by their external OAuth identity
	// alone, with no tenant scoping — unlike Microsoft's token, Google's
	// doesn't carry an organization claim to scope the search by, and
	// each OAuth subject belongs to exactly one tenant anyway (the one
	// created for them on first login), so a global lookup is both
	// necessary and sufficient.
	FindByOAuthSubject(ctx context.Context, subject string) (*domain.User, error)
	Create(ctx context.Context, u *domain.User) error
}

type AuthService struct {
	tenants tenantProvisioner
	users   authUserRepository
	jwt     *auth.JWTIssuer
}

func NewAuthService(tenants tenantProvisioner, users authUserRepository, jwt *auth.JWTIssuer) *AuthService {
	return &AuthService{tenants: tenants, users: users, jwt: jwt}
}

// HandleGoogleLogin maps a validated Google identity onto our own
// Tenant/User records and issues our session JWT.
//
// Mapping rule: a Google account carries no organization/tenant claim —
// it's just a person — so there's nothing external to reuse as our
// Tenant.ID the way Microsoft's "tid" was. Instead, a user signing in for
// the first time (no row with this OAuthSubject anywhere yet) gets a
// brand-new Tenant created just for them, and becomes its "manager". This
// matches ordinary self-service SaaS signup: "sign up and your company
// workspace is created for you". A returning user (OAuthSubject already on
// file) logs back into the same tenant/user they already have.
//
// There's no self-service driver signup via OAuth — drivers are added by
// a manager afterwards, through the existing User CRUD endpoints.
func (s *AuthService) HandleGoogleLogin(ctx context.Context, claims auth.GoogleClaims) (string, error) {
	user, err := s.users.FindByOAuthSubject(ctx, claims.Subject)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		tenantID := uuid.New()
		if err := s.tenants.FindOrCreate(ctx, tenantID); err != nil {
			return "", fmt.Errorf("provision tenant: %w", err)
		}

		subject := claims.Subject
		user = &domain.User{
			TenantID:     tenantID,
			Email:        claims.Email,
			Name:         claims.Name,
			Role:         domain.RoleManager,
			OAuthSubject: &subject,
		}
		if err := s.users.Create(ctx, user); err != nil {
			return "", fmt.Errorf("create user: %w", err)
		}
	case err != nil:
		return "", fmt.Errorf("look up user: %w", err)
	}

	return s.jwt.Issue(user.ID, user.TenantID, user.Role)
}
