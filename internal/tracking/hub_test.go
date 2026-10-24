package tracking_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MykolaShev/fleet-management-saas/internal/tracking"
)

func TestHub_PublishDeliversToSubscriber(t *testing.T) {
	h := tracking.NewHub()
	tenant := uuid.New()
	ch := h.Subscribe(tenant)
	defer h.Unsubscribe(tenant, ch)

	want := tracking.Position{VehicleID: uuid.New(), Lat: 1, Lng: 2, At: time.Now()}
	h.Publish(tenant, want)

	select {
	case got := <-ch:
		assert.Equal(t, want.VehicleID, got.VehicleID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for published position")
	}
}

func TestHub_TenantIsolation_OtherTenantNeverReceives(t *testing.T) {
	h := tracking.NewHub()
	tenantA, tenantB := uuid.New(), uuid.New()
	chB := h.Subscribe(tenantB)
	defer h.Unsubscribe(tenantB, chB)

	h.Publish(tenantA, tracking.Position{VehicleID: uuid.New(), At: time.Now()})

	select {
	case <-chB:
		t.Fatal("tenant B must never receive tenant A's updates")
	case <-time.After(50 * time.Millisecond):
		// expected: nothing arrived
	}
}

func TestHub_Wait_ReturnsTrueOnNewerUpdate(t *testing.T) {
	h := tracking.NewHub()
	tenant := uuid.New()
	since := time.Now()

	go func() {
		time.Sleep(10 * time.Millisecond)
		h.Publish(tenant, tracking.Position{VehicleID: uuid.New(), At: time.Now()})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.True(t, h.Wait(ctx, tenant, since))
}

func TestHub_Wait_AlreadyNewerUpdate_ReturnsImmediately(t *testing.T) {
	h := tracking.NewHub()
	tenant := uuid.New()
	since := time.Now()
	h.Publish(tenant, tracking.Position{VehicleID: uuid.New(), At: since.Add(time.Second)})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	require.True(t, h.Wait(ctx, tenant, since))
}

func TestHub_Wait_TimesOutWithNoUpdate(t *testing.T) {
	h := tracking.NewHub()
	tenant := uuid.New()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	assert.False(t, h.Wait(ctx, tenant, time.Now()))
}