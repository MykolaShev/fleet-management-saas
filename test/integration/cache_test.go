package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/MykolaShev/fleet-management-saas/internal/auth"
	"github.com/MykolaShev/fleet-management-saas/internal/cache"
	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/handler"
	"github.com/MykolaShev/fleet-management-saas/internal/repository"
	"github.com/MykolaShev/fleet-management-saas/internal/service"
)

func newCacheTestApp(t *testing.T) (router *gin.Engine, issuer *auth.JWTIssuer, mem *cache.Memory, tenantRepo *repository.TenantRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
	require.NoError(t, db.AutoMigrate(&domain.Tenant{}, &domain.Vehicle{}))

	tenantRepo = repository.NewTenantRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)
	vehicleService := service.NewVehicleService(vehicleRepo)
	vehicleHandler := handler.NewVehicleHandler(vehicleService)

	mem = cache.NewMemory()
	issuer = auth.NewJWTIssuer("test-secret", time.Hour)

	router = gin.New()
	api := router.Group("/api/v1")
	protected := api.Group("", handler.Authenticate(issuer), handler.CacheMiddleware(mem, time.Hour))
	vehicleHandler.RegisterRoutes(protected)

	return router, issuer, mem, tenantRepo
}

func doGET(t *testing.T, router *gin.Engine, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestCache_FirstGetIsMiss_SecondGetIsHitWithSameBody(t *testing.T) {
	router, issuer, _, _ := newCacheTestApp(t)
	token, err := issuer.Issue(uuid.New(), uuid.New(), domain.RoleManager)
	require.NoError(t, err)

	first := doGET(t, router, "/api/v1/vehicles", token)
	second := doGET(t, router, "/api/v1/vehicles", token)

	assert.Equal(t, "MISS", first.Header().Get("X-Cache"))
	assert.Equal(t, "HIT", second.Header().Get("X-Cache"))
	assert.Equal(t, first.Body.String(), second.Body.String())
}

func TestCache_PATCH_InvalidatesSoNextGetIsFreshAndMiss(t *testing.T) {
	router, issuer, _, tenantRepo := newCacheTestApp(t)
	tenantID := uuid.New()
	require.NoError(t, tenantRepo.FindOrCreate(context.Background(), tenantID))
	token, err := issuer.Issue(uuid.New(), tenantID, domain.RoleManager)
	require.NoError(t, err)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/vehicles", strings.NewReader(`{"plate_number":"AA1111BC","model":"Transit"}`))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	// Warm the cache.
	first := doGET(t, router, "/api/v1/vehicles", token)
	require.Equal(t, "MISS", first.Header().Get("X-Cache"))
	cached := doGET(t, router, "/api/v1/vehicles", token)
	require.Equal(t, "HIT", cached.Header().Get("X-Cache"))

	// A second vehicle, via POST — must invalidate what's cached above.
	createReq2 := httptest.NewRequest(http.MethodPost, "/api/v1/vehicles", strings.NewReader(`{"plate_number":"BB2222CD","model":"Sprinter"}`))
	createReq2.Header.Set("Authorization", "Bearer "+token)
	createReq2.Header.Set("Content-Type", "application/json")
	createRec2 := httptest.NewRecorder()
	router.ServeHTTP(createRec2, createReq2)
	require.Equal(t, http.StatusCreated, createRec2.Code)

	after := doGET(t, router, "/api/v1/vehicles", token)
	assert.Equal(t, "MISS", after.Header().Get("X-Cache"), "a successful write must invalidate the cache")
	assert.Contains(t, after.Body.String(), "BB2222CD", "the response after invalidation must reflect the new vehicle, not the stale cached list")
}

func TestCache_DifferentTenants_DoNotShareCacheEntries(t *testing.T) {
	router, issuer, _, _ := newCacheTestApp(t)
	tokenA, err := issuer.Issue(uuid.New(), uuid.New(), domain.RoleManager)
	require.NoError(t, err)
	tokenB, err := issuer.Issue(uuid.New(), uuid.New(), domain.RoleManager)
	require.NoError(t, err)

	first := doGET(t, router, "/api/v1/vehicles", tokenA)
	second := doGET(t, router, "/api/v1/vehicles", tokenB)

	assert.Equal(t, "MISS", first.Header().Get("X-Cache"))
	assert.Equal(t, "MISS", second.Header().Get("X-Cache"), "tenant B must never get a cache hit off tenant A's entry")
}