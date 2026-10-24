// Package llm provides a minimal, provider-agnostic chat client. There's
// exactly one implementation right now (Groq, OpenAI-compatible), but
// keeping the interface separate from it means swapping providers later —
// or changing how tool-calling works — touches one file, not every
// caller.
package llm

import "context"

// Message is a single turn in a chat conversation. Role is "system",
// "user", "assistant", or "tool".
//
// ToolCalls is set on an "assistant" message that requested one or more
// tool calls (echoed back verbatim in the follow-up request, per the
// OpenAI-compatible wire format). ToolCallID is set on the "tool" message
// that answers one specific call, and must match that call's ID.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// Tool describes one function the model may call, in the OpenAI-
// compatible "tools" request shape. service.AgentService builds these
// directly from mcp.ToolDefinition — see toLLMTools in agent_service.go.
type Tool struct {
	Type     string       `json:"type"` // always "function"
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolCall is one function call the model requested (OpenAI-compatible
// wire shape). Function.Arguments is a JSON-encoded string, not a nested
// object — that's how OpenAI-compatible APIs send it, and callers must
// json.Unmarshal it themselves.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // always "function"
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatResponse is the result of a single non-streaming Chat call: either
// plain text content, or one or more tool calls the caller must execute
// and feed back as "tool"-role messages before calling Chat/StreamChat
// again.
type ChatResponse struct {
	Content   string
	ToolCalls []ToolCall
}

// StreamChunk is one piece of a streamed response. A chunk carries at
// most one of {Content, ToolCall, ToolResult} — the fields it doesn't use
// stay at their zero value.
//
// ToolCall/ToolResult exist so a caller watching the raw SSE output (see
// handler.AgentHandler.Chat) can see which tool fired and what it
// returned, not just the model's final prose — this project has no
// frontend yet, so the raw stream itself is how C21's "show intermediate
// results / tool calling" requirement gets demonstrated for now.
type StreamChunk struct {
	Content    string
	ToolCall   *ToolCallEvent
	ToolResult *ToolResultEvent
	Done       bool
	Err        error
}

// ToolCallEvent is emitted once, right before a tool is actually invoked.
type ToolCallEvent struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolResultEvent is emitted once the tool call above has returned.
type ToolResultEvent struct {
	Name   string `json:"name"`
	Result string `json:"result"`
}

// Client is a chat completion provider.
type Client interface {
	// StreamChat streams a chat completion with no tools attached. The
	// returned channel is closed by the implementation once the stream
	// ends (cleanly or with an error) — callers range over it rather
	// than polling.
	StreamChat(ctx context.Context, messages []Message) (<-chan StreamChunk, error)

	// Chat is a non-streaming completion used for the agent's "decide"
	// step (see service.AgentService.Chat): passing tools lets the model
	// request a function call instead of answering directly.
	Chat(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error)
}