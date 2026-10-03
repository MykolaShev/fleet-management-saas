package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/repository"
)

func setupFeatureFlagTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupTestDB(t) // Tenant + Vehicle, from vehicle_repository_test.go
	require.NoError(t, db.AutoMigrate(&domain.FeatureFlag{}))
	return db
}

func TestFeatureFlagRepository_Get_NotFoundWhenUnconfigured(t *testing.T) {
	db := setupFeatureFlagTestDB(t)
	repo := repository.NewFeatureFlagRepository(db)
	ctx := context.Background()

	tenantID := uuid.New()
	createTestTenant(t, db, tenantID)

	_, err := repo.Get(ctx, tenantID, domain.FeatureAIAgent)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestFeatureFlagRepository_Set_CreatesOnFirstCall(t *testing.T) {
	db := setupFeatureFlagTestDB(t)
	repo := repository.NewFeatureFlagRepository(db)
	ctx := context.Background()

	tenantID := uuid.New()
	createTestTenant(t, db, tenantID)

	f, err := repo.Set(ctx, tenantID, domain.FeatureAIAgent, true)
	require.NoError(t, err)
	assert.True(t, f.Enabled)

	got, err := repo.Get(ctx, tenantID, domain.FeatureAIAgent)
	require.NoError(t, err)
	assert.True(t, got.Enabled)
}

func TestFeatureFlagRepository_Set_UpdatesExistingRowRatherThanDuplicating(t *testing.T) {
	db := setupFeatureFlagTestDB(t)
	repo := repository.NewFeatureFlagRepository(db)
	ctx := context.Background()

	tenantID := uuid.New()
	createTestTenant(t, db, tenantID)

	_, err := repo.Set(ctx, tenantID, domain.FeatureAIAgent, true)
	require.NoError(t, err)

	// Flip it back to false — this must UPDATE the existing row, not skip
	// the write because false is a Go zero value, and must not create a
	// second row for the same (tenant_id, key) pair.
	f, err := repo.Set(ctx, tenantID, domain.FeatureAIAgent, false)
	require.NoError(t, err)
	assert.False(t, f.Enabled)

	flags, err := repo.List(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, flags, 1)
	assert.False(t, flags[0].Enabled)
}

func TestFeatureFlagRepository_List_OnlyReturnsConfiguredFlags(t *testing.T) {
	db := setupFeatureFlagTestDB(t)
	repo := repository.NewFeatureFlagRepository(db)
	ctx := context.Background()

	tenantID := uuid.New()
	createTestTenant(t, db, tenantID)

	_, err := repo.Set(ctx, tenantID, domain.FeatureLiveTracking, true)
	require.NoError(t, err)

	flags, err := repo.List(ctx, tenantID)
	require.NoError(t, err)
	// Only the one flag explicitly set, not the whole catalog — merging in
	// the rest as "disabled by default" is FeatureFlagService's job.
	require.Len(t, flags, 1)
	assert.Equal(t, domain.FeatureLiveTracking, flags[0].Key)
}

func TestFeatureFlagRepository_List_IsScopedToTenant(t *testing.T) {
	db := setupFeatureFlagTestDB(t)
	repo := repository.NewFeatureFlagRepository(db)
	ctx := context.Background()

	tenantA := uuid.New()
	tenantB := uuid.New()
	createTestTenant(t, db, tenantA)
	createTestTenant(t, db, tenantB)

	_, err := repo.Set(ctx, tenantA, domain.FeatureAIAgent, true)
	require.NoError(t, err)
	_, err = repo.Set(ctx, tenantB, domain.FeatureAIAgent, true)
	require.NoError(t, err)

	flagsA, err := repo.List(ctx, tenantA)
	require.NoError(t, err)
	assert.Len(t, flagsA, 1)
}
