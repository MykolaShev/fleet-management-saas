// Package integration_test exercises the real HTTP stack end-to-end
// (router + Authenticate middleware + handler + service + repository,
// against an in-memory SQLite DB) rather than any single layer in
// isolation. This is QA3's evidence: it proves authorization actually
// works as experienced by a caller, not just that each layer's unit tests
// pass individually.
//
// These tests craft JWTs directly via auth.JWTIssuer, bypassing the OAuth
// login endpoints entirely — what's under test here is authorization (does
// a valid/invalid/cross-tenant token get the right response), not the
// OAuth handshake itself. That's covered separately by
// internal/service/auth_service_test.go and the manual browser demo (it
// needs a real browser + Google account, so it isn't automatable here).
package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/MykolaShev/fleet-management-saas/internal/auth"
	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/handler"
	"github.com/MykolaShev/fleet-management-saas/internal/repository"
	"github.com/MykolaShev/fleet-management-saas/internal/service"
)

type testApp struct {
	router     *gin.Engine
	jwtIssuer  *auth.JWTIssuer
	tenantRepo *repository.TenantRepository
}

// newTestApp wires the same handler/service/repository stack as
// cmd/api/main.go, but against an in-memory SQLite DB with foreign keys
// enforced (see docs/TESTING.md for why that pragma matters) and no real
// OAuth config, since these tests never touch the login endpoints.
func newTestApp(t *testing.T) *testApp {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
	require.NoError(t, db.AutoMigrate(
		&domain.Tenant{}, &domain.User{}, &domain.Vehicle{}, &domain.Delivery{}, &domain.FeatureFlag{},
	))

	tenantRepo := repository.NewTenantRepository(db)

	vehicleRepo := repository.NewVehicleRepository(db)
	vehicleHandler := handler.NewVehicleHandler(service.NewVehicleService(vehicleRepo))

	userRepo := repository.NewUserRepository(db)
	userHandler := handler.NewUserHandler(service.NewUserService(userRepo))

	deliveryRepo := repository.NewDeliveryRepository(db)
	deliveryHandler := handler.NewDeliveryHandler(service.NewDeliveryService(deliveryRepo, vehicleRepo, userRepo))

	featureFlagRepo := repository.NewFeatureFlagRepository(db)
	featureFlagHandler := handler.NewFeatureFlagHandler(service.NewFeatureFlagService(featureFlagRepo))

	jwtIssuer := auth.NewJWTIssuer("test-secret", time.Hour)

	router := gin.New()
	api := router.Group("/api/v1")
	protected := api.Group("", handler.Authenticate(jwtIssuer))
	vehicleHandler.RegisterRoutes(protected)
	userHandler.RegisterRoutes(protected)
	deliveryHandler.RegisterRoutes(protected)
	featureFlagHandler.RegisterRoutes(protected)

	return &testApp{router: router, jwtIssuer: jwtIssuer, tenantRepo: tenantRepo}
}

// newTenantToken provisions a fresh tenant (mirroring what the real
// dev-token endpoint and OAuth callback both do) and returns a valid token
// for it.
func (a *testApp) newTenantToken(t *testing.T) (uuid.UUID, string) {
	t.Helper()
	tenantID := uuid.New()
	require.NoError(t, a.tenantRepo.FindOrCreate(context.Background(), tenantID))
	token, err := a.jwtIssuer.Issue(uuid.New(), tenantID, domain.RoleManager)
	require.NoError(t, err)
	return tenantID, token
}

func (a *testApp) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	return rec
}

