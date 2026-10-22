package integration_test

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MykolaShev/fleet-management-saas/internal/auth"
	"github.com/MykolaShev/fleet-management-saas/internal/domain"
	"github.com/MykolaShev/fleet-management-saas/internal/handler"
	"github.com/MykolaShev/fleet-management-saas/internal/llm"
	"github.com/MykolaShev/fleet-management-saas/internal/service"
)

// fakeLLMClient stands in for a real Groq call. Per docs/TESTING.md
// category 4 (never assert on exact LLM output), these tests only prove
// the SSE wire format is well-formed and that auth/routing behave
// correctly — never anything about what a real model would say.
type fakeLLMClient struct {
	chunks []llm.StreamChunk
}

func (f *fakeLLMClient) StreamChat(_ context.Context, _ []llm.Message) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, len(f.chunks))
	for _, c := range f.chunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

type emptyVehicleLister struct{}

func (emptyVehicleLister) List(_ context.Context, _ uuid.UUID, _, _ int) (*service.ListResult, error) {
	return &service.ListResult{}, nil
}

type emptyDeliveryLister struct{}

func (emptyDeliveryLister) List(_ context.Context, _ uuid.UUID, _, _ int) (*service.DeliveryListResult, error) {
	return &service.DeliveryListResult{}, nil
}

func newAgentTestRouter(fakeLLM *fakeLLMClient) (*gin.Engine, *auth.JWTIssuer) {
	gin.SetMode(gin.TestMode)

	agentService := service.NewAgentService(fakeLLM, emptyVehicleLister{}, emptyDeliveryLister{})
	agentHandler := handler.NewAgentHandler(agentService)

	jwtIssuer := auth.NewJWTIssuer("test-secret", time.Hour)

	router := gin.New()
	api := router.Group("/api/v1")
	protected := api.Group("", handler.Authenticate(jwtIssuer))
	agentHandler.RegisterRoutes(protected)

	return router, jwtIssuer
}

// TestAgentChat_StreamsWellFormedSSEEvents uses a REAL httptest.Server
// (not httptest.NewRecorder) because gin's c.Stream() calls
// ResponseWriter.CloseNotify() internally, which the plain in-memory
// ResponseRecorder doesn't implement — it panics with "missing method
// CloseNotify". A real server's connection does implement it, same as it
// would in production.
func TestAgentChat_StreamsWellFormedSSEEvents(t *testing.T) {
	fakeLLM := &fakeLLMClient{chunks: []llm.StreamChunk{
		{Content: "Vehicle "},
		{Content: "AA1234 is idle."},
		{Done: true},
	}}
	router, jwtIssuer := newAgentTestRouter(fakeLLM)
	server := httptest.NewServer(router)
	defer server.Close()

	token, err := jwtIssuer.Issue(uuid.New(), uuid.New(), domain.RoleManager)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/agent/chat?message=where+is+my+fleet", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	body := string(bodyBytes)

	assert.Contains(t, body, "data: Vehicle ")
	assert.Contains(t, body, "data: AA1234 is idle.")
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

func TestAgentChat_MissingMessage_Returns400(t *testing.T) {
	// No streaming reached here (handler returns 400 before c.Stream()),
	// so the plain recorder is fine.
	router, jwtIssuer := newAgentTestRouter(&fakeLLMClient{})
	token, err := jwtIssuer.Issue(uuid.New(), uuid.New(), domain.RoleManager)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/chat", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestAgentChat_TokenViaQueryParam_WorksForEventSourceCompatibility also
// needs a real server, for the same CloseNotify reason as above — this
// request succeeds all the way through to c.Stream().
func TestAgentChat_TokenViaQueryParam_WorksForEventSourceCompatibility(t *testing.T) {
	fakeLLM := &fakeLLMClient{chunks: []llm.StreamChunk{{Content: "hi"}, {Done: true}}}
	router, jwtIssuer := newAgentTestRouter(fakeLLM)
	server := httptest.NewServer(router)
	defer server.Close()

	token, err := jwtIssuer.Issue(uuid.New(), uuid.New(), domain.RoleManager)
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
	router, _ := newAgentTestRouter(&fakeLLMClient{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/chat?message=hi", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
