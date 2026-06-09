package bootstrap

// Dependencies are composed in app.go. Keep this file for production wiring variants.

import (
	"github.com/agent-memory/agent-memory/internal/adapters/http/middleware"
	"github.com/agent-memory/agent-memory/internal/config"
)

// limiterFor selects the rate limiter: disabled unless MEMORY_RATE_LIMIT_RPS
// is set. The Redis-backed variant is chosen when MEMORY_REDIS_ADDR is set.
func limiterFor(cfg config.Config) middleware.RateLimiter {
	if cfg.RateLimitRPS <= 0 {
		return nil
	}
	return middleware.NewTokenBucket(cfg.RateLimitRPS, cfg.RateLimitBurst, nil)
}
