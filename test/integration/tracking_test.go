package integration_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MykolaShev/fleet-management-saas/internal/auth"
	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/handler"
	"github.com/MykolaShev/fleet-management-saas/internal/tracking"
)

// longPollTestTimeout keeps TestLongPoll_NoUpdate_TimesOutWith204 fast;
// production uses handler.DefaultLongPollTimeout (25s).
const longPollTestTimeout = 150 * time.Millisecond

func newTrackingTestApp(t *testing.T) (*gin.Engine, *auth.JWTIssuer, *tracking.Hub) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	hub := tracking.NewHub()
	trackingHandler := handler.NewTrackingHandlerWithTimeout(hub, longPollTestTimeout)
	jwtIssuer := auth.NewJWTIssuer("test-secret", time.Hour)

	router := gin.New()
	api := router.Group("/api/v1")
	protected := api.Group("", handler.Authenticate(jwtIssuer))
	trackingHandler.RegisterRoutes(protected)

	return router, jwtIssuer, hub
}

func TestTrackingWS_ReceivesPublishedPosition(t *testing.T) {
	router, jwtIssuer, hub := newTrackingTestApp(t)
	server := httptest.NewServer(router)
	defer server.Close()

	tenantID := uuid.New()
	token, err := jwtIssuer.Issue(uuid.New(), tenantID, domain.RoleManager)
	require.NoError(t, err)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/vehicles/ws"
	header := http.Header{"Authorization": {"Bearer " + token}}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	require.NoError(t, err)
	defer conn.Close()

	vehicleID := uuid.New()
	// Give the handler a moment to subscribe before publishing — WS
	// upgrade + Subscribe happens asynchronously relative to Dial
	// returning.
	time.Sleep(20 * time.Millisecond)
	hub.Publish(tenantID, tracking.Position{VehicleID: vehicleID, Status: domain.VehicleEnRoute, Lat: 1, Lng: 2, At: time.Now()})

	var got tracking.Position
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	require.NoError(t, conn.ReadJSON(&got))
	assert.Equal(t, vehicleID, got.VehicleID)
}

func TestTrackingWS_OtherTenantNeverReceives(t *testing.T) {
	router, jwtIssuer, hub := newTrackingTestApp(t)
	server := httptest.NewServer(router)
	defer server.Close()

	tenantA := uuid.New()
	tenantB := uuid.New()
	tokenB, err := jwtIssuer.Issue(uuid.New(), tenantB, domain.RoleManager)
	require.NoError(t, err)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/vehicles/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Authorization": {"Bearer " + tokenB}})
	require.NoError(t, err)
	defer conn.Close()

	time.Sleep(20 * time.Millisecond)
	hub.Publish(tenantA, tracking.Position{VehicleID: uuid.New(), At: time.Now()})

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(100*time.Millisecond)))
	var got tracking.Position
	err = conn.ReadJSON(&got)
	assert.Error(t, err, "tenant B's connection must not receive tenant A's position")
}

func TestLongPoll_ReturnsOnceAnUpdateArrives(t *testing.T) {
	router, jwtIssuer, hub := newTrackingTestApp(t)
	tenantID := uuid.New()
	token, err := jwtIssuer.Issue(uuid.New(), tenantID, domain.RoleManager)
	require.NoError(t, err)

	since := time.Now().Format(time.RFC3339Nano)
	go func() {
		time.Sleep(50 * time.Millisecond)
		hub.Publish(tenantID, tracking.Position{VehicleID: uuid.New(), At: time.Now()})
	}()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vehicles/poll?since="+since, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	start := time.Now()
	router.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Less(t, elapsed, 5*time.Second, "long poll must return promptly once an update arrives, not wait for the full timeout")
}

func TestLongPoll_NoUpdate_TimesOutWith204(t *testing.T) {
	router, jwtIssuer, _ := newTrackingTestApp(t)
	token, err := jwtIssuer.Issue(uuid.New(), uuid.New(), domain.RoleManager)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vehicles/poll", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	start := time.Now()
	router.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.GreaterOrEqual(t, elapsed, longPollTestTimeout)
}