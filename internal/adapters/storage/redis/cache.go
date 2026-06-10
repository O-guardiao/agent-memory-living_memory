package redis

// Placeholder for Redis/Valkey cache adapter.

import (
	"context"
	"strconv"
	"time"
)

// Cache implements ports.Cache with GET/SET PX/DEL.
type Cache struct {
	client *Client
}

func NewCache(client *Client) *Cache {
	return &Cache{client: client}
}

func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	reply, err := c.client.Do(ctx, "GET", key)
	if err != nil {
		return nil, false, err
	}
	if reply.Null {
		return nil, false, nil
	}
	return []byte(reply.Str), true, nil
}

func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	args := []string{"SET", key, string(value)}
	if ttl > 0 {
		args = append(args, "PX", strconv.FormatInt(ttl.Milliseconds(), 10))
	}
	_, err := c.client.Do(ctx, args...)
	return err
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	_, err := c.client.Do(ctx, "DEL", key)
	return err
}
