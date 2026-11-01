package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client sends messages as one configured bot, to one configured chat.
// A multi-chat/multi-bot setup would need a different shape; this
// project only ever notifies one ops channel.
type Client struct {
	Token   string
	ChatID  string
	BaseURL string // defaults to the real API; overridable in tests
	HTTP    *http.Client
}

func NewClient(token, chatID string) *Client {
	return &Client{
		Token:   token,
		ChatID:  chatID,
		BaseURL: "https://api.telegram.org",
		HTTP:    &http.Client{},
	}
}

type sendMessageRequest struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

type apiResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

// SendMessage posts text to the configured chat via sendMessage.
func (c *Client) SendMessage(ctx context.Context, text string) error {
	reqBody, err := json.Marshal(sendMessageRequest{ChatID: c.ChatID, Text: text})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", c.BaseURL, c.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("call telegram: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("parse telegram response: %w", err)
	}
	if !parsed.OK {
		return fmt.Errorf("telegram API error: %s", parsed.Description)
	}
	return nil
}