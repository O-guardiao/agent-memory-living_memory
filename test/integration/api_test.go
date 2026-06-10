package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agent-memory/agent-memory/internal/bootstrap"
	"github.com/agent-memory/agent-memory/internal/config"
)

func TestIntegrationPlaceholder(t *testing.T) {
	t.Log("Add docker-backed integration tests for Postgres/Qdrant/Neo4j here.")
}

func doJSON(t *testing.T, app *bootstrap.App, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	app.HTTPServer.ServeHTTP(rec, req)
	var out map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec, out
}

// TestAPIEndToEnd exercises the full route surface on the in-memory
// stack: it always runs, no external services required.
func TestAPIEndToEnd(t *testing.T) {
	app := bootstrap.NewApp(config.Config{
		Env:                 "test",
		Port:                "0",
		DefaultTenant:       "tenant_it",
		EvalRecorderEnabled: true,
	})

	// Sync ingestion.
	rec, ingested := doJSON(t, app, http.MethodPost, "/v1/events", map[string]any{
		"tenant_id": "tenant_it", "user_id": "user_1", "agent_id": "agent_1",
		"session_id": "session_1", "role": "user",
		"content": "Meu email é pessoa@example.com e prefiro Go para o core.",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("ingest: %d %s", rec.Code, rec.Body.String())
	}
	memories := ingested["memories"].([]any)
	if len(memories) == 0 {
		t.Fatal("expected distilled memories")
	}
	memoryID := memories[0].(map[string]any)["id"].(string)

	// Search.
	rec, searched := doJSON(t, app, http.MethodPost, "/v1/memories/search", map[string]any{
		"tenant_id": "tenant_it", "user_id": "user_1", "query": "prefere Go core", "limit": 5,
	})
	if rec.Code != http.StatusOK || searched["trace_id"] == "" {
		t.Fatalf("search: %d %s", rec.Code, rec.Body.String())
	}
	traceID := searched["trace_id"].(string)

	// Context assembly with token budget and citations.
	rec, pack := doJSON(t, app, http.MethodPost, "/v1/context/assemble", map[string]any{
		"tenant_id": "tenant_it", "user_id": "user_1", "query": "prefere Go", "limit": 5, "token_budget": 1000,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("assemble: %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := pack["citations"]; !ok {
		t.Fatalf("expected citations in context pack: %v", pack)
	}

	// Trace fetch.
	rec, _ = doJSON(t, app, http.MethodGet, "/v1/traces/"+traceID+"?tenant_id=tenant_it", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("trace: %d %s", rec.Code, rec.Body.String())
	}

	// Memory list.
	rec, listed := doJSON(t, app, http.MethodGet, "/v1/memories?tenant_id=tenant_it&user_id=user_1", nil)
	if rec.Code != http.StatusOK || len(listed["memories"].([]any)) == 0 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}

	// Redaction (partial deletion).
	rec, redacted := doJSON(t, app, http.MethodPost, "/v1/memories/"+memoryID+"/redact?tenant_id=tenant_it", nil)
	if rec.Code != http.StatusOK || redacted["id"] != "redact_"+memoryID {
		t.Fatalf("redact: %d %s", rec.Code, rec.Body.String())
	}
	rec, fetched := doJSON(t, app, http.MethodGet, "/v1/memories/"+memoryID+"?tenant_id=tenant_it", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get after redact: %d", rec.Code)
	}
	if content := fetched["content"].(string); strings.Contains(content, "pessoa@example.com") {
		t.Fatalf("redaction left PII: %q", content)
	}

	// Agentic control plane.
	rec, _ = doJSON(t, app, http.MethodGet, "/v1/agentic/skills", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("skills: %d", rec.Code)
	}
	rec, _ = doJSON(t, app, http.MethodPost, "/v1/agentic/decide", map[string]any{
		"tenant_id": "tenant_it", "user_id": "user_1", "task": "como prosseguir?",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("decide: %d %s", rec.Code, rec.Body.String())
	}

	// Evals export (recorder enabled).
	rec, _ = doJSON(t, app, http.MethodGet, "/v1/evals/export", nil)
	if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
		t.Fatalf("evals export: %d", rec.Code)
	}

	// Deletion with receipt.
	rec, receipt := doJSON(t, app, http.MethodDelete, "/v1/memories/"+memoryID+"?tenant_id=tenant_it&reason=test", nil)
	if rec.Code != http.StatusOK || receipt["id"] != "del_"+memoryID {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}

	// Metrics endpoint.
	rec, _ = doJSON(t, app, http.MethodGet, "/metrics", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "memory_retrieval_latency_ms") {
		t.Fatalf("metrics: %d", rec.Code)
	}
}

// TestAPIAsyncIngestionViaQueue covers the async path with the worker.
func TestAPIAsyncIngestionViaQueue(t *testing.T) {
	app := bootstrap.NewApp(config.Config{
		Env: "test", Port: "0", DefaultTenant: "tenant_it", IngestionMode: "async",
	})
	if err := app.Worker.StartWorker(httptest.NewRequest(http.MethodGet, "/", nil).Context()); err != nil {
		t.Fatal(err)
	}
	rec, _ := doJSON(t, app, http.MethodPost, "/v1/events", map[string]any{
		"user_id": "user_1", "role": "user", "content": "evento assíncrono para a fila",
	})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("async ingest: %d %s", rec.Code, rec.Body.String())
	}
	rec, _ = doJSON(t, app, http.MethodPost, "/v1/memories/search", map[string]any{
		"tenant_id": "tenant_it", "user_id": "user_1", "query": "evento assíncrono", "limit": 5,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("search after async: %d", rec.Code)
	}
}
