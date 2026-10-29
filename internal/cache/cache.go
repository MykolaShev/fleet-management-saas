// Package cache is B9: a minimal key-value cache with two backends
// (memory, Redis) behind one interface, so the HTTP layer — and B8's
// queue later, which needs the same Redis connection — don't care which
// is active.
package cache

import (
	"context"
	"time"
)

// Cache is deliberately small: get/set-with-ttl/delete-by-prefix is all
// the HTTP response cache in internal/handler/cache_middleware.go needs.
type Cache interface {
	// Get returns (value, true, nil) on a hit, ("", false, nil) on a
	// clean miss (expired or never set) — only a real backend failure
	// returns a non-nil error.
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	// DeleteByPrefix removes every key starting with prefix — how cache
	// invalidation works here (per-tenant namespace, not per-resource).
	DeleteByPrefix(ctx context.Context, prefix string) error
}