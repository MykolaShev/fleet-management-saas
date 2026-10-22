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
	"github.com/MykolaShev/fleet-management-saas/internal/mcp"
	"github.com/MykolaShev/fleet-management-saas/internal/repository"
	"github.com/MykolaShev/fleet-management-saas/internal/service"
)

// mcpTestApp wires the real router the same way cmd/api/main.go does for
// the MCP endpoint specifically — this is C22's evidence that the tool
// server works independently of whether the LLM agent calls it correctly
// (it's hit directly over HTTP here, exactly as Postman/curl would).
type mcpTestApp struct {
	router       *gin.Engine
	jwtIssuer    *auth.JWTIssuer
	tenantRepo   *repository.TenantRepository
	vehicleRepo  *repository.VehicleRepository
	deliveryRepo *repository.DeliveryRepository
}

func newMCPTestApp(t *testing.T) *mcpTestApp {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
	require.NoError(t, db.AutoMigrate(&domain.Tenant{}, &domain.User{}, &domain.Vehicle{}, &domain.Delivery{}))

	tenantRepo := repository.NewTenantRepository(db)

	vehicleRepo := repository.NewVehicleRepository(db)
	vehicleService := service.NewVehicleService(vehicleRepo)

	userRepo := repository.NewUserRepository(db)
	deliveryRepo := repository.NewDeliveryRepository(db)
	deliveryService := service.NewDeliveryService(deliveryRepo, vehicleRepo, userRepo)

	mcpServer := mcp.NewServer()
	mcp.RegisterFleetTools(mcpServer, vehicleService, deliveryService, deliveryService)
	mcpHandler := handler.NewMCPHandler(mcpServer)

	jwtIssuer := auth.NewJWTIssuer("test-secret", time.Hour)

	router := gin.New()
	api := router.Group("/api/v1")
	protected := api.Group("", handler.Authenticate(jwtIssuer))
	mcpHandler.RegisterRoutes(protected)

	return &mcpTestApp{router: router, jwtIssuer: jwtIssuer, tenantRepo: tenantRepo, vehicleRepo: vehicleRepo, deliveryRepo: deliveryRepo}
}

func (a *mcpTestApp) post(t *testing.T, token string, req mcp.Request) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/mcp", bytes.NewReader(raw))
	httpReq.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, httpReq)
	return rec
}

func TestMCP_Initialize_ReturnsServerInfo(t *testing.T) {
	app := newMCPTestApp(t)
	token, err := app.jwtIssuer.Issue(uuid.New(), uuid.New(), domain.RoleManager)
	require.NoError(t, err)

	rec := app.post(t, token, mcp.Request{JSONRPC: "2.0", Method: "initialize"})

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp mcp.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Nil(t, resp.Error)
}

func TestMCP_ToolsList_ReturnsTheThreeFleetTools(t *testing.T) {
	app := newMCPTestApp(t)
	token, err := app.jwtIssuer.Issue(uuid.New(), uuid.New(), domain.RoleManager)
	require.NoError(t, err)

	rec := app.post(t, token, mcp.Request{JSONRPC: "2.0", Method: "tools/list"})

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Result struct {
			Tools []mcp.ToolDefinition `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp.Result.Tools, 3)
}

func TestMCP_NoToken_Returns401(t *testing.T) {
	app := newMCPTestApp(t)

	rec := app.post(t, "", mcp.Request{JSONRPC: "2.0", Method: "initialize"})

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestMCP_ToolsCall_CrossTenant_VehicleLocation_IsRejected is this
// project's explicit cross-tenant rejection test for the MCP tool layer
// (mirroring QA3's authorization tests, but for /api/v1/mcp specifically):
// tenant B's valid token must not be able to read tenant A's vehicle
// through get_vehicle_location, even though the tool call itself succeeds
// at the HTTP/JSON-RPC level.
func TestMCP_ToolsCall_CrossTenant_VehicleLocation_IsRejected(t *testing.T) {
	app := newMCPTestApp(t)

	tenantA := uuid.New()
	tenantB := uuid.New()
	require.NoError(t, app.tenantRepo.FindOrCreate(context.Background(), tenantA))
	require.NoError(t, app.tenantRepo.FindOrCreate(context.Background(), tenantB))

	vehicle := &domain.Vehicle{TenantID: tenantA, PlateNumber: "FOREIGN1", Model: "Transit", Status: domain.VehicleIdle}
	require.NoError(t, app.vehicleRepo.Create(context.Background(), vehicle))

	tokenB, err := app.jwtIssuer.Issue(uuid.New(), tenantB, domain.RoleManager)
	require.NoError(t, err)

	params, err := json.Marshal(map[string]any{
		"name":      "get_vehicle_location",
		"arguments": json.RawMessage(`{"vehicle_id":"` + vehicle.ID.String() + `"}`),
	})
	require.NoError(t, err)

	rec := app.post(t, tokenB, mcp.Request{JSONRPC: "2.0", Method: "tools/call", Params: params})

	// The HTTP call itself succeeds (200) — JSON-RPC carries the failure
	// inside the envelope, same as a well-formed "method not found" would.
	assert.Equal(t, http.StatusOK, rec.Code)
	var resp mcp.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotNil(t, resp.Error, "cross-tenant vehicle lookup must not succeed")
}

func TestMCP_ToolsCall_UpdateDeliveryStatus_WorksForOwnTenant(t *testing.T) {
	app := newMCPTestApp(t)

	tenantID := uuid.New()
	require.NoError(t, app.tenantRepo.FindOrCreate(context.Background(), tenantID))
	token, err := app.jwtIssuer.Issue(uuid.New(), tenantID, domain.RoleManager)
	require.NoError(t, err)

	// Create a delivery directly via the same repo the service layer
	// uses — no vehicle/driver handler is wired into this MCP-only test
	// router, so we don't go through HTTP for setup here.
	delivery := &domain.Delivery{TenantID: tenantID, PickupAddress: "A", DropoffAddress: "B", Status: domain.DeliveryAssigned}
	require.NoError(t, app.deliveryRepo.Create(context.Background(), delivery))

	params, err := json.Marshal(map[string]any{
		"name":      "update_delivery_status",
		"arguments": json.RawMessage(`{"delivery_id":"` + delivery.ID.String() + `","status":"in_transit"}`),
	})
	require.NoError(t, err)

	rec := app.post(t, token, mcp.Request{JSONRPC: "2.0", Method: "tools/call", Params: params})

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp mcp.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Nil(t, resp.Error)

	updated, err := app.deliveryRepo.GetByID(context.Background(), tenantID, delivery.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.DeliveryInTransit, updated.Status)
}