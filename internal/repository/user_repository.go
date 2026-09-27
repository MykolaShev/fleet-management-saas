package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
)

// ErrConflict is returned when a create would violate a uniqueness
// constraint (e.g. an email already used within the same tenant).
var ErrConflict = errors.New("already exists")

// UserRepository handles persistence for User, scoped to a tenant.
type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create checks for an existing tenant+email pair before inserting, rather
// than relying on parsing the DB driver's unique-violation error (which
// differs between Postgres and SQLite and is easy to get subtly wrong).
// This isn't perfectly race-free under concurrent creates, but for this
// project's scale that's an acceptable, easy-to-reason-about trade-off.
func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.User{}).
		Where("tenant_id = ? AND email = ?", u.TenantID, u.Email).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrConflict
	}

	return r.db.WithContext(ctx).Create(u).Error
}

func (r *UserRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.User, error) {
	var u domain.User
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]domain.User, int64, error) {
	page, pageSize = normalizePage(page, pageSize)

	var users []domain.User
	var total int64

	q := r.db.WithContext(ctx).Model(&domain.User{}).Where("tenant_id = ?", tenantID)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := q.
		Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&users).Error
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// Update explicitly selects the mutable columns for the same reason as
// VehicleRepository.Update: GORM's struct-based Updates silently skips
// zero-value fields, which would make it impossible to persist certain
// legitimate changes.
func (r *UserRepository) Update(ctx context.Context, tenantID uuid.UUID, u *domain.User) error {
	res := r.db.WithContext(ctx).
		Model(&domain.User{}).
		Where("tenant_id = ? AND id = ?", tenantID, u.ID).
		Select("email", "name", "role").
		Updates(u)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&domain.User{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
