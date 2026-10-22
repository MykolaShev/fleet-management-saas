// Package llm provides a minimal, provider-agnostic streaming chat client.
// There's exactly one implementation right now (Groq, OpenAI-compatible),
// but keeping the interface separate from it means swapping providers
// later — or adding tool-calling for C22 — touches one file, not every
// caller.
package llm

import "context"

// Message is a single turn in a chat conversation. Role is "system",
// "user", or "assistant".
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamChunk is one piece of a streamed response.
type StreamChunk struct {
	Content string
	Done    bool
	Err     error
}

// Client streams a chat completion. The returned channel is closed by the
// implementation once the stream ends (cleanly or with an error) — callers
// range over it rather than polling.
type Client interface {
	StreamChat(ctx context.Context, messages []Message) (<-chan StreamChunk, error)
}