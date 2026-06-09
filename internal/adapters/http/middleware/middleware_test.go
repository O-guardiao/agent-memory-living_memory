package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func okHandler(t *testing.T, check func(r *http.Request)) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if check != nil {
			check(r)
		}
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequestIDGeneratesAndEchoes(t *testing.T) {
	var seen string
	h := RequestID(func() string { return "req_test" })(okHandler(t, func(r *http.Request) {
		seen = RequestIDFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if seen != "req_test" {
		t.Fatalf("expected generated request id in context, got %q", seen)
	}
	if got := rec.Header().Get("X-Request-ID"); got != "req_test" {
		t.Fatalf("expected echoed header, got %q", got)
	}
}

func TestRequestIDKeepsInbound(t *testing.T) {
	h := RequestID(nil)(okHandler(t, nil))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "req_inbound")
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Request-ID"); got != "req_inbound" {
		t.Fatalf("expected inbound id preserved, got %q", got)
	}
}

func TestParseAPIKeys(t *testing.T) {
	keys := ParseAPIKeys("alpha, beta:tenant_b ,,")
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if keys["alpha"] != "" || keys["beta"] != "tenant_b" {
		t.Fatalf("unexpected parse result: %#v", keys)
	}
}

func TestAuthDisabledPassesThrough(t *testing.T) {
	h := Auth(nil)(okHandler(t, nil))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with auth disabled, got %d", rec.Code)
	}
}

func TestAuthRejectsUnknownKey(t *testing.T) {
	h := Auth(map[string]string{"good": ""})(okHandler(t, nil))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without key, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "bad")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong key, got %d", rec.Code)
	}
}

func TestAuthAcceptsBearerAndBindsTenant(t *testing.T) {
	var bound string
	var ok bool
	h := Auth(map[string]string{"secret": "tenant_x"})(okHandler(t, func(r *http.Request) {
		bound, ok = BoundTenantFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !ok || bound != "tenant_x" {
		t.Fatalf("expected bound tenant tenant_x, got %q (ok=%v)", bound, ok)
	}
}

func TestTenantResolutionOrder(t *testing.T) {
	var resolved string
	inner := okHandler(t, func(r *http.Request) {
		resolved = TenantFromContext(r.Context())
	})

	h := Tenant("tenant_default")(inner)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if resolved != "tenant_default" {
		t.Fatalf("expected default tenant, got %q", resolved)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "tenant_h")
	h.ServeHTTP(rec, req)
	if resolved != "tenant_h" {
		t.Fatalf("expected header tenant, got %q", resolved)
	}

	chained := Auth(map[string]string{"k": "tenant_bound"})(Tenant("tenant_default")(inner))
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "k")
	chained.ServeHTTP(rec, req)
	if resolved != "tenant_bound" {
		t.Fatalf("expected key-bound tenant, got %q", resolved)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "k")
	req.Header.Set("X-Tenant-ID", "tenant_other")
	chained.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on tenant conflict, got %d", rec.Code)
	}
}

func TestTokenBucketRefills(t *testing.T) {
	now := time.Unix(0, 0)
	tb := NewTokenBucket(1, 1, func() time.Time { return now })

	allowed, err := tb.Allow(context.Background(), "k")
	if err != nil || !allowed {
		t.Fatalf("first request should pass: allowed=%v err=%v", allowed, err)
	}
	allowed, _ = tb.Allow(context.Background(), "k")
	if allowed {
		t.Fatal("second immediate request should be limited")
	}
	now = now.Add(time.Second)
	allowed, _ = tb.Allow(context.Background(), "k")
	if !allowed {
		t.Fatal("request after refill should pass")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	now := time.Unix(0, 0)
	tb := NewTokenBucket(1, 1, func() time.Time { return now })
	h := Tenant("tenant_default")(RateLimit(tb)(okHandler(t, nil)))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "1" {
		t.Fatalf("expected Retry-After header, got %q", rec.Header().Get("Retry-After"))
	}

	rec = httptest.NewRecorder()
	RateLimit(nil)(okHandler(t, nil)).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("nil limiter must pass through, got %d", rec.Code)
	}
}
