package memstorage

import (
	"context"
	"sync"
	"time"
)

type cacheEntry struct {
	value     []byte
	expiresAt time.Time
}

// Cache is an in-process ports.Cache for development and tests.
type Cache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
}

func NewCache() *Cache {
	return &Cache{entries: make(map[string]cacheEntry)}
}

func (c *Cache) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil, false, nil
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false, nil
	}
	value := make([]byte, len(entry.value))
	copy(value, entry.value)
	return value, true, nil
}

func (c *Cache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := cacheEntry{value: make([]byte, len(value))}
	copy(entry.value, value)
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	c.entries[key] = entry
	return nil
}

func (c *Cache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
	return nil
}