func TestAuthorization_MissingToken_Returns401(t *testing.T) {
	app := newTestApp(t)

	rec := app.do(t, http.MethodGet, "/api/v1/vehicles", "", nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthorization_GarbageToken_Returns401(t *testing.T) {
	app := newTestApp(t)

	rec := app.do(t, http.MethodGet, "/api/v1/vehicles", "not-a-real-token", nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthorization_ExpiredToken_Returns401(t *testing.T) {
	app := newTestApp(t)

	expiredIssuer := auth.NewJWTIssuer("test-secret", -time.Hour) // issues already-expired tokens, same secret as the app
	expiredToken, err := expiredIssuer.Issue(uuid.New(), uuid.New(), domain.RoleManager)
	require.NoError(t, err)

	rec := app.do(t, http.MethodGet, "/api/v1/vehicles", expiredToken, nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthorization_TokenSignedWithDifferentSecret_Returns401(t *testing.T) {
	app := newTestApp(t)

	foreignIssuer := auth.NewJWTIssuer("a-different-secret", time.Hour)
	foreignToken, err := foreignIssuer.Issue(uuid.New(), uuid.New(), domain.RoleManager)
	require.NoError(t, err)

	rec := app.do(t, http.MethodGet, "/api/v1/vehicles", foreignToken, nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthorization_ValidToken_CanAccessOwnTenantData(t *testing.T) {
	app := newTestApp(t)
	_, token := app.newTenantToken(t)

	rec := app.do(t, http.MethodGet, "/api/v1/vehicles", token, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthorization_CrossTenant_CannotReadAnotherTenantsVehicle(t *testing.T) {
	app := newTestApp(t)
	_, tokenA := app.newTenantToken(t)
	_, tokenB := app.newTenantToken(t)

	createRec := app.do(t, http.MethodPost, "/api/v1/vehicles", tokenA, map[string]string{"plate_number": "AA1111"})
	require.Equal(t, http.StatusCreated, createRec.Code)

	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	// Tenant B, with a perfectly valid token of its own, tries to read
	// tenant A's vehicle by ID. This must come back as 404 — not 403 —
	// so tenant B can't even learn that a vehicle with this ID exists.
	getRec := app.do(t, http.MethodGet, "/api/v1/vehicles/"+created.ID, tokenB, nil)

	assert.Equal(t, http.StatusNotFound, getRec.Code)
}

func TestAuthorization_CrossTenant_CannotDeleteAnotherTenantsVehicle(t *testing.T) {
	app := newTestApp(t)
	_, tokenA := app.newTenantToken(t)
	_, tokenB := app.newTenantToken(t)

	createRec := app.do(t, http.MethodPost, "/api/v1/vehicles", tokenA, map[string]string{"plate_number": "BB2222"})
	require.Equal(t, http.StatusCreated, createRec.Code)
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	deleteRec := app.do(t, http.MethodDelete, "/api/v1/vehicles/"+created.ID, tokenB, nil)
	assert.Equal(t, http.StatusNotFound, deleteRec.Code)

	// And it must still be there, untouched, for tenant A.
	getRec := app.do(t, http.MethodGet, "/api/v1/vehicles/"+created.ID, tokenA, nil)
	assert.Equal(t, http.StatusOK, getRec.Code)
}

func TestAuthorization_CrossTenant_ListDoesNotLeakOtherTenantsVehicles(t *testing.T) {
	app := newTestApp(t)
	_, tokenA := app.newTenantToken(t)
	_, tokenB := app.newTenantToken(t)

	require.Equal(t, http.StatusCreated, app.do(t, http.MethodPost, "/api/v1/vehicles", tokenA, map[string]string{"plate_number": "A1"}).Code)
	require.Equal(t, http.StatusCreated, app.do(t, http.MethodPost, "/api/v1/vehicles", tokenA, map[string]string{"plate_number": "A2"}).Code)
	require.Equal(t, http.StatusCreated, app.do(t, http.MethodPost, "/api/v1/vehicles", tokenB, map[string]string{"plate_number": "B1"}).Code)

	listRec := app.do(t, http.MethodGet, "/api/v1/vehicles", tokenB, nil)
	require.Equal(t, http.StatusOK, listRec.Code)

	var result struct {
		TotalItems int64 `json:"total_items"`
	}
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &result))
	assert.Equal(t, int64(1), result.TotalItems, "tenant B should only see its own single vehicle")
}

func TestAuthorization_CrossTenant_CannotAssignAnotherTenantsVehicleToDelivery(t *testing.T) {
	app := newTestApp(t)
	_, tokenA := app.newTenantToken(t)
	_, tokenB := app.newTenantToken(t)

	createRec := app.do(t, http.MethodPost, "/api/v1/vehicles", tokenA, map[string]string{"plate_number": "FOREIGN1"})
	require.Equal(t, http.StatusCreated, createRec.Code)
	var vehicle struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &vehicle))

	// Tenant B tries to create a delivery assigned to tenant A's vehicle.
	deliveryRec := app.do(t, http.MethodPost, "/api/v1/deliveries", tokenB, map[string]string{
		"pickup_address":  "A",
		"dropoff_address": "B",
		"vehicle_id":      vehicle.ID,
	})

	assert.Equal(t, http.StatusUnprocessableEntity, deliveryRec.Code)
}

func TestAuthorization_CrossTenant_FeatureFlagsAreIsolated(t *testing.T) {
	app := newTestApp(t)
	_, tokenA := app.newTenantToken(t)
	_, tokenB := app.newTenantToken(t)

	setRec := app.do(t, http.MethodPatch, "/api/v1/feature-flags/ai_agent", tokenA, map[string]bool{"enabled": true})
	require.Equal(t, http.StatusOK, setRec.Code)

	// Tenant B enabled nothing — it must still see its own catalog as
	// entirely disabled, unaffected by tenant A's change.
	listRec := app.do(t, http.MethodGet, "/api/v1/feature-flags", tokenB, nil)
	require.Equal(t, http.StatusOK, listRec.Code)

	var result struct {
		Flags []struct {
			Key     string `json:"key"`
			Enabled bool   `json:"enabled"`
		} `json:"flags"`
	}
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &result))
	for _, f := range result.Flags {
		assert.False(t, f.Enabled, "tenant B's flags must be unaffected by tenant A's change")
	}
}
