package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MykolaShev/fleet-management-saas/internal/repository"
)

func TestTenantRepository_FindOrCreate_CreatesOnFirstCall(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTenantRepository(db)
	ctx := context.Background()

	id := uuid.New()
	require.NoError(t, repo.FindOrCreate(ctx, id))

	var count int64
	require.NoError(t, db.Table("tenants").Where("id = ?", id).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestTenantRepository_FindOrCreate_IsIdempotent(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTenantRepository(db)
	ctx := context.Background()

	id := uuid.New()
	require.NoError(t, repo.FindOrCreate(ctx, id))
	require.NoError(t, repo.FindOrCreate(ctx, id)) // second call must not error or duplicate

	var count int64
	require.NoError(t, db.Table("tenants").Where("id = ?", id).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
