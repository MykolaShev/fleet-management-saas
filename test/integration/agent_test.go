package integration_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
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
	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/handler"
	"github.com/MykolaShev/fleet-management-saas/internal/llm"
	"github.com/MykolaShev/fleet-management-saas/internal/mcp"
	"github.com/MykolaShev/fleet-management-saas/internal/repository"
	"github.com/MykolaShev/fleet-management-saas/internal/service"
)

// fakeLLMClient stands in for a real Groq call. Per docs/TESTING.md
// category 4 (never assert on exact LLM output), these tests only prove
// the SSE wire format and the real tool-calling path (AgentService ->
// mcp.Server -> VehicleService/DeliveryService -> DB) behave correctly —
// never anything about what a real model would say.
type fakeLLMClient struct {
	chatResponse *llm.ChatResponse
	streamChunks []llm.StreamChunk
}

func (f *fakeLLMClient) Chat(_ context.Context, _ []llm.Message, _ []llm.Tool) (*llm.ChatResponse, error) {
	return f.chatResponse, nil
}

func (f *fakeLLMClient) StreamChat(_ context.Context, _ []llm.Message) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, len(f.streamChunks))
	for _, c := range f.streamChunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

// agentTestApp wires the real service/repository/mcp stack (in-memory
// SQLite with foreign keys enforced, same as authorization_test.go)
// behind a fake LLM client, so these tests can exercise a genuine
// tool-calling round trip without depending on a real Groq account.
type agentTestApp struct {
	router      *gin.Engine
	jwtIssuer   *auth.JWTIssuer
	vehicleRepo *repository.VehicleRepository
	tenantRepo  *repository.TenantRepository
}

func newAgentTestApp(t *testing.T, fakeLLM *fakeLLMClient) *agentTestApp {
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
	userService := service.NewUserService(userRepo)
	deliveryRepo := repository.NewDeliveryRepository(db)
	deliveryService := service.NewDeliveryService(deliveryRepo, vehicleRepo, userRepo)

	mcpServer := mcp.NewServer()
	mcp.RegisterFleetTools(mcpServer, vehicleService, deliveryService, deliveryService, userService)

	agentService := service.NewAgentService(fakeLLM, vehicleService, deliveryService, mcpServer)
	agentHandler := handler.NewAgentHandler(agentService)

	jwtIssuer := auth.NewJWTIssuer("test-secret", time.Hour)

	router := gin.New()
	api := router.Group("/api/v1")
	protected := api.Group("", handler.Authenticate(jwtIssuer))
	agentHandler.RegisterRoutes(protected)

	return &agentTestApp{router: router, jwtIssuer: jwtIssuer, vehicleRepo: vehicleRepo, tenantRepo: tenantRepo}
}

// TestAgentChat_StreamsWellFormedSSEEvents uses a REAL httptest.Server
// (not httptest.NewRecorder) because gin's c.Stream() calls
// ResponseWriter.CloseNotify() internally, which the plain in-memory
// ResponseRecorder doesn't implement — it panics with "missing method
// CloseNotify". A real server's connection does implement it, same as it
// would in production.
func TestAgentChat_StreamsWellFormedSSEEvents(t *testing.T) {
	fakeLLM := &fakeLLMClient{chatResponse: &llm.ChatResponse{Content: "Vehicle AA1234 is idle."}}
	app := newAgentTestApp(t, fakeLLM)
	server := httptest.NewServer(app.router)
	defer server.Close()

	token, err := app.jwtIssuer.Issue(uuid.New(), uuid.New(), domain.RoleManager)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/agent/chat?message=where+is+my+fleet", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream; charset=utf-8", resp.Header.Get("Content-Type"))

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	body := string(bodyBytes)

	assert.Contains(t, body, "data: Vehicle AA1234 is idle.")
	assert.Contains(t, body, "event: done")

	// Structural contract: every non-blank line must start with "data:"
	// or "event:" — this is the proof called for in docs/TESTING.md
	// category 4, not an assertion about what the (fake) model said.
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		assert.True(t, strings.HasPrefix(line, "data:") || strings.HasPrefix(line, "event:"),
			"malformed SSE line: %q", line)
	}
}

