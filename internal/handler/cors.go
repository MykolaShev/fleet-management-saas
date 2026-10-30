package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS allows the F5 SPA (served from a different origin during dev —
// Vite's default is http://localhost:5173) to call this API with a
// Bearer token. allowedOrigins is an explicit allow-list, never "*" —
// browsers reject "*" anyway once a request carries a custom header
// like Authorization.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}