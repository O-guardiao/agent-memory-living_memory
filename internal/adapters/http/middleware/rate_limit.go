package middleware

// Add tenant/user rate limiting here.

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimiter is implemented by in-memory and Redis-backed limiters.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// TokenBucket is an in-process per-key token bucket with lazy refill.
type TokenBucket struct {
	mu      sync.Mutex
	rps     int
	burst   int
	now     func() time.Time
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func NewTokenBucket(rps, burst int, now func() time.Time) *TokenBucket {
	if burst <= 0 {
		burst = 2 * rps
	}
	if now == nil {
		now = time.Now
	}
	return &TokenBucket{
		rps:     rps,
		burst:   burst,
		now:     now,
		buckets: make(map[string]*bucket),
	}
}

func (t *TokenBucket) Allow(_ context.Context, key string) (bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	b, ok := t.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(t.burst), last: now}
		t.buckets[key] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * float64(t.rps)
		if b.tokens > float64(t.burst) {
			b.tokens = float64(t.burst)
		}
		b.last = now
	}
	if b.tokens < 1 {
		return false, nil
	}
	b.tokens--
	return true, nil
}

// RateLimit enforces the limiter per tenant (falling back to the client
// host). A nil limiter keeps rate limiting disabled.
func RateLimit(limiter RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if limiter == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := TenantFromContext(r.Context())
			if key == "" {
				if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
					key = host
				} else {
					key = r.RemoteAddr
				}
			}
			allowed, err := limiter.Allow(r.Context(), key)
			if err != nil {
				// Fail open: a limiter backend outage must not take the API down.
				allowed = true
			}
			if !allowed {
				w.Header().Set("Retry-After", "1")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
