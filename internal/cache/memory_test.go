package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MykolaShev/fleet-management-saas/internal/cache"
)

func TestMemory_SetThenGet_Hit(t *testing.T) {
	c := cache.NewMemory()
	require.NoError(t, c.Set(context.Background(), "k", "v", time.Minute))

	value, found, err := c.Get(context.Background(), "k")

	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "v", value)
}

func TestMemory_Get_NeverSet_Miss(t *testing.T) {
	c := cache.NewMemory()

	_, found, err := c.Get(context.Background(), "nope")

	require.NoError(t, err)
	assert.False(t, found)
}

func TestMemory_Get_Expired_Miss(t *testing.T) {
	c := cache.NewMemory()
	require.NoError(t, c.Set(context.Background(), "k", "v", time.Millisecond))
	time.Sleep(5 * time.Millisecond)

	_, found, err := c.Get(context.Background(), "k")

	require.NoError(t, err)
	assert.False(t, found)
}

func TestMemory_DeleteByPrefix_RemovesOnlyMatching(t *testing.T) {
	c := cache.NewMemory()
	ctx := context.Background()
	require.NoError(t, c.Set(ctx, "httpcache:tenantA:/vehicles", "a", time.Minute))
	require.NoError(t, c.Set(ctx, "httpcache:tenantA:/deliveries", "b", time.Minute))
	require.NoError(t, c.Set(ctx, "httpcache:tenantB:/vehicles", "c", time.Minute))

	require.NoError(t, c.DeleteByPrefix(ctx, "httpcache:tenantA:"))

	_, foundA1, _ := c.Get(ctx, "httpcache:tenantA:/vehicles")
	_, foundA2, _ := c.Get(ctx, "httpcache:tenantA:/deliveries")
	_, foundB, _ := c.Get(ctx, "httpcache:tenantB:/vehicles")
	assert.False(t, foundA1)
	assert.False(t, foundA2)
	assert.True(t, foundB, "a prefix delete for tenant A must not touch tenant B's entries")
}