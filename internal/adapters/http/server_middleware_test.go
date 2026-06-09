package httpadapter_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agent-memory/agent-memory/internal/bootstrap"
	"github.com/agent-memory/agent-memory/internal/config"
)

func TestAuthEnabledRejectsAndAcceptsKeys(t *testing.T) {
	app := bootstrap.NewApp(config.Config{
		Env:           "test",
		Port:          "0",
		DefaultTenant: "tenant_test",
		APIKeys:       "open-key,scoped-key:tenant_scoped",
	})

	body := []byte(`{"user_id":"user_1","role":"user","content":"auth check"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))
	app.HTTPServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without key, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer open-key")
	app.HTTPServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 with valid key, got %d: %s", rec.Code, rec.Body.String())
	}

	// A tenant-scoped key may not act on another tenant supplied in the body.
	scopedBody := []byte(`{"tenant_id":"tenant_other","user_id":"user_1","role":"user","content":"cross tenant"}`)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(scopedBody))
	req.Header.Set("X-API-Key", "scoped-key")
	app.HTTPServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-tenant body, got %d: %s", rec.Code, rec.Body.String())
	}

	// Without an explicit tenant the key-bound tenant is used.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "scoped-key")
	app.HTTPServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for scoped key, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTenantHeaderResolution(t *testing.T) {
	app := bootstrap.NewApp(config.Config{Env: "test", Port: "0", DefaultTenant: "tenant_test"})

	body := []byte(`{"user_id":"user_1","role":"user","content":"tenant via header"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))
	req.Header.Set("X-Tenant-ID", "tenant_header")
	app.HTTPServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("tenant_header")) {
		t.Fatalf("expected response scoped to header tenant: %s", rec.Body.String())
	}
}

func TestRateLimitReturns429(t *testing.T) {
	app := bootstrap.NewApp(config.Config{
		Env:            "test",
		Port:           "0",
		DefaultTenant:  "tenant_test",
		RateLimitRPS:   1,
		RateLimitBurst: 1,
	})

	body := `{"tenant_id":"tenant_test","user_id":"user_1","query":"q","limit":1}`
	codes := make(map[int]int)
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/memories/search", bytes.NewReader([]byte(body)))
		app.HTTPServer.ServeHTTP(rec, req)
		codes[rec.Code]++
	}
	if codes[http.StatusTooManyRequests] == 0 {
		t.Fatalf("expected at least one 429, got %v", codes)
	}
	if codes[http.StatusOK] == 0 {
		t.Fatalf("expected at least one 200, got %v", codes)
	}
}

func TestRequestIDEchoedOnResponses(t *testing.T) {
	app := bootstrap.NewApp(config.Config{Env: "test", Port: "0", DefaultTenant: "tenant_test"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/search", bytes.NewReader([]byte(`{"query":"q"}`)))
	req.Header.Set("X-Request-ID", "req_fixed")
	app.HTTPServer.ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Request-ID"); got != "req_fixed" {
		t.Fatalf("expected request id echo, got %q", got)
	}
	if got := rec.Header().Get("X-Agent-Memory"); got != "mvp" {
		t.Fatalf("expected legacy mvp header preserved, got %q", got)
	}
}
