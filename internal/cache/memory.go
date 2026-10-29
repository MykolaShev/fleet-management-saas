package cache

import (
	"context"
	"strings"
	"sync"
	"time"
)

type entry struct {
	value     string
	expiresAt time.Time
}

// Memory is an in-process Cache — no extra infrastructure, lost on
// restart. Expiry is checked lazily on Get, not swept in the background;
type Memory struct {
	mu      sync.Mutex
	entries map[string]entry
}

func NewMemory() *Memory {
	return &Memory{entries: make(map[string]entry)}
}

func (m *Memory) Get(_ context.Context, key string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	e, ok := m.entries[key]
	if !ok || time.Now().After(e.expiresAt) {
		return "", false, nil
	}
	return e.value, true, nil
}

func (m *Memory) Set(_ context.Context, key, value string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[key] = entry{value: value, expiresAt: time.Now().Add(ttl)}
	return nil
}

func (m *Memory) DeleteByPrefix(_ context.Context, prefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key := range m.entries {
		if strings.HasPrefix(key, prefix) {
			delete(m.entries, key)
		}
	}
	return nil
}