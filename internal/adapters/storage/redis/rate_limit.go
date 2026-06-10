package redis

// Placeholder for Redis/Valkey rate limit adapter.

import (
	"context"
	"fmt"
	"time"
)

// RateLimiter implements middleware.RateLimiter with a fixed one-second
// window per key: INCR + PEXPIRE NX, allowing up to rps+burst in a window.
type RateLimiter struct {
	client *Client
	rps    int
	burst  int
}

func NewRateLimiter(client *Client, rps, burst int) *RateLimiter {
	if burst <= 0 {
		burst = rps
	}
	return &RateLimiter{client: client, rps: rps, burst: burst}
}

func (l *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	window := time.Now().Unix()
	bucket := fmt.Sprintf("ratelimit:%s:%d", key, window)
	reply, err := l.client.Do(ctx, "INCR", bucket)
	if err != nil {
		return false, err
	}
	if reply.Int == 1 {
		if _, err := l.client.Do(ctx, "PEXPIRE", bucket, "1000"); err != nil {
			return false, err
		}
	}
	return reply.Int <= int64(l.rps+l.burst), nil
}
