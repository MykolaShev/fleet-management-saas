package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MykolaShev/fleet-management-saas/internal/mcp"
)

func TestServer_Initialize_ReturnsServerInfo(t *testing.T) {
	s := mcp.NewServer()

	resp := s.Handle(context.Background(), uuid.New(), mcp.Request{JSONRPC: "2.0", Method: "initialize"})

	require.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}

func TestServer_ToolsList_ReturnsRegisteredTools(t *testing.T) {
	s := mcp.NewServer()
	s.RegisterTool(mcp.ToolDefinition{Name: "ping", Description: "test tool"},
		func(_ context.Context, _ uuid.UUID, _ json.RawMessage) (string, error) {
			return "pong", nil
		})

	resp := s.Handle(context.Background(), uuid.New(), mcp.Request{JSONRPC: "2.0", Method: "tools/list"})
	require.Nil(t, resp.Error)

	result, ok := resp.Result.(map[string]any)
	require.True(t, ok, "result should be a map")
	tools, ok := result["tools"].([]mcp.ToolDefinition)
	require.True(t, ok, "tools should be a []ToolDefinition")
	require.Len(t, tools, 1)
	assert.Equal(t, "ping", tools[0].Name)
}

func TestServer_ToolsCall_InvokesRegisteredHandlerWithTenantFromContext(t *testing.T) {
	s := mcp.NewServer()
	var gotTenant uuid.UUID
	var gotArgs json.RawMessage
	s.RegisterTool(mcp.ToolDefinition{Name: "echo"},
		func(_ context.Context, tenantID uuid.UUID, arguments json.RawMessage) (string, error) {
			gotTenant = tenantID
			gotArgs = arguments
			return "ok", nil
		})

	tenant := uuid.New()
	params, err := json.Marshal(map[string]any{"name": "echo", "arguments": json.RawMessage(`{"x":1}`)})
	require.NoError(t, err)

	resp := s.Handle(context.Background(), tenant, mcp.Request{JSONRPC: "2.0", Method: "tools/call", Params: params})

	require.Nil(t, resp.Error)
	assert.Equal(t, tenant, gotTenant)
	assert.JSONEq(t, `{"x":1}`, string(gotArgs))
}

func TestServer_ToolsCall_UnknownTool_ReturnsJSONRPCError(t *testing.T) {
	s := mcp.NewServer()
	params, err := json.Marshal(map[string]any{"name": "does_not_exist", "arguments": json.RawMessage(`{}`)})
	require.NoError(t, err)

	resp := s.Handle(context.Background(), uuid.New(), mcp.Request{JSONRPC: "2.0", Method: "tools/call", Params: params})

	require.NotNil(t, resp.Error)
}

func TestServer_UnknownMethod_ReturnsMethodNotFound(t *testing.T) {
	s := mcp.NewServer()

	resp := s.Handle(context.Background(), uuid.New(), mcp.Request{JSONRPC: "2.0", Method: "no/such/method"})

	require.NotNil(t, resp.Error)
}