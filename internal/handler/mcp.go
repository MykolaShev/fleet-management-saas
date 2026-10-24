package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/MykolaShev/fleet-management-saas/internal/mcp"
)

// MCPHandler exposes the fleet MCP server (internal/mcp) over HTTP as a
// single JSON-RPC 2.0 endpoint. It sits behind the same Authenticate
// middleware as every other route, and tenant id is always taken from the
// authenticated context — never trusted from the JSON-RPC body — which is
// what makes this endpoint independently testable/demoable via
// Postman/curl, separate from whether the LLM agent calls it correctly.
type MCPHandler struct {
	server *mcp.Server
}

func NewMCPHandler(server *mcp.Server) *MCPHandler {
	return &MCPHandler{server: server}
}

func (h *MCPHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/mcp", h.Handle)
}

func (h *MCPHandler) Handle(c *gin.Context) {
	tID, ok := tenantID(c)
	if !ok {
		return
	}

	var req mcp.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON-RPC request: " + err.Error()})
		return
	}

	resp := h.server.Handle(c.Request.Context(), tID, req)
	c.JSON(http.StatusOK, resp)
}