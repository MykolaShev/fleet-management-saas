package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
)

// userRepository is the subset of *repository.UserRepository this service
// needs, mirroring the vehicleRepository pattern so tests can use a fake.
type userRepository interface {
	Create(ctx context.Context, u *domain.User) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.User, error)
	List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]domain.User, int64, error)
	Update(ctx context.Context, tenantID uuid.UUID, u *domain.User) error
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

type UserService struct {
	repo userRepository
}

func NewUserService(repo userRepository) *UserService {
	return &UserService{repo: repo}
}

func isValidRole(r domain.UserRole) bool {
	return r == domain.RoleManager || r == domain.RoleDriver
}

type CreateUserInput struct {
	TenantID uuid.UUID
	Email    string
	Name     string
	Role     domain.UserRole
}

func (s *UserService) Create(ctx context.Context, in CreateUserInput) (*domain.User, error) {
	if in.Email == "" {
		return nil, fmt.Errorf("%w: email is required", ErrValidation)
	}
	if !isValidRole(in.Role) {
		return nil, fmt.Errorf("%w: role must be %q or %q", ErrValidation, domain.RoleManager, domain.RoleDriver)
	}

	u := &domain.User{
		TenantID: in.TenantID,
		Email:    in.Email,
		Name:     in.Name,
		Role:     in.Role,
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *UserService) Get(ctx context.Context, tenantID, id uuid.UUID) (*domain.User, error) {
	return s.repo.GetByID(ctx, tenantID, id)
}

type UserListResult struct {
	Items      []domain.User `json:"items"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	TotalItems int64         `json:"total_items"`
	TotalPages int           `json:"total_pages"`
}

func (s *UserService) List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) (*UserListResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	items, total, err := s.repo.List(ctx, tenantID, page, pageSize)
	if err != nil {
		return nil, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &UserListResult{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		TotalItems: total,
		TotalPages: totalPages,
	}, nil
}

type UpdateUserInput struct {
	Name *string
	Role *domain.UserRole
}

func (s *UserService) Update(ctx context.Context, tenantID, id uuid.UUID, in UpdateUserInput) (*domain.User, error) {
	u, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	if in.Name != nil {
		u.Name = *in.Name
	}
	if in.Role != nil {
		if !isValidRole(*in.Role) {
			return nil, fmt.Errorf("%w: role must be %q or %q", ErrValidation, domain.RoleManager, domain.RoleDriver)
		}
		u.Role = *in.Role
	}

	if err := s.repo.Update(ctx, tenantID, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *UserService) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.repo.Delete(ctx, tenantID, id)
}
