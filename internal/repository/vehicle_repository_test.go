package repository_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/repository"
)

// setupTestDB spins up a fresh in-memory SQLite DB per test, migrated with
// the same GORM models used against Postgres in production. This exercises
// real repository/query logic without needing Docker in CI. It is not a
// substitute for testing against real Postgres (see docs/TESTING.md) —
// that's covered by the docker-compose health-check smoke test.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&domain.Tenant{}, &domain.Vehicle{}))

	return db
}

func TestVehicleRepository_CreateAndGet(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewVehicleRepository(db)
	ctx := context.Background()

	tenantID := uuid.New()
	v := &domain.Vehicle{TenantID: tenantID, PlateNumber: "AA1234BC", Model: "Ford Transit"}

	require.NoError(t, repo.Create(ctx, v))
	assert.NotEqual(t, uuid.Nil, v.ID)

	got, err := repo.GetByID(ctx, tenantID, v.ID)
	require.NoError(t, err)
	assert.Equal(t, "AA1234BC", got.PlateNumber)
	assert.Equal(t, domain.VehicleStatus(""), got.Status) // status set by service, not repo
}

func TestVehicleRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewVehicleRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New(), uuid.New())
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestVehicleRepository_List_IsScopedToTenant(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewVehicleRepository(db)
	ctx := context.Background()

	tenantA := uuid.New()
	tenantB := uuid.New()

	require.NoError(t, repo.Create(ctx, &domain.Vehicle{TenantID: tenantA, PlateNumber: "A1"}))
	require.NoError(t, repo.Create(ctx, &domain.Vehicle{TenantID: tenantA, PlateNumber: "A2"}))
	require.NoError(t, repo.Create(ctx, &domain.Vehicle{TenantID: tenantB, PlateNumber: "B1"}))

	items, total, err := repo.List(ctx, tenantA, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, items, 2)
}

func TestVehicleRepository_List_Pagination(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewVehicleRepository(db)
	ctx := context.Background()

	tenantID := uuid.New()
	for i := 0; i < 5; i++ {
		require.NoError(t, repo.Create(ctx, &domain.Vehicle{TenantID: tenantID, PlateNumber: "V"}))
	}

	page1, total, err := repo.List(ctx, tenantID, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, page1, 2)

	page3, _, err := repo.List(ctx, tenantID, 3, 2)
	require.NoError(t, err)
	assert.Len(t, page3, 1) // 5 items, page size 2 -> last page has the remainder
}

func TestVehicleRepository_Update_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewVehicleRepository(db)
	ctx := context.Background()

	err := repo.Update(ctx, uuid.New(), &domain.Vehicle{BaseModel: domain.BaseModel{ID: uuid.New()}})
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestVehicleRepository_Update_PersistsZeroValueModel(t *testing.T) {
	// Regression test for the GORM struct-Updates footgun documented in
	// vehicle_repository.go: clearing Model to "" must actually persist,
	// not be silently skipped because it's a Go zero value.
	db := setupTestDB(t)
	repo := repository.NewVehicleRepository(db)
	ctx := context.Background()

	tenantID := uuid.New()
	v := &domain.Vehicle{TenantID: tenantID, PlateNumber: "P1", Model: "Old Model"}
	require.NoError(t, repo.Create(ctx, v))

	v.Model = ""
	require.NoError(t, repo.Update(ctx, tenantID, v))

	got, err := repo.GetByID(ctx, tenantID, v.ID)
	require.NoError(t, err)
	assert.Equal(t, "", got.Model)
}

func TestVehicleRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewVehicleRepository(db)
	ctx := context.Background()

	tenantID := uuid.New()
	v := &domain.Vehicle{TenantID: tenantID, PlateNumber: "DEL1"}
	require.NoError(t, repo.Create(ctx, v))

	require.NoError(t, repo.Delete(ctx, tenantID, v.ID))

	_, err := repo.GetByID(ctx, tenantID, v.ID)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestVehicleRepository_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewVehicleRepository(db)
	ctx := context.Background()

	err := repo.Delete(ctx, uuid.New(), uuid.New())
	assert.ErrorIs(t, err, repository.ErrNotFound)
}
