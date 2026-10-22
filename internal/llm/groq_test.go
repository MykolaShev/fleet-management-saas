package llm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MykolaShev/fleet-management-saas/internal/llm"
)

func TestGroqClient_StreamChat_ParsesSSEChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := llm.NewGroqClient("test-key", "test-model")
	client.BaseURL = server.URL

	chunks, err := client.StreamChat(context.Background(), []llm.Message{{Role: "user", Content: "hi"}})
	require.NoError(t, err)

	var received []llm.StreamChunk
	for c := range chunks {
		received = append(received, c)
	}

	require.Len(t, received, 3)
	assert.Equal(t, "Hello", received[0].Content)
	assert.Equal(t, " world", received[1].Content)
	assert.True(t, received[2].Done)
}

func TestGroqClient_StreamChat_IgnoresEmptyDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := llm.NewGroqClient("test-key", "test-model")
	client.BaseURL = server.URL

	chunks, err := client.StreamChat(context.Background(), []llm.Message{{Role: "user", Content: "hi"}})
	require.NoError(t, err)

	var received []llm.StreamChunk
	for c := range chunks {
		received = append(received, c)
	}

	// The empty-delta chunk produces no StreamChunk at all — only the
	// real content and the final Done arrive.
	require.Len(t, received, 2)
	assert.Equal(t, "ok", received[0].Content)
	assert.True(t, received[1].Done)
}

func TestGroqClient_StreamChat_NonOKStatus_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"invalid api key"}`)
	}))
	defer server.Close()

	client := llm.NewGroqClient("bad-key", "test-model")
	client.BaseURL = server.URL

	_, err := client.StreamChat(context.Background(), []llm.Message{{Role: "user", Content: "hi"}})

	assert.Error(t, err)
}

func TestGroqClient_StreamChat_SendsCorrectRequestShape(t *testing.T) {
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := llm.NewGroqClient("test-key", "llama-3.3-70b-versatile")
	client.BaseURL = server.URL

	chunks, err := client.StreamChat(context.Background(), []llm.Message{{Role: "user", Content: "hi"}})
	require.NoError(t, err)
	for range chunks {
	}

	assert.Equal(t, "llama-3.3-70b-versatile", capturedBody["model"])
	assert.Equal(t, true, capturedBody["stream"])
}

func TestGroqClient_Chat_NoToolCall_ReturnsContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"content":"3 vehicles are idle."}}]}`)
	}))
	defer server.Close()

	client := llm.NewGroqClient("test-key", "test-model")
	client.BaseURL = server.URL

	resp, err := client.Chat(context.Background(), []llm.Message{{Role: "user", Content: "how many idle?"}}, nil)

	require.NoError(t, err)
	assert.Equal(t, "3 vehicles are idle.", resp.Content)
	assert.Empty(t, resp.ToolCalls)
}

func TestGroqClient_Chat_ToolCallRequested_ReturnsToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_vehicle_location","arguments":"{\"vehicle_id\":\"abc\"}"}}]}}]}`)
	}))
	defer server.Close()

	client := llm.NewGroqClient("test-key", "test-model")
	client.BaseURL = server.URL

	resp, err := client.Chat(context.Background(), []llm.Message{{Role: "user", Content: "where is it?"}},
		[]llm.Tool{{Type: "function", Function: llm.ToolFunction{Name: "get_vehicle_location"}}})

	require.NoError(t, err)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "get_vehicle_location", resp.ToolCalls[0].Function.Name)
	assert.Equal(t, `{"vehicle_id":"abc"}`, resp.ToolCalls[0].Function.Arguments)
}

func TestGroqClient_Chat_SendsToolsAndNonStreamingFlag(t *testing.T) {
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer server.Close()

	client := llm.NewGroqClient("test-key", "test-model")
	client.BaseURL = server.URL

	tools := []llm.Tool{{Type: "function", Function: llm.ToolFunction{Name: "get_vehicle_location", Parameters: map[string]any{"type": "object"}}}}
	_, err := client.Chat(context.Background(), []llm.Message{{Role: "user", Content: "hi"}}, tools)
	require.NoError(t, err)

	assert.Equal(t, false, capturedBody["stream"])
	capturedTools, ok := capturedBody["tools"].([]any)
	require.True(t, ok)
	require.Len(t, capturedTools, 1)
}

func TestGroqClient_Chat_NonOKStatus_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"invalid api key"}`)
	}))
	defer server.Close()

	client := llm.NewGroqClient("bad-key", "test-model")
	client.BaseURL = server.URL

	_, err := client.Chat(context.Background(), []llm.Message{{Role: "user", Content: "hi"}}, nil)

	assert.Error(t, err)
}