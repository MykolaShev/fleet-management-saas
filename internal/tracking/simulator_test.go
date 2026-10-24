package tracking_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/tracking"
)

type fakeVehicleStore struct {
	active    []domain.Vehicle
	updates   map[uuid.UUID][2]float64 // vehicleID -> [lat, lng]
	updateErr error
}

func (f *fakeVehicleStore) ListAllActive(_ context.Context) ([]domain.Vehicle, error) {
	return f.active, nil
}

func (f *fakeVehicleStore) UpdateLocation(_ context.Context, _, id uuid.UUID, lat, lng float64, _ time.Time) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	if f.updates == nil {
		f.updates = map[uuid.UUID][2]float64{}
	}
	f.updates[id] = [2]float64{lat, lng}
	return nil
}

func TestSimulator_Tick_UpdatesAndPublishesEveryActiveVehicle(t *testing.T) {
	tenant := uuid.New()
	v1 := domain.Vehicle{BaseModel: domain.BaseModel{ID: uuid.New()}, TenantID: tenant, Status: domain.VehicleEnRoute}
	store := &fakeVehicleStore{active: []domain.Vehicle{v1}}
	hub := tracking.NewHub()
	ch := hub.Subscribe(tenant)
	defer hub.Unsubscribe(tenant, ch)

	sim := tracking.NewSimulator(store, hub, time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	go sim.Run(ctx)

	select {
	case pos := <-ch:
		assert.Equal(t, v1.ID, pos.VehicleID)
		assert.Equal(t, domain.VehicleEnRoute, pos.Status)
	case <-time.After(time.Second):
		t.Fatal("simulator never published a position")
	}

	_, wasUpdated := store.updates[v1.ID]
	assert.True(t, wasUpdated, "simulator must persist the drifted position")
}

func TestSimulator_Tick_FirstMoveStartsNearDemoOrigin(t *testing.T) {
	tenant := uuid.New()
	v1 := domain.Vehicle{BaseModel: domain.BaseModel{ID: uuid.New()}, TenantID: tenant, Status: domain.VehicleEnRoute} // CurrentLat/Lng nil
	store := &fakeVehicleStore{active: []domain.Vehicle{v1}}
	hub := tracking.NewHub()
	ch := hub.Subscribe(tenant)
	defer hub.Unsubscribe(tenant, ch)

	sim := tracking.NewSimulator(store, hub, time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	go sim.Run(ctx)

	select {
	case pos := <-ch:
		// within one drift step of the documented demo origin — proves
		// a never-tracked vehicle doesn't start at (0, 0) or panic on
		// nil CurrentLat/Lng.
		assert.InDelta(t, 50.4501, pos.Lat, 0.01)
		assert.InDelta(t, 30.5234, pos.Lng, 0.01)
	case <-time.After(time.Second):
		t.Fatal("simulator never published a position")
	}
}

func TestSimulator_Tick_RepositoryError_DoesNotPanicOrPublish(t *testing.T) {
	tenant := uuid.New()
	v1 := domain.Vehicle{BaseModel: domain.BaseModel{ID: uuid.New()}, TenantID: tenant, Status: domain.VehicleEnRoute}
	store := &fakeVehicleStore{active: []domain.Vehicle{v1}, updateErr: assert.AnError}
	hub := tracking.NewHub()
	ch := hub.Subscribe(tenant)
	defer hub.Unsubscribe(tenant, ch)

	sim := tracking.NewSimulator(store, hub, time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	sim.Run(ctx) // synchronous: if this panics, the test fails loudly

	select {
	case <-ch:
		t.Fatal("a failed UpdateLocation must not still publish a position")
	default:
		// expected: nothing published
	}
	require.Empty(t, store.updates)
}