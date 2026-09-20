package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
)

// ErrValidation wraps input validation failures, so handlers can map them
// to 422 without the service needing to know about HTTP.
var ErrValidation = errors.New("validation error")

// vehicleRepository is the subset of *repository.VehicleRepository this
// service needs.
type vehicleRepository interface {
	Create(ctx context.Context, v *domain.Vehicle) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Vehicle, error)
	List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]domain.Vehicle, int64, error)
	Update(ctx context.Context, tenantID uuid.UUID, v *domain.Vehicle) error
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

type VehicleService struct {
	repo vehicleRepository
}

func NewVehicleService(repo vehicleRepository) *VehicleService {
	return &VehicleService{repo: repo}
}

type CreateVehicleInput struct {
	TenantID    uuid.UUID
	PlateNumber string
	Model       string
}

func (s *VehicleService) Create(ctx context.Context, in CreateVehicleInput) (*domain.Vehicle, error) {
	if in.PlateNumber == "" {
		return nil, fmt.Errorf("%w: plate_number is required", ErrValidation)
	}

	v := &domain.Vehicle{
		TenantID:    in.TenantID,
		PlateNumber: in.PlateNumber,
		Model:       in.Model,
		Status:      domain.VehicleIdle,
	}

	if err := s.repo.Create(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *VehicleService) Get(ctx context.Context, tenantID, id uuid.UUID) (*domain.Vehicle, error) {
	return s.repo.GetByID(ctx, tenantID, id)
}

// ListResult is a page of vehicles plus the metadata needed to render
// pagination controls (B1).
type ListResult struct {
	Items      []domain.Vehicle `json:"items"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalItems int64            `json:"total_items"`
	TotalPages int              `json:"total_pages"`
}

func (s *VehicleService) List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) (*ListResult, error) {
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

	return &ListResult{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		TotalItems: total,
		TotalPages: totalPages,
	}, nil
}

// UpdateVehicleInput uses pointers so "not provided" (nil) is distinguishable
// from "explicitly set to zero value" — a plain PATCH semantics.
type UpdateVehicleInput struct {
	PlateNumber *string
	Model       *string
	Status      *domain.VehicleStatus
}

func (s *VehicleService) Update(ctx context.Context, tenantID, id uuid.UUID, in UpdateVehicleInput) (*domain.Vehicle, error) {
	v, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	if in.PlateNumber != nil {
		if *in.PlateNumber == "" {
			return nil, fmt.Errorf("%w: plate_number cannot be empty", ErrValidation)
		}
		v.PlateNumber = *in.PlateNumber
	}
	if in.Model != nil {
		v.Model = *in.Model
	}
	if in.Status != nil {
		v.Status = *in.Status
	}

	if err := s.repo.Update(ctx, tenantID, v); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *VehicleService) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.repo.Delete(ctx, tenantID, id)
}
