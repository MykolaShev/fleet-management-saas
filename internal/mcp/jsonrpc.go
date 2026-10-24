// Package mcp implements a minimal subset of the Model Context Protocol —
// just enough JSON-RPC 2.0 (initialize/tools/list/tools/call) for the AI
// agent (internal/service.AgentService) to discover and invoke a small,
// fixed set of fleet tools. 
//
// This package depends only on internal/domain, never internal/service.
package mcp

import "encoding/json"

// Request is one JSON-RPC 2.0 call.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is one JSON-RPC 2.0 reply — exactly one of Result/Error is set.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error is a JSON-RPC 2.0 error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Standard JSON-RPC 2.0 error codes — only the ones this server actually
// returns.
const (
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

func errorResponse(id json.RawMessage, code int, message string) Response {
	return Response{JSONRPC: "2.0", ID: id, Error: &Error{Code: code, Message: message}}
}

func resultResponse(id json.RawMessage, result any) Response {
	return Response{JSONRPC: "2.0", ID: id, Result: result}
}