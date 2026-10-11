package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
)

// FeatureFlagRepository has no Create/Delete, unlike Vehicle/User/Delivery.
// A feature flag isn't a resource a tenant creates and removes — it's a
// toggle on a fixed catalog the system defines (see domain.KnownFeatureFlags).
// So the only operations that make sense are: read the current state
// (Get/List) and set a value (Set, which upserts).
type FeatureFlagRepository struct {
	db *gorm.DB
}

func NewFeatureFlagRepository(db *gorm.DB) *FeatureFlagRepository {
	return &FeatureFlagRepository{db: db}
}

// Get returns the row only if the tenant has explicitly configured this
// flag. ErrNotFound here means "never configured", which callers (see
// FeatureFlagService.IsEnabled) should treat as disabled, not as an error.
func (r *FeatureFlagRepository) Get(ctx context.Context, tenantID uuid.UUID, key domain.FeatureFlagKey) (*domain.FeatureFlag, error) {
	var f domain.FeatureFlag
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND key = ?", tenantID, key).
		First(&f).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// List returns only the flags this tenant has explicitly touched — merging
// in the rest of the catalog as "disabled by default" is the service
// layer's job (it owns the catalog, the repository just stores rows).
func (r *FeatureFlagRepository) List(ctx context.Context, tenantID uuid.UUID) ([]domain.FeatureFlag, error) {
	var flags []domain.FeatureFlag
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("key").
		Find(&flags).Error
	return flags, err
}

// Set upserts the flag's value. FirstOrCreate alone isn't enough here: it
// only inserts when the row is missing, it never updates an existing one —
// so an explicit follow-up Update is required to actually change an
// already-configured flag's value. The Select("enabled") on that update is
// the same pattern as elsewhere in this codebase: Enabled=false is a
// perfectly normal value to persist, not an empty/"unset" one, so it must
// never be silently skipped by GORM's zero-value handling.
func (r *FeatureFlagRepository) Set(ctx context.Context, tenantID uuid.UUID, key domain.FeatureFlagKey, enabled bool) (*domain.FeatureFlag, error) {
	var f domain.FeatureFlag
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND key = ?", tenantID, key).
		Attrs(domain.FeatureFlag{TenantID: tenantID, Key: key}).
		FirstOrCreate(&f).Error
	if err != nil {
		return nil, err
	}

	err = r.db.WithContext(ctx).
		Model(&domain.FeatureFlag{}).
		Where("tenant_id = ? AND key = ?", tenantID, key).
		Select("enabled").
		Updates(map[string]any{"enabled": enabled}).Error
	if err != nil {
		return nil, err
	}

	f.Enabled = enabled
	return &f, nil
}
