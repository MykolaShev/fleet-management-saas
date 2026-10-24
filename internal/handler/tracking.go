package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/MykolaShev/fleet-management-saas/internal/tracking"
)

// upgrader allows any origin — fine for a course project with no public
// deployment of this endpoint; a real product would restrict this.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// DefaultLongPollTimeout is what cmd/api wires in production. Tests pass
// a much shorter one via NewTrackingHandlerWithTimeout — waiting out a
// real 25s in every test run would make the suite unusably slow.
const DefaultLongPollTimeout = 25 * time.Second

type TrackingHandler struct {
	hub             *tracking.Hub
	longPollTimeout time.Duration
}

func NewTrackingHandler(hub *tracking.Hub) *TrackingHandler {
	return NewTrackingHandlerWithTimeout(hub, DefaultLongPollTimeout)
}

func NewTrackingHandlerWithTimeout(hub *tracking.Hub, timeout time.Duration) *TrackingHandler {
	return &TrackingHandler{hub: hub, longPollTimeout: timeout}
}

func (h *TrackingHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/vehicles/ws", h.WS)
	rg.GET("/vehicles/poll", h.LongPoll)
}

// WS upgrades and pushes every position update for this tenant
// as it happens
func (h *TrackingHandler) WS(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return // upgrader already wrote its own error response
	}
	defer conn.Close()

	ch := h.hub.Subscribe(tID)
	defer h.hub.Unsubscribe(tID, ch)

	for pos := range ch {
		if err := conn.WriteJSON(pos); err != nil {
			return
		}
	}
}

// LongPoll blocks up to longPollTimeout for a position newer than
// ?since, then returns it — or 204 on timeout, so the client immediately
// reissues the request. since defaults to "now" if missing/unparseable.
func (h *TrackingHandler) LongPoll(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	since := time.Now()
	if raw := c.Query("since"); raw != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			since = parsed
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.longPollTimeout)
	defer cancel()

	if !h.hub.Wait(ctx, tID, since) {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated_at": h.hub.LastUpdated(tID)})
}