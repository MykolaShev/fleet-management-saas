package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GroqClient talks to Groq's OpenAI-compatible chat completions endpoint.
// No SDK dependency — the request/response shape is simple enough that a
// stdlib net/http client is clearer than pulling in a library for it.
type GroqClient struct {
	APIKey  string
	Model   string
	BaseURL string // defaults to Groq's real endpoint; overridable in tests
	HTTP    *http.Client
}

// NewGroqClient builds a client for the given API key and model.
func NewGroqClient(apiKey, model string) *GroqClient {
	return &GroqClient{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: "https://api.groq.com/openai/v1/chat/completions",
		HTTP:    &http.Client{},
	}
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Tools    []Tool    `json:"tools,omitempty"`
}

// chatStreamChunk matches the OpenAI-compatible SSE chunk shape. Only the
// fields we actually use are declared — everything else in Groq's
// response is silently ignored by json.Unmarshal.
type chatStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func (c *GroqClient) StreamChat(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	reqBody, err := json.Marshal(chatRequest{Model: c.Model, Messages: messages, Stream: true})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call groq: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("groq returned %d: %s", resp.StatusCode, string(body))
	}

	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		// SSE: each event is a line "data: <payload>", events separated
		// by a blank line. bufio.Scanner's default line-split handles
		// this fine — we just ignore every line that isn't a "data: "
		// line (blank separators, any other field Groq might add).
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")
			if payload == "[DONE]" {
				out <- StreamChunk{Done: true}
				return
			}

			var chunk chatStreamChunk
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				out <- StreamChunk{Err: fmt.Errorf("parse stream chunk: %w", err)}
				return
			}
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				out <- StreamChunk{Content: chunk.Choices[0].Delta.Content}
			}
		}
		if err := scanner.Err(); err != nil {
			out <- StreamChunk{Err: fmt.Errorf("read stream: %w", err)}
		}
	}()

	return out, nil
}

// chatCompletionResponse matches the OpenAI-compatible non-streaming
// completion shape. Only the fields Chat actually uses are declared.
type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

// Chat performs one non-streaming completion. With tools attached, the
// model may answer directly (Content set) or request a tool call
// (ToolCalls set) instead — never both, per the OpenAI-compatible
// contract this mirrors.
func (c *GroqClient) Chat(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error) {
	reqBody, err := json.Marshal(chatRequest{Model: c.Model, Messages: messages, Stream: false, Tools: tools})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call groq: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("groq returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("parse chat response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("groq returned no choices")
	}

	choice := parsed.Choices[0].Message
	return &ChatResponse{Content: choice.Content, ToolCalls: choice.ToolCalls}, nil
}