// TestAgentChat_ToolCallPath_ShowsToolCallAndResultInRawSSE is C21/C22's
// key piece of evidence: with no frontend yet to render tool-calling
// specially, this proves the raw SSE stream itself already exposes which
// tool fired and what it returned, through the real mcp.Server and a real
// (SQLite-backed) VehicleService — only the LLM provider is faked.
func TestAgentChat_ToolCallPath_ShowsToolCallAndResultInRawSSE(t *testing.T) {
	fakeLLM := &fakeLLMClient{}
	app := newAgentTestApp(t, fakeLLM)

	tenantID := uuid.New()
	require.NoError(t, app.tenantRepo.FindOrCreate(context.Background(), tenantID))

	vehicle := &domain.Vehicle{TenantID: tenantID, PlateNumber: "AA1234BC", Model: "Transit", Status: domain.VehicleIdle}
	require.NoError(t, app.vehicleRepo.Create(context.Background(), vehicle))

	argsJSON, err := json.Marshal(map[string]string{"vehicle_id": vehicle.ID.String()})
	require.NoError(t, err)

	fakeLLM.chatResponse = &llm.ChatResponse{
		ToolCalls: []llm.ToolCall{{
			ID:       "call_1",
			Type:     "function",
			Function: llm.ToolCallFunction{Name: "get_vehicle_location", Arguments: string(argsJSON)},
		}},
	}
	fakeLLM.streamChunks = []llm.StreamChunk{
		{Content: "It hasn't reported "},
		{Content: "a GPS position yet."},
		{Done: true},
	}

	server := httptest.NewServer(app.router)
	defer server.Close()

	token, err := app.jwtIssuer.Issue(uuid.New(), tenantID, domain.RoleManager)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/agent/chat?message=where+is+AA1234BC", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	body := string(bodyBytes)

	assert.Contains(t, body, "event: tool_call")
	assert.Contains(t, body, "get_vehicle_location")
	assert.Contains(t, body, "event: tool_result")
	assert.Contains(t, body, "has not yet reported")
	assert.Contains(t, body, "It hasn't reported ")
	assert.Contains(t, body, "event: done")
}

func TestAgentChat_MissingMessage_Returns400(t *testing.T) {
	// No streaming reached here (handler returns 400 before c.Stream()),
	// so the plain recorder is fine.
	app := newAgentTestApp(t, &fakeLLMClient{})
	token, err := app.jwtIssuer.Issue(uuid.New(), uuid.New(), domain.RoleManager)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/chat", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	app.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestAgentChat_TokenViaQueryParam_WorksForEventSourceCompatibility also
// needs a real server, for the same CloseNotify reason as above — this
// request succeeds all the way through to c.Stream().
func TestAgentChat_TokenViaQueryParam_WorksForEventSourceCompatibility(t *testing.T) {
	fakeLLM := &fakeLLMClient{chatResponse: &llm.ChatResponse{Content: "hi"}}
	app := newAgentTestApp(t, fakeLLM)
	server := httptest.NewServer(app.router)
	defer server.Close()

	token, err := app.jwtIssuer.Issue(uuid.New(), uuid.New(), domain.RoleManager)
	require.NoError(t, err)

	// No Authorization header at all — only the query param, exactly how
	// a browser's native EventSource would have to call this endpoint.
	resp, err := http.Get(server.URL + "/api/v1/agent/chat?message=hi&token=" + token)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAgentChat_NoToken_Returns401(t *testing.T) {
	// Rejected by Authenticate before the handler (and c.Stream()) ever
	// runs, so the plain recorder is fine here too.
	app := newAgentTestApp(t, &fakeLLMClient{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/chat?message=hi", nil)
	rec := httptest.NewRecorder()

	app.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}