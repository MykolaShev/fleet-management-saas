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

func setupDeliveryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := setupTestDB(t) // Tenant + Vehicle, from vehicle_repository_test.go
	require.NoError(t, db.AutoMigrate(&domain.User{}, &domain.Delivery{}))
	return db
}

func TestDeliveryRepository_CreateAndGet(t *testing.T) {
	db := setupDeliveryTestDB(t)
	repo := repository.NewDeliveryRepository(db)
	ctx := context.Background()

	tenantID := uuid.New()
	createTestTenant(t, db, tenantID)
	d := &domain.Delivery{
		TenantID:       tenantID,
		Status:         domain.DeliveryPending,
		PickupAddress:  "Warehouse 1",
		DropoffAddress: "Client St 42",
	}

	require.NoError(t, repo.Create(ctx, d))
	assert.NotEqual(t, uuid.Nil, d.ID)

	got, err := repo.GetByID(ctx, tenantID, d.ID)
	require.NoError(t, err)
	assert.Equal(t, "Warehouse 1", got.PickupAddress)
	assert.Nil(t, got.VehicleID)
}

func TestDeliveryRepository_AssignThenUnassignVehicle(t *testing.T) {
	db := setupDeliveryTestDB(t)
	repo := repository.NewDeliveryRepository(db)
	ctx := context.Background()

	tenantID := uuid.New()
	createTestTenant(t, db, tenantID)
	d := &domain.Delivery{TenantID: tenantID, Status: domain.DeliveryPending, PickupAddress: "A", DropoffAddress: "B"}
	require.NoError(t, repo.Create(ctx, d))

	// Delivery.VehicleID has its own FK to vehicles.id, so — same as the
	// tenant FK above — this needs a real Vehicle row, not just any UUID.
	vehicle := &domain.Vehicle{TenantID: tenantID, PlateNumber: "ASSIGN1"}
	require.NoError(t, repository.NewVehicleRepository(db).Create(ctx, vehicle))
	vehicleID := vehicle.ID
	d.VehicleID = &vehicleID
	d.Status = domain.DeliveryAssigned
	require.NoError(t, repo.Update(ctx, tenantID, d))

	got, err := repo.GetByID(ctx, tenantID, d.ID)
	require.NoError(t, err)
	require.NotNil(t, got.VehicleID)
	assert.Equal(t, vehicleID, *got.VehicleID)

	// Unassign: nil pointer must persist as NULL, not be silently skipped
	// (same GORM struct-Updates footgun as before, this time on a pointer
	// field instead of a plain string).
	d.VehicleID = nil
	require.NoError(t, repo.Update(ctx, tenantID, d))

	got, err = repo.GetByID(ctx, tenantID, d.ID)
	require.NoError(t, err)
	assert.Nil(t, got.VehicleID)
}

func TestDeliveryRepository_List_IsScopedToTenant(t *testing.T) {
	db := setupDeliveryTestDB(t)
	repo := repository.NewDeliveryRepository(db)
	ctx := context.Background()

	tenantA := uuid.New()
	tenantB := uuid.New()
	createTestTenant(t, db, tenantA)
	createTestTenant(t, db, tenantB)

	require.NoError(t, repo.Create(ctx, &domain.Delivery{TenantID: tenantA, PickupAddress: "A1", DropoffAddress: "B1"}))
	require.NoError(t, repo.Create(ctx, &domain.Delivery{TenantID: tenantA, PickupAddress: "A2", DropoffAddress: "B2"}))
	require.NoError(t, repo.Create(ctx, &domain.Delivery{TenantID: tenantB, PickupAddress: "A3", DropoffAddress: "B3"}))

	items, total, err := repo.List(ctx, tenantA, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, items, 2)
}

func TestDeliveryRepository_Update_NotFound(t *testing.T) {
	db := setupDeliveryTestDB(t)
	repo := repository.NewDeliveryRepository(db)
	ctx := context.Background()

	err := repo.Update(ctx, uuid.New(), &domain.Delivery{BaseModel: domain.BaseModel{ID: uuid.New()}})
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestDeliveryRepository_Delete(t *testing.T) {
	db := setupDeliveryTestDB(t)
	repo := repository.NewDeliveryRepository(db)
	ctx := context.Background()

	tenantID := uuid.New()
	createTestTenant(t, db, tenantID)
	d := &domain.Delivery{TenantID: tenantID, PickupAddress: "A", DropoffAddress: "B"}
	require.NoError(t, repo.Create(ctx, d))

	require.NoError(t, repo.Delete(ctx, tenantID, d.ID))

	_, err := repo.GetByID(ctx, tenantID, d.ID)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}
