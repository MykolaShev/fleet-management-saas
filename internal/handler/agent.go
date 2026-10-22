package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/MykolaShev/fleet-management-saas/internal/service"
)

type AgentHandler struct {
	service *service.AgentService
}

func NewAgentHandler(s *service.AgentService) *AgentHandler {
	return &AgentHandler{service: s}
}

func (h *AgentHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/agent/chat", h.Chat)
}

// Chat streams the AI agent's response as Server-Sent Events. The message
// is a query parameter (?message=...), not a JSON body, because this
// endpoint is meant to be callable from a browser's native EventSource
// API, which only supports GET with no custom body — and, for the same
// reason, Authenticate accepts the token via ?token= here too, since
// EventSource can't set an Authorization header either (see
// middleware.go).
func (h *AgentHandler) Chat(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	message := c.Query("message")
	if message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message query parameter is required"})
		return
	}

	chunks, err := h.service.Chat(c.Request.Context(), tID, message)
	if err != nil {
		// Logged server-side only — the client gets a generic message so
		// we don't leak upstream provider details, but without this
		// you'd have no way to see *why* Groq rejected the request.
		log.Printf("agent chat error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "agent request failed"})
		return
	}

	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	c.Stream(func(w io.Writer) bool {
		chunk, open := <-chunks
		if !open {
			return false
		}
		switch {
		case chunk.Err != nil:
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", escapeSSE(chunk.Err.Error()))
			return false
		case chunk.Done:
			fmt.Fprint(w, "event: done\ndata: {}\n\n")
			return false
		default:
			fmt.Fprintf(w, "data: %s\n\n", escapeSSE(chunk.Content))
			return true
		}
	})
}

// escapeSSE replaces newlines in a content fragment, since the SSE format
// uses a bare newline to end a "data:" line — an unescaped newline inside
// the content would silently truncate the event.
func escapeSSE(s string) string {
	return strings.ReplaceAll(s, "\n", "\\n")
}
