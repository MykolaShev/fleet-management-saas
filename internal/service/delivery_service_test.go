package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/repository"
	"github.com/MykolaShev/fleet-management-saas/internal/service"
)

type fakeDeliveryRepo struct {
	deliveries map[uuid.UUID]domain.Delivery
}

func newFakeDeliveryRepo() *fakeDeliveryRepo {
	return &fakeDeliveryRepo{deliveries: map[uuid.UUID]domain.Delivery{}}
}

func (f *fakeDeliveryRepo) Create(_ context.Context, d *domain.Delivery) error {
	d.ID = uuid.New()
	f.deliveries[d.ID] = *d
	return nil
}

func (f *fakeDeliveryRepo) GetByID(_ context.Context, tenantID, id uuid.UUID) (*domain.Delivery, error) {
	d, ok := f.deliveries[id]
	if !ok || d.TenantID != tenantID {
		return nil, repository.ErrNotFound
	}
	return &d, nil
}

func (f *fakeDeliveryRepo) List(_ context.Context, tenantID uuid.UUID, page, pageSize int) ([]domain.Delivery, int64, error) {
	var all []domain.Delivery
	for _, d := range f.deliveries {
		if d.TenantID == tenantID {
			all = append(all, d)
		}
	}
	total := int64(len(all))
	start := (page - 1) * pageSize
	if start > len(all) {
		start = len(all)
	}
	end := start + pageSize
	if end > len(all) {
		end = len(all)
	}
	return all[start:end], total, nil
}

func (f *fakeDeliveryRepo) Update(_ context.Context, tenantID uuid.UUID, d *domain.Delivery) error {
	existing, ok := f.deliveries[d.ID]
	if !ok || existing.TenantID != tenantID {
		return repository.ErrNotFound
	}
	f.deliveries[d.ID] = *d
	return nil
}

func (f *fakeDeliveryRepo) Delete(_ context.Context, tenantID, id uuid.UUID) error {
	existing, ok := f.deliveries[id]
	if !ok || existing.TenantID != tenantID {
		return repository.ErrNotFound
	}
	delete(f.deliveries, id)
	return nil
}

// fakeVehicleLookup / fakeUserLookup stand in for the narrow lookup
// interfaces DeliveryService uses to validate assignment, so these tests
// never touch a real database.
type fakeVehicleLookup struct {
	vehicles map[uuid.UUID]domain.Vehicle
}

func (f *fakeVehicleLookup) GetByID(_ context.Context, tenantID, id uuid.UUID) (*domain.Vehicle, error) {
	v, ok := f.vehicles[id]
	if !ok || v.TenantID != tenantID {
		return nil, repository.ErrNotFound
	}
	return &v, nil
}

type fakeUserLookup struct {
	users map[uuid.UUID]domain.User
}

func (f *fakeUserLookup) GetByID(_ context.Context, tenantID, id uuid.UUID) (*domain.User, error) {
	u, ok := f.users[id]
	if !ok || u.TenantID != tenantID {
		return nil, repository.ErrNotFound
	}
	return &u, nil
}

func TestDeliveryService_Create_RequiresAddresses(t *testing.T) {
	svc := service.NewDeliveryService(newFakeDeliveryRepo(), &fakeVehicleLookup{}, &fakeUserLookup{})

	_, err := svc.Create(context.Background(), service.CreateDeliveryInput{
		TenantID: uuid.New(),
	})

	assert.ErrorIs(t, err, service.ErrValidation)
}

func TestDeliveryService_Create_WithoutAssignment_StartsPending(t *testing.T) {
	svc := service.NewDeliveryService(newFakeDeliveryRepo(), &fakeVehicleLookup{}, &fakeUserLookup{})

	d, err := svc.Create(context.Background(), service.CreateDeliveryInput{
		TenantID:       uuid.New(),
		PickupAddress:  "A",
		DropoffAddress: "B",
	})

	require.NoError(t, err)
	assert.Equal(t, domain.DeliveryPending, d.Status)
}

func TestDeliveryService_Create_RejectsVehicleFromOtherTenant(t *testing.T) {
	tenantID := uuid.New()
	otherTenantVehicleID := uuid.New()

	vehicles := &fakeVehicleLookup{vehicles: map[uuid.UUID]domain.Vehicle{
		otherTenantVehicleID: {BaseModel: domain.BaseModel{ID: otherTenantVehicleID}, TenantID: uuid.New()}, // different tenant
	}}

	svc := service.NewDeliveryService(newFakeDeliveryRepo(), vehicles, &fakeUserLookup{})

	_, err := svc.Create(context.Background(), service.CreateDeliveryInput{
		TenantID:       tenantID,
		PickupAddress:  "A",
		DropoffAddress: "B",
		VehicleID:      &otherTenantVehicleID,
	})

	assert.ErrorIs(t, err, service.ErrValidation)
}

func TestDeliveryService_Create_WithValidVehicleAndDriver_BecomesAssigned(t *testing.T) {
	tenantID := uuid.New()
	vehicleID := uuid.New()
	driverID := uuid.New()

	vehicles := &fakeVehicleLookup{vehicles: map[uuid.UUID]domain.Vehicle{
		vehicleID: {BaseModel: domain.BaseModel{ID: vehicleID}, TenantID: tenantID},
	}}
	users := &fakeUserLookup{users: map[uuid.UUID]domain.User{
		driverID: {BaseModel: domain.BaseModel{ID: driverID}, TenantID: tenantID, Role: domain.RoleDriver},
	}}

	svc := service.NewDeliveryService(newFakeDeliveryRepo(), vehicles, users)

	d, err := svc.Create(context.Background(), service.CreateDeliveryInput{
		TenantID:       tenantID,
		PickupAddress:  "A",
		DropoffAddress: "B",
		VehicleID:      &vehicleID,
		DriverID:       &driverID,
	})

	require.NoError(t, err)
	assert.Equal(t, domain.DeliveryAssigned, d.Status)
}

func TestDeliveryService_Create_RejectsNonDriverAsDriver(t *testing.T) {
	tenantID := uuid.New()
	managerID := uuid.New()

	users := &fakeUserLookup{users: map[uuid.UUID]domain.User{
		managerID: {BaseModel: domain.BaseModel{ID: managerID}, TenantID: tenantID, Role: domain.RoleManager},
	}}

	svc := service.NewDeliveryService(newFakeDeliveryRepo(), &fakeVehicleLookup{}, users)

	_, err := svc.Create(context.Background(), service.CreateDeliveryInput{
		TenantID:       tenantID,
		PickupAddress:  "A",
		DropoffAddress: "B",
		DriverID:       &managerID,
	})

	assert.ErrorIs(t, err, service.ErrValidation)
}

func TestDeliveryService_Update_UnassignVehicle(t *testing.T) {
	tenantID := uuid.New()
	vehicleID := uuid.New()

	repo := newFakeDeliveryRepo()
	vehicles := &fakeVehicleLookup{vehicles: map[uuid.UUID]domain.Vehicle{
		vehicleID: {BaseModel: domain.BaseModel{ID: vehicleID}, TenantID: tenantID},
	}}
	svc := service.NewDeliveryService(repo, vehicles, &fakeUserLookup{})

	d, err := svc.Create(context.Background(), service.CreateDeliveryInput{
		TenantID:       tenantID,
		PickupAddress:  "A",
		DropoffAddress: "B",
		VehicleID:      &vehicleID,
	})
	require.NoError(t, err)
	require.NotNil(t, d.VehicleID)

	updated, err := svc.Update(context.Background(), tenantID, d.ID, service.UpdateDeliveryInput{
		UnassignVehicle: true,
	})

	require.NoError(t, err)
	assert.Nil(t, updated.VehicleID)
}

func TestDeliveryService_Update_RejectsInvalidStatus(t *testing.T) {
	repo := newFakeDeliveryRepo()
	svc := service.NewDeliveryService(repo, &fakeVehicleLookup{}, &fakeUserLookup{})
	tenantID := uuid.New()

	d, err := svc.Create(context.Background(), service.CreateDeliveryInput{
		TenantID:       tenantID,
		PickupAddress:  "A",
		DropoffAddress: "B",
	})
	require.NoError(t, err)

	badStatus := domain.DeliveryStatus("lost_in_space")
	_, err = svc.Update(context.Background(), tenantID, d.ID, service.UpdateDeliveryInput{
		Status: &badStatus,
	})

	assert.ErrorIs(t, err, service.ErrValidation)
}

func TestDeliveryService_Delete_NotFound(t *testing.T) {
	svc := service.NewDeliveryService(newFakeDeliveryRepo(), &fakeVehicleLookup{}, &fakeUserLookup{})

	err := svc.Delete(context.Background(), uuid.New(), uuid.New())

	assert.ErrorIs(t, err, repository.ErrNotFound)
}
