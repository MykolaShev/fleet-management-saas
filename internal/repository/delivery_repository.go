package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
)

// DeliveryRepository handles persistence for Delivery, scoped to a tenant.
// Cross-tenant referential validation (does VehicleID/DriverID actually
// belong to this tenant?) is NOT this repository's job — that's business
// logic and lives in DeliveryService, which can be unit-tested without a
// database.
type DeliveryRepository struct {
	db *gorm.DB
}

func NewDeliveryRepository(db *gorm.DB) *DeliveryRepository {
	return &DeliveryRepository{db: db}
}

func (r *DeliveryRepository) Create(ctx context.Context, d *domain.Delivery) error {
	return r.db.WithContext(ctx).Create(d).Error
}

func (r *DeliveryRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Delivery, error) {
	var d domain.Delivery
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DeliveryRepository) List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]domain.Delivery, int64, error) {
	page, pageSize = normalizePage(page, pageSize)

	var deliveries []domain.Delivery
	var total int64

	q := r.db.WithContext(ctx).Model(&domain.Delivery{}).Where("tenant_id = ?", tenantID)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := q.
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&deliveries).Error
	if err != nil {
		return nil, 0, err
	}

	return deliveries, total, nil
}

// Update explicitly selects the mutable columns, same rationale as
// VehicleRepository/UserRepository.Update — with the extra wrinkle that
// VehicleID/DriverID are *pointers*: selecting them forces GORM to persist
// a nil pointer as SQL NULL (i.e. "unassign"), instead of silently
// skipping the column the way plain Updates() would.
func (r *DeliveryRepository) Update(ctx context.Context, tenantID uuid.UUID, d *domain.Delivery) error {
	res := r.db.WithContext(ctx).
		Model(&domain.Delivery{}).
		Where("tenant_id = ? AND id = ?", tenantID, d.ID).
		Select("vehicle_id", "driver_id", "status", "pickup_address", "dropoff_address").
		Updates(d)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *DeliveryRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&domain.Delivery{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
