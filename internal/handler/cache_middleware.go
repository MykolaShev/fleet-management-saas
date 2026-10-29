package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MykolaShev/fleet-management-saas/internal/cache"
)

const cacheKeyPrefix = "httpcache:"

// cacheRecorder captures a handler's response body so a GET can be
// stored verbatim, while still writing it through to the real client.
type cacheRecorder struct {
	gin.ResponseWriter
	body   bytes.Buffer
	status int
}

func (w *cacheRecorder) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *cacheRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// CacheMiddleware is B9: a GET is served from cache when present
// (X-Cache: HIT) or cached after the handler runs (X-Cache: MISS); any
// non-GET request that succeeds (2xx) wipes that tenant's entire cache
// namespace. Invalidation is whole-tenant, not per-resource, on purpose —
// simple to reason about and impossible to leave stale data behind by
// missing one resource's case; narrower invalidation is the natural next
// step if cache efficiency ever becomes the bottleneck. Must sit after
// Authenticate (needs tenantID already in context).
func CacheMiddleware(c cache.Cache, ttl time.Duration) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		tID, ok := tenantID(ctx)
		if !ok {
			return
		}

		if ctx.Request.Method != http.MethodGet {
			ctx.Next()
			if status := ctx.Writer.Status(); status >= 200 && status < 300 {
				_ = c.DeleteByPrefix(ctx.Request.Context(), cacheKeyPrefix+tID.String()+":")
			}
			return
		}

		key := fmt.Sprintf("%s%s:%s?%s", cacheKeyPrefix, tID, ctx.Request.URL.Path, ctx.Request.URL.RawQuery)

		if body, found, err := c.Get(ctx.Request.Context(), key); err == nil && found {
			ctx.Header("X-Cache", "HIT")
			ctx.Data(http.StatusOK, "application/json; charset=utf-8", []byte(body))
			ctx.Abort()
			return
		}

		ctx.Header("X-Cache", "MISS")
		rec := &cacheRecorder{ResponseWriter: ctx.Writer, status: http.StatusOK}
		ctx.Writer = rec
		ctx.Next()

		if rec.status == http.StatusOK {
			_ = c.Set(ctx.Request.Context(), key, rec.body.String(), ttl)
		}
	}
}