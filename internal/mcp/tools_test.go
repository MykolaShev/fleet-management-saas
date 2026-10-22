package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/mcp"
)

// errNotFound stands in for repository.ErrNotFound without importing the
// repository package — the tool handlers only care that Get/UpdateStatus
// returned *some* error, never which sentinel it was.
var errNotFound = errors.New("not found")

// fakeVehicles is scoped per-tenant on purpose: Get on the wrong tenant
// behaves exactly like the real repository would for a cross-tenant
// lookup (ErrNotFound, not a leak of the other tenant's data).
type fakeVehicles struct {
	byTenant map[uuid.UUID]map[uuid.UUID]domain.Vehicle
}

func (f *fakeVehicles) Get(_ context.Context, tenantID, id uuid.UUID) (*domain.Vehicle, error) {
	tenantVehicles, ok := f.byTenant[tenantID]
	if !ok {
		return nil, errNotFound
	}
	v, ok := tenantVehicles[id]
	if !ok {
		return nil, errNotFound
	}
	return &v, nil
}

type fakeDeliveries struct {
	byTenant map[uuid.UUID]map[uuid.UUID]domain.Delivery
}

func (f *fakeDeliveries) Get(_ context.Context, tenantID, id uuid.UUID) (*domain.Delivery, error) {
	tenantDeliveries, ok := f.byTenant[tenantID]
	if !ok {
		return nil, errNotFound
	}
	d, ok := tenantDeliveries[id]
	if !ok {
		return nil, errNotFound
	}
	return &d, nil
}

func (f *fakeDeliveries) UpdateStatus(_ context.Context, tenantID, id uuid.UUID, status domain.DeliveryStatus) (*domain.Delivery, error) {
	tenantDeliveries, ok := f.byTenant[tenantID]
	if !ok {
		return nil, errNotFound
	}
	d, ok := tenantDeliveries[id]
	if !ok {
		return nil, errNotFound
	}
	d.Status = status
	tenantDeliveries[id] = d
	return &d, nil
}

func newServerWithFleetTools() (*mcp.Server, *fakeVehicles, *fakeDeliveries) {
	vehicles := &fakeVehicles{byTenant: map[uuid.UUID]map[uuid.UUID]domain.Vehicle{}}
	deliveries := &fakeDeliveries{byTenant: map[uuid.UUID]map[uuid.UUID]domain.Delivery{}}
	s := mcp.NewServer()
	mcp.RegisterFleetTools(s, vehicles, deliveries, deliveries)
	return s, vehicles, deliveries
}

func TestGetVehicleLocation_NoPositionYet_ReturnsHonestStub(t *testing.T) {
	s, vehicles, _ := newServerWithFleetTools()

	tenant := uuid.New()
	vehicleID := uuid.New()
	vehicles.byTenant[tenant] = map[uuid.UUID]domain.Vehicle{
		vehicleID: {BaseModel: domain.BaseModel{ID: vehicleID}, PlateNumber: "AA1234BC"},
	}

	result, err := s.Call(context.Background(), tenant, "get_vehicle_location", json.RawMessage(`{"vehicle_id":"`+vehicleID.String()+`"}`))

	require.NoError(t, err)
	assert.Contains(t, result, "has not yet reported")
	assert.Contains(t, result, "AA1234BC")
}

func TestGetVehicleLocation_WithPosition_ReturnsCoordinates(t *testing.T) {
	s, vehicles, _ := newServerWithFleetTools()

	tenant := uuid.New()
	vehicleID := uuid.New()
	lat, lng := 50.4501, 30.5234
	vehicles.byTenant[tenant] = map[uuid.UUID]domain.Vehicle{
		vehicleID: {BaseModel: domain.BaseModel{ID: vehicleID}, PlateNumber: "AA1234BC", CurrentLat: &lat, CurrentLng: &lng},
	}

	result, err := s.Call(context.Background(), tenant, "get_vehicle_location", json.RawMessage(`{"vehicle_id":"`+vehicleID.String()+`"}`))

	require.NoError(t, err)
	assert.Contains(t, result, "50.450100")
	assert.Contains(t, result, "30.523400")
}

func TestGetVehicleLocation_CrossTenant_IsRejected(t *testing.T) {
	s, vehicles, _ := newServerWithFleetTools()

	ownerTenant := uuid.New()
	otherTenant := uuid.New()
	vehicleID := uuid.New()
	vehicles.byTenant[ownerTenant] = map[uuid.UUID]domain.Vehicle{
		vehicleID: {BaseModel: domain.BaseModel{ID: vehicleID}, PlateNumber: "AA1234BC"},
	}

	// otherTenant asks for a vehicle that only exists under ownerTenant.
	_, err := s.Call(context.Background(), otherTenant, "get_vehicle_location", json.RawMessage(`{"vehicle_id":"`+vehicleID.String()+`"}`))

	assert.ErrorIs(t, err, errNotFound)
}

func TestGetDeliveryETA_ReturnsHonestNotAvailableStub(t *testing.T) {
	s, _, deliveries := newServerWithFleetTools()

	tenant := uuid.New()
	deliveryID := uuid.New()
	deliveries.byTenant[tenant] = map[uuid.UUID]domain.Delivery{
		deliveryID: {BaseModel: domain.BaseModel{ID: deliveryID}, Status: domain.DeliveryInTransit},
	}

	result, err := s.Call(context.Background(), tenant, "get_delivery_eta", json.RawMessage(`{"delivery_id":"`+deliveryID.String()+`"}`))

	require.NoError(t, err)
	assert.Contains(t, result, "not available")
	assert.Contains(t, result, "in_transit")
}

func TestUpdateDeliveryStatus_UpdatesAndReturnsConfirmation(t *testing.T) {
	s, _, deliveries := newServerWithFleetTools()

	tenant := uuid.New()
	deliveryID := uuid.New()
	deliveries.byTenant[tenant] = map[uuid.UUID]domain.Delivery{
		deliveryID: {BaseModel: domain.BaseModel{ID: deliveryID}, Status: domain.DeliveryAssigned},
	}

	result, err := s.Call(context.Background(), tenant, "update_delivery_status",
		json.RawMessage(`{"delivery_id":"`+deliveryID.String()+`","status":"in_transit"}`))

	require.NoError(t, err)
	assert.Contains(t, result, "in_transit")
	assert.Equal(t, domain.DeliveryInTransit, deliveries.byTenant[tenant][deliveryID].Status)
}

func TestUpdateDeliveryStatus_CrossTenant_IsRejected(t *testing.T) {
	s, _, deliveries := newServerWithFleetTools()

	ownerTenant := uuid.New()
	otherTenant := uuid.New()
	deliveryID := uuid.New()
	deliveries.byTenant[ownerTenant] = map[uuid.UUID]domain.Delivery{
		deliveryID: {BaseModel: domain.BaseModel{ID: deliveryID}, Status: domain.DeliveryAssigned},
	}

	_, err := s.Call(context.Background(), otherTenant, "update_delivery_status",
		json.RawMessage(`{"delivery_id":"`+deliveryID.String()+`","status":"in_transit"}`))

	assert.ErrorIs(t, err, errNotFound)
	// And the record under its real tenant is untouched.
	assert.Equal(t, domain.DeliveryAssigned, deliveries.byTenant[ownerTenant][deliveryID].Status)
}