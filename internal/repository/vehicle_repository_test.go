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
//
// SQLite does NOT enforce foreign keys by default (unlike Postgres), so we
// turn it on explicitly here. Without this, a test could insert a Vehicle
// referencing a Tenant that was never created and still pass locally,
// while the exact same insert fails against real Postgres with a foreign
// key violation — which is exactly what happened before this fix.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)

	require.NoError(t, db.AutoMigrate(&domain.Tenant{}, &domain.Vehicle{}))

	return db
}

// createTestTenant inserts a minimal Tenant row so FK-constrained inserts
// (Vehicle/User/Delivery, all of which reference tenants.id) succeed. Every
// test that creates one of those now needs a tenant to exist first — this
// mirrors what TenantRepository.FindOrCreate does for real requests.
func createTestTenant(t *testing.T, db *gorm.DB, id uuid.UUID) {
	t.Helper()
	require.NoError(t, db.Create(&domain.Tenant{BaseModel: domain.BaseModel{ID: id}, Name: "Test Tenant"}).Error)
}

func TestVehicleRepository_CreateAndGet(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewVehicleRepository(db)
	ctx := context.Background()

	tenantID := uuid.New()
	createTestTenant(t, db, tenantID)
	v := &domain.Vehicle{TenantID: tenantID, PlateNumber: "AA1234BC", Model: "Ford Transit"}

	require.NoError(t, repo.Create(ctx, v))
	assert.NotEqual(t, uuid.Nil, v.ID)

	got, err := repo.GetByID(ctx, tenantID, v.ID)
	require.NoError(t, err)
	assert.Equal(t, "AA1234BC", got.PlateNumber)
	// The gorm `default:idle` tag on Status means the DB applies this
	// default whenever the Go zero value ("") is inserted — even calling
	// the repository directly (bypassing the service) still gets "idle".
	assert.Equal(t, domain.VehicleIdle, got.Status)
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
	createTestTenant(t, db, tenantA)
	createTestTenant(t, db, tenantB)

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
	createTestTenant(t, db, tenantID)
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
	createTestTenant(t, db, tenantID)
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
	createTestTenant(t, db, tenantID)
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
