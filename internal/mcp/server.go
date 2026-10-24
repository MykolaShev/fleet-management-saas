package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// ToolDefinition is what tools/list advertises — name, human-readable
// description, and a JSON Schema for its arguments (the shape every
// OpenAI-compatible tool-calling API expects).
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ToolHandler executes one tool call. tenantID always comes from the
// authenticated HTTP context (see handler.MCPHandler) or, when
// AgentService calls Call directly in-process, from the agent's own
// authenticated request — never from the JSON-RPC request body itself.
type ToolHandler func(ctx context.Context, tenantID uuid.UUID, arguments json.RawMessage) (string, error)

type registeredTool struct {
	def     ToolDefinition
	handler ToolHandler
}

// Server is a minimal MCP server: a fixed registry of tools plus
// initialize/tools/list/tools/call dispatch.
type Server struct {
	tools map[string]registeredTool
}

// NewServer returns an empty server; register tools with RegisterTool.
func NewServer() *Server {
	return &Server{tools: make(map[string]registeredTool)}
}

// RegisterTool adds a tool to the registry. Called at startup only (see
// cmd/api/main.go) — never concurrently with Handle/Call — so no locking.
func (s *Server) RegisterTool(def ToolDefinition, handler ToolHandler) {
	s.tools[def.Name] = registeredTool{def: def, handler: handler}
}

// Definitions returns every registered tool's definition, in a stable
// shape for both tools/list and AgentService's own use.
func (s *Server) Definitions() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(s.tools))
	for _, t := range s.tools {
		defs = append(defs, t.def)
	}
	return defs
}

// Call invokes one registered tool directly, with no JSON-RPC envelope.
func (s *Server) Call(ctx context.Context, tenantID uuid.UUID, name string, arguments json.RawMessage) (string, error) {
	t, ok := s.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	return t.handler(ctx, tenantID, arguments)
}

// Handle dispatches one JSON-RPC 2.0 request — the path
// handler.MCPHandler uses for POST /api/v1/mcp.
func (s *Server) Handle(ctx context.Context, tenantID uuid.UUID, req Request) Response {
	switch req.Method {
	case "initialize":
		return resultResponse(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]string{"name": "fleet-management-mcp", "version": "0.1.0"},
		})
	case "tools/list":
		return resultResponse(req.ID, map[string]any{"tools": s.Definitions()})
	case "tools/call":
		return s.handleToolsCall(ctx, tenantID, req)
	default:
		return errorResponse(req.ID, codeMethodNotFound, fmt.Sprintf("unknown method %q", req.Method))
	}
}

type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) handleToolsCall(ctx context.Context, tenantID uuid.UUID, req Request) Response {
	var params toolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, codeInvalidParams, "invalid tools/call params: "+err.Error())
	}

	result, err := s.Call(ctx, tenantID, params.Name, params.Arguments)
	if err != nil {
		return errorResponse(req.ID, codeInvalidParams, err.Error())
	}

	return resultResponse(req.ID, map[string]any{
		"content": []map[string]string{{"type": "text", "text": result}},
	})
}