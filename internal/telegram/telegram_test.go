package telegram_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MykolaShev/fleet-management-saas/internal/telegram"
)

func TestClient_SendMessage_Success(t *testing.T) {
	var capturedPath string
	var capturedBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedBody))
		fmt.Fprint(w, `{"ok":true,"result":{}}`)
	}))
	defer server.Close()

	client := telegram.NewClient("test-token", "12345")
	client.BaseURL = server.URL

	err := client.SendMessage(context.Background(), "hello")

	require.NoError(t, err)
	assert.Equal(t, "/bottest-token/sendMessage", capturedPath)
	assert.Equal(t, "12345", capturedBody["chat_id"])
	assert.Equal(t, "hello", capturedBody["text"])
}

func TestClient_SendMessage_APIReportsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ok":false,"description":"chat not found"}`)
	}))
	defer server.Close()

	client := telegram.NewClient("test-token", "wrong-chat")
	client.BaseURL = server.URL

	err := client.SendMessage(context.Background(), "hello")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "chat not found")
}

func TestClient_SendMessage_NetworkError(t *testing.T) {
	client := telegram.NewClient("test-token", "12345")
	client.BaseURL = "http://127.0.0.1:1" // nothing listens here

	err := client.SendMessage(context.Background(), "hello")

	assert.Error(t, err)
}