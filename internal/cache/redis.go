package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis wraps a go-redis client. Not covered by go test — that would
// need a live Redis in CI; exercised via docker-compose + Postman
// instead (the X-Cache header in cache_middleware.go is the evidence).
type Redis struct {
	client *redis.Client
}

func NewRedis(addr, password string) *Redis {
	return &Redis{client: redis.NewClient(&redis.Options{Addr: addr, Password: password})}
}

// Ping fails fast at startup if Redis isn't reachable, instead of only
// discovering it on the first request.
func (r *Redis) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *Redis) Get(ctx context.Context, key string) (string, bool, error) {
	value, err := r.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (r *Redis) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

// DeleteByPrefix uses SCAN, not KEYS — KEYS blocks the whole Redis
// instance on a large keyspace, SCAN doesn't.
func (r *Redis) DeleteByPrefix(ctx context.Context, prefix string) error {
	var keys []string
	iter := r.client.Scan(ctx, 0, prefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	return r.client.Del(ctx, keys...).Err()
}