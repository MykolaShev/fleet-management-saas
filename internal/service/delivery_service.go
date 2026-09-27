package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/repository"
)

type deliveryRepository interface {
	Create(ctx context.Context, d *domain.Delivery) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Delivery, error)
	List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]domain.Delivery, int64, error)
	Update(ctx context.Context, tenantID uuid.UUID, d *domain.Delivery) error
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

// vehicleLookup and userLookup are narrow read-only views used only to
// validate that a delivery is assigned to a vehicle/driver from the same
// tenant. *repository.VehicleRepository and *repository.UserRepository
// already satisfy these — no adapter needed — and tests can supply small
// fakes instead of standing up a real database.
type vehicleLookup interface {
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Vehicle, error)
}

type userLookup interface {
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.User, error)
}

type DeliveryService struct {
	repo     deliveryRepository
	vehicles vehicleLookup
	users    userLookup
}

func NewDeliveryService(repo deliveryRepository, vehicles vehicleLookup, users userLookup) *DeliveryService {
	return &DeliveryService{repo: repo, vehicles: vehicles, users: users}
}

func isValidDeliveryStatus(s domain.DeliveryStatus) bool {
	switch s {
	case domain.DeliveryPending, domain.DeliveryAssigned, domain.DeliveryInTransit, domain.DeliveryDelivered, domain.DeliveryFailed:
		return true
	default:
		return false
	}
}

// validateVehicle confirms the vehicle exists and belongs to tenantID —
// this is what stops one tenant from assigning a delivery to another
// tenant's vehicle just by guessing/reusing its ID.
func (s *DeliveryService) validateVehicle(ctx context.Context, tenantID, vehicleID uuid.UUID) error {
	if _, err := s.vehicles.GetByID(ctx, tenantID, vehicleID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("%w: vehicle_id does not belong to this tenant", ErrValidation)
		}
		return err
	}
	return nil
}

// validateDriver confirms the user exists, belongs to tenantID, and has
// the driver role (a manager can't be assigned as a delivery's driver).
func (s *DeliveryService) validateDriver(ctx context.Context, tenantID, driverID uuid.UUID) error {
	u, err := s.users.GetByID(ctx, tenantID, driverID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("%w: driver_id does not belong to this tenant", ErrValidation)
		}
		return err
	}
	if u.Role != domain.RoleDriver {
		return fmt.Errorf("%w: driver_id must reference a user with role %q", ErrValidation, domain.RoleDriver)
	}
	return nil
}

type CreateDeliveryInput struct {
	TenantID       uuid.UUID
	VehicleID      *uuid.UUID
	DriverID       *uuid.UUID
	PickupAddress  string
	DropoffAddress string
}

func (s *DeliveryService) Create(ctx context.Context, in CreateDeliveryInput) (*domain.Delivery, error) {
	if in.PickupAddress == "" {
		return nil, fmt.Errorf("%w: pickup_address is required", ErrValidation)
	}
	if in.DropoffAddress == "" {
		return nil, fmt.Errorf("%w: dropoff_address is required", ErrValidation)
	}

	if in.VehicleID != nil {
		if err := s.validateVehicle(ctx, in.TenantID, *in.VehicleID); err != nil {
			return nil, err
		}
	}
	if in.DriverID != nil {
		if err := s.validateDriver(ctx, in.TenantID, *in.DriverID); err != nil {
			return nil, err
		}
	}

	// Simple state rule: a delivery created with a vehicle and/or driver
	// already attached starts "assigned" instead of "pending".
	status := domain.DeliveryPending
	if in.VehicleID != nil || in.DriverID != nil {
		status = domain.DeliveryAssigned
	}

	d := &domain.Delivery{
		TenantID:       in.TenantID,
		VehicleID:      in.VehicleID,
		DriverID:       in.DriverID,
		Status:         status,
		PickupAddress:  in.PickupAddress,
		DropoffAddress: in.DropoffAddress,
	}

	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *DeliveryService) Get(ctx context.Context, tenantID, id uuid.UUID) (*domain.Delivery, error) {
	return s.repo.GetByID(ctx, tenantID, id)
}

type DeliveryListResult struct {
	Items      []domain.Delivery `json:"items"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalItems int64             `json:"total_items"`
	TotalPages int               `json:"total_pages"`
}

func (s *DeliveryService) List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) (*DeliveryListResult, error) {
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

	return &DeliveryListResult{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		TotalItems: total,
		TotalPages: totalPages,
	}, nil
}

// UpdateDeliveryInput uses an explicit Unassign flag per relation instead
// of a bare *uuid.UUID: with only a pointer there's no way to tell "leave
// this relation as-is" (nil) apart from "clear it" (also nil). Two fields
// per relation is more verbose but leaves nothing ambiguous.
type UpdateDeliveryInput struct {
	Status          *domain.DeliveryStatus
	VehicleID       *uuid.UUID
	UnassignVehicle bool
	DriverID        *uuid.UUID
	UnassignDriver  bool
	PickupAddress   *string
	DropoffAddress  *string
}

func (s *DeliveryService) Update(ctx context.Context, tenantID, id uuid.UUID, in UpdateDeliveryInput) (*domain.Delivery, error) {
	d, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	switch {
	case in.UnassignVehicle:
		d.VehicleID = nil
	case in.VehicleID != nil:
		if err := s.validateVehicle(ctx, tenantID, *in.VehicleID); err != nil {
			return nil, err
		}
		d.VehicleID = in.VehicleID
	}

	switch {
	case in.UnassignDriver:
		d.DriverID = nil
	case in.DriverID != nil:
		if err := s.validateDriver(ctx, tenantID, *in.DriverID); err != nil {
			return nil, err
		}
		d.DriverID = in.DriverID
	}

	if in.Status != nil {
		if !isValidDeliveryStatus(*in.Status) {
			return nil, fmt.Errorf("%w: invalid status %q", ErrValidation, *in.Status)
		}
		d.Status = *in.Status
	}
	if in.PickupAddress != nil {
		if *in.PickupAddress == "" {
			return nil, fmt.Errorf("%w: pickup_address cannot be empty", ErrValidation)
		}
		d.PickupAddress = *in.PickupAddress
	}
	if in.DropoffAddress != nil {
		if *in.DropoffAddress == "" {
			return nil, fmt.Errorf("%w: dropoff_address cannot be empty", ErrValidation)
		}
		d.DropoffAddress = *in.DropoffAddress
	}

	if err := s.repo.Update(ctx, tenantID, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *DeliveryService) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.repo.Delete(ctx, tenantID, id)
}
