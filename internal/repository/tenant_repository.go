package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
)

// TenantRepository manages Tenant rows. For now it only needs to guarantee
// a tenant exists (see FindOrCreate) — full tenant management
// (registration, settings, billing, ...) belongs to a later stage, once
// real auth (C16) decides how tenants actually get created (e.g. "first
// login provisions the company").
type TenantRepository struct {
	db *gorm.DB
}

func NewTenantRepository(db *gorm.DB) *TenantRepository {
	return &TenantRepository{db: db}
}

// FindOrCreate ensures a Tenant row with this exact ID exists, inserting a
// minimal placeholder one if not. Idempotent: calling it repeatedly with
// the same id after the first call is a no-op.
func (r *TenantRepository) FindOrCreate(ctx context.Context, id uuid.UUID) error {
	tenant := domain.Tenant{
		BaseModel: domain.BaseModel{ID: id},
		Name:      "Tenant " + id.String()[:8],
	}
	return r.db.WithContext(ctx).
		Where("id = ?", id).
		FirstOrCreate(&tenant).Error
}
