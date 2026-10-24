package tracking

import (
	"context"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
)

// demoOriginLat/Lng is an arbitrary starting point (Kyiv) for a vehicle
// that has never reported a position — same honest-stub spirit as
// get_delivery_eta: there's no real GPS feed, so a simulated one has to
// start somewhere.
const (
	demoOriginLat = 50.4501
	demoOriginLng = 30.5234
	driftStepDeg  = 0.0015 // ~150m per tick at this latitude
)

// vehicleStore is the narrow slice of *repository.VehicleRepository the
// simulator needs.
type vehicleStore interface {
	ListAllActive(ctx context.Context) ([]domain.Vehicle, error)
	UpdateLocation(ctx context.Context, tenantID, id uuid.UUID, lat, lng float64, at time.Time) error
}

// Simulator nudges every non-idle vehicle's position by a small random
// drift on each tick and publishes the change via Hub — this project's
// stand-in for a real GPS feed.
type Simulator struct {
	repo     vehicleStore
	hub      *Hub
	interval time.Duration
}

func NewSimulator(repo vehicleStore, hub *Hub, interval time.Duration) *Simulator {
	return &Simulator{repo: repo, hub: hub, interval: interval}
}

// Run ticks until ctx is done. Call it in its own goroutine.
func (s *Simulator) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Simulator) tick(ctx context.Context) {
	vehicles, err := s.repo.ListAllActive(ctx)
	if err != nil {
		log.Printf("tracking simulator: list active vehicles: %v", err)
		return
	}

	for _, v := range vehicles {
		lat, lng := nextPosition(v)
		now := time.Now()

		if err := s.repo.UpdateLocation(ctx, v.TenantID, v.ID, lat, lng, now); err != nil {
			log.Printf("tracking simulator: update vehicle %s: %v", v.ID, err)
			continue
		}
		s.hub.Publish(v.TenantID, Position{VehicleID: v.ID, Status: v.Status, Lat: lat, Lng: lng, At: now})
	}
}

// nextPosition starts a never-tracked vehicle near demoOrigin, then drifts
// by a small random step each tick so it visibly moves without teleporting.
func nextPosition(v domain.Vehicle) (lat, lng float64) {
	lat, lng = demoOriginLat, demoOriginLng
	if v.CurrentLat != nil && v.CurrentLng != nil {
		lat, lng = *v.CurrentLat, *v.CurrentLng
	}
	lat += (rand.Float64()*2 - 1) * driftStepDeg
	lng += (rand.Float64()*2 - 1) * driftStepDeg
	return lat, lng
}