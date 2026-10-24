package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
)

// ErrNotFound is returned when a lookup, update, or delete targets a row
// that doesn't exist (or doesn't belong to the given tenant).
var ErrNotFound = errors.New("record not found")

// VehicleRepository handles persistence for Vehicle. Every method takes a
// tenantID and scopes the query to it — this is the enforcement point for
// tenant isolation (see the B5 stage, which will add dedicated tests for
// cross-tenant access attempts).
type VehicleRepository struct {
	db *gorm.DB
}

func NewVehicleRepository(db *gorm.DB) *VehicleRepository {
	return &VehicleRepository{db: db}
}

func (r *VehicleRepository) Create(ctx context.Context, v *domain.Vehicle) error {
	return r.db.WithContext(ctx).Create(v).Error
}

func (r *VehicleRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Vehicle, error) {
	var v domain.Vehicle
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// List returns a page of vehicles for the tenant, plus the total count
// (needed to compute total pages for the pagination response).
func (r *VehicleRepository) List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]domain.Vehicle, int64, error) {
	page, pageSize = normalizePage(page, pageSize)

	var vehicles []domain.Vehicle
	var total int64

	q := r.db.WithContext(ctx).Model(&domain.Vehicle{}).Where("tenant_id = ?", tenantID)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := q.
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&vehicles).Error
	if err != nil {
		return nil, 0, err
	}

	return vehicles, total, nil
}

// Update writes back only the mutable fields (plate number, model, status).
// It explicitly selects those columns rather than calling plain
// .Updates(v): GORM's struct-based Updates silently skips zero-value
// fields (e.g. an intentionally empty Model would never be persisted),
// which is a common source of "why didn't my update take effect" bugs.
func (r *VehicleRepository) Update(ctx context.Context, tenantID uuid.UUID, v *domain.Vehicle) error {
	res := r.db.WithContext(ctx).
		Model(&domain.Vehicle{}).
		Where("tenant_id = ? AND id = ?", tenantID, v.ID).
		Select("plate_number", "model", "status").
		Updates(v)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateLocation writes only lat/lng/timestamp — separate from Update
// because that method's Select list deliberately excludes these columns
// (they weren't mutable via the regular API until C10). Used by the C10
// position simulator.
func (r *VehicleRepository) UpdateLocation(ctx context.Context, tenantID, id uuid.UUID, lat, lng float64, at time.Time) error {
	res := r.db.WithContext(ctx).
		Model(&domain.Vehicle{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Select("current_lat", "current_lng", "last_location_at").
		Updates(&domain.Vehicle{CurrentLat: &lat, CurrentLng: &lng, LastLocationAt: &at})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ListAllActive returns every non-idle vehicle across ALL tenants — the
// one deliberate exception to this file's tenant-scoping rule. It exists
// only for the C10 simulator, a background process with no request
// context and nothing user-facing to leak into; it must never be called
// from a handler.
func (r *VehicleRepository) ListAllActive(ctx context.Context) ([]domain.Vehicle, error) {
	var vehicles []domain.Vehicle
	err := r.db.WithContext(ctx).
		Where("status != ?", domain.VehicleIdle).
		Find(&vehicles).Error
	return vehicles, err
}

func (r *VehicleRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&domain.Vehicle{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}