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

// fakeVehicleRepo is an in-memory stand-in for repository.VehicleRepository,
// satisfying the same narrow interface the service depends on. It returns
// repository.ErrNotFound so handler-level error mapping stays consistent
// whether the real DB-backed repo or this fake is behind the service.
type fakeVehicleRepo struct {
	vehicles map[uuid.UUID]domain.Vehicle
}

func newFakeVehicleRepo() *fakeVehicleRepo {
	return &fakeVehicleRepo{vehicles: map[uuid.UUID]domain.Vehicle{}}
}

func (f *fakeVehicleRepo) Create(_ context.Context, v *domain.Vehicle) error {
	v.ID = uuid.New()
	f.vehicles[v.ID] = *v
	return nil
}

func (f *fakeVehicleRepo) GetByID(_ context.Context, tenantID, id uuid.UUID) (*domain.Vehicle, error) {
	v, ok := f.vehicles[id]
	if !ok || v.TenantID != tenantID {
		return nil, repository.ErrNotFound
	}
	return &v, nil
}

func (f *fakeVehicleRepo) List(_ context.Context, tenantID uuid.UUID, page, pageSize int) ([]domain.Vehicle, int64, error) {
	var all []domain.Vehicle
	for _, v := range f.vehicles {
		if v.TenantID == tenantID {
			all = append(all, v)
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

func (f *fakeVehicleRepo) Update(_ context.Context, tenantID uuid.UUID, v *domain.Vehicle) error {
	existing, ok := f.vehicles[v.ID]
	if !ok || existing.TenantID != tenantID {
		return repository.ErrNotFound
	}
	f.vehicles[v.ID] = *v
	return nil
}

func (f *fakeVehicleRepo) Delete(_ context.Context, tenantID, id uuid.UUID) error {
	existing, ok := f.vehicles[id]
	if !ok || existing.TenantID != tenantID {
		return repository.ErrNotFound
	}
	delete(f.vehicles, id)
	return nil
}

func TestVehicleService_Create_Validation(t *testing.T) {
	svc := service.NewVehicleService(newFakeVehicleRepo())

	_, err := svc.Create(context.Background(), service.CreateVehicleInput{
		TenantID: uuid.New(),
	})

	assert.ErrorIs(t, err, service.ErrValidation)
}

func TestVehicleService_Create_Success(t *testing.T) {
	svc := service.NewVehicleService(newFakeVehicleRepo())
	tenantID := uuid.New()

	v, err := svc.Create(context.Background(), service.CreateVehicleInput{
		TenantID:    tenantID,
		PlateNumber: "AA1234BC",
	})

	require.NoError(t, err)
	assert.Equal(t, domain.VehicleIdle, v.Status)
	assert.NotEqual(t, uuid.Nil, v.ID)
}

func TestVehicleService_Get_NotFound(t *testing.T) {
	svc := service.NewVehicleService(newFakeVehicleRepo())

	_, err := svc.Get(context.Background(), uuid.New(), uuid.New())

	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestVehicleService_List_ComputesTotalPages(t *testing.T) {
	repo := newFakeVehicleRepo()
	svc := service.NewVehicleService(repo)
	tenantID := uuid.New()

	for i := 0; i < 5; i++ {
		_, err := svc.Create(context.Background(), service.CreateVehicleInput{
			TenantID:    tenantID,
			PlateNumber: "V",
		})
		require.NoError(t, err)
	}

	result, err := svc.List(context.Background(), tenantID, 1, 2)

	require.NoError(t, err)
	assert.Equal(t, int64(5), result.TotalItems)
	assert.Equal(t, 3, result.TotalPages) // 5 items / page size 2 -> 3 pages
}

func TestVehicleService_Update_PartialFields(t *testing.T) {
	repo := newFakeVehicleRepo()
	svc := service.NewVehicleService(repo)
	tenantID := uuid.New()

	v, err := svc.Create(context.Background(), service.CreateVehicleInput{
		TenantID:    tenantID,
		PlateNumber: "OLD",
		Model:       "Old Model",
	})
	require.NoError(t, err)

	newPlate := "NEW"
	updated, err := svc.Update(context.Background(), tenantID, v.ID, service.UpdateVehicleInput{
		PlateNumber: &newPlate,
	})

	require.NoError(t, err)
	assert.Equal(t, "NEW", updated.PlateNumber)
	assert.Equal(t, "Old Model", updated.Model) // untouched field preserved
}

func TestVehicleService_Update_RejectsEmptyPlateNumber(t *testing.T) {
	repo := newFakeVehicleRepo()
	svc := service.NewVehicleService(repo)
	tenantID := uuid.New()

	v, err := svc.Create(context.Background(), service.CreateVehicleInput{
		TenantID:    tenantID,
		PlateNumber: "OLD",
	})
	require.NoError(t, err)

	empty := ""
	_, err = svc.Update(context.Background(), tenantID, v.ID, service.UpdateVehicleInput{
		PlateNumber: &empty,
	})

	assert.ErrorIs(t, err, service.ErrValidation)
}

func TestVehicleService_Delete_NotFound(t *testing.T) {
	svc := service.NewVehicleService(newFakeVehicleRepo())

	err := svc.Delete(context.Background(), uuid.New(), uuid.New())

	assert.ErrorIs(t, err, repository.ErrNotFound)
}
