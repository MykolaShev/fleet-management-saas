// Package tracking simulates and distributes live vehicle positions —
// the C10 stage. One Hub backs all three connectivity mechanisms: WS
// subscribes to it directly, long polling calls Wait, adaptive polling
// doesn't touch it at all (it just re-polls the existing vehicles list
// endpoint on a client-adjusted interval).
package tracking

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
)

// Position is one vehicle's new location.
type Position struct {
	VehicleID uuid.UUID            `json:"vehicle_id"`
	Status    domain.VehicleStatus `json:"status"`
	Lat       float64              `json:"lat"`
	Lng       float64              `json:"lng"`
	At        time.Time            `json:"at"`
}

// Hub fans out positions to per-tenant subscribers and tracks each
// tenant's latest update time.
type Hub struct {
	mu          sync.Mutex
	subs        map[uuid.UUID]map[chan Position]struct{}
	lastUpdated map[uuid.UUID]time.Time
}

func NewHub() *Hub {
	return &Hub{
		subs:        make(map[uuid.UUID]map[chan Position]struct{}),
		lastUpdated: make(map[uuid.UUID]time.Time),
	}
}

// Subscribe registers a channel for tenantID's updates. Buffered so one
// slow subscriber can't block Publish for everyone else; Unsubscribe must
// be called once done (defer right after Subscribe).
func (h *Hub) Subscribe(tenantID uuid.UUID) chan Position {
	ch := make(chan Position, 16)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[tenantID] == nil {
		h.subs[tenantID] = make(map[chan Position]struct{})
	}
	h.subs[tenantID][ch] = struct{}{}
	return ch
}

func (h *Hub) Unsubscribe(tenantID uuid.UUID, ch chan Position) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subs[tenantID], ch)
	close(ch)
}

// Publish broadcasts pos to tenantID's current subscribers and records
// the time. A full channel drops the update rather than blocking the
// simulator — a slow WS client misses one tick, not a correctness bug on
// a live map.
func (h *Hub) Publish(tenantID uuid.UUID, pos Position) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastUpdated[tenantID] = pos.At
	for ch := range h.subs[tenantID] {
		select {
		case ch <- pos:
		default:
		}
	}
}

// LastUpdated reports when tenantID's fleet last changed.
func (h *Hub) LastUpdated(tenantID uuid.UUID) time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastUpdated[tenantID]
}

// Wait blocks until tenantID's fleet changes after since, or ctx ends.
// Returns false on timeout/cancel. This is long polling's entire
// mechanism — subscribe like a WS client would, but return on the first
// relevant update instead of streaming forever.
func (h *Hub) Wait(ctx context.Context, tenantID uuid.UUID, since time.Time) bool {
	if h.LastUpdated(tenantID).After(since) {
		return true
	}
	ch := h.Subscribe(tenantID)
	defer h.Unsubscribe(tenantID, ch)

	for {
		select {
		case <-ctx.Done():
			return false
		case pos, open := <-ch:
			if !open {
				return false
			}
			if pos.At.After(since) {
				return true
			}
		}
	}
}