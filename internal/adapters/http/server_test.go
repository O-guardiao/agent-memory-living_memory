package httpadapter_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agent-memory/agent-memory/internal/bootstrap"
	"github.com/agent-memory/agent-memory/internal/config"
)

func TestIngestAndAssembleContext(t *testing.T) {
	app := bootstrap.NewApp(config.Config{Env: "test", Port: "0", DefaultTenant: "tenant_test"})

	eventBody := []byte(`{"tenant_id":"tenant_test","user_id":"user_1","agent_id":"agent_1","session_id":"session_1","role":"user","content":"Eu prefiro Go para o core de produção e Python para evals."}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(eventBody))
	req.Header.Set("Content-Type", "application/json")
	app.HTTPServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	searchBody := []byte(`{"tenant_id":"tenant_test","user_id":"user_1","query":"Qual linguagem prefere para core?","limit":5}`)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/context/assemble", bytes.NewReader(searchBody))
	req.Header.Set("Content-Type", "application/json")
	app.HTTPServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["trace_id"] == "" {
		t.Fatalf("expected trace_id, got %#v", out)
	}
}

func TestAsyncIngestReturnsAcceptedAndUsesWorkerQueue(t *testing.T) {
	app := bootstrap.NewApp(config.Config{
		Env:           "test",
		Port:          "0",
		DefaultTenant: "tenant_test",
		IngestionMode: "async",
	})
	if err := app.Worker.StartWorker(httptest.NewRequest(http.MethodGet, "/", nil).Context()); err != nil {
		t.Fatalf("start worker: %v", err)
	}

	eventBody := []byte(`{"user_id":"user_1","agent_id":"agent_1","session_id":"session_1","role":"user","content":"Async ingestion deve passar pela fila antes de indexar."}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(eventBody))
	req.Header.Set("Content-Type", "application/json")
	app.HTTPServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	var accepted map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	if accepted["status"] != "accepted" || accepted["tenant_id"] != "tenant_test" {
		t.Fatalf("unexpected accepted response: %#v", accepted)
	}

	searchBody := []byte(`{"tenant_id":"tenant_test","user_id":"user_1","query":"fila indexar","limit":5}`)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/context/assemble", bytes.NewReader(searchBody))
	req.Header.Set("Content-Type", "application/json")
	app.HTTPServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 after async worker, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAgenticControlPlaneEndpoints(t *testing.T) {
	app := bootstrap.NewApp(config.Config{Env: "test", Port: "0", DefaultTenant: "tenant_test"})

	decideBody := []byte(`{"tenant_id":"tenant_test","user_id":"user_1","task":"Adicionar camada inspirada em superpowers e spec-driven development com links atuais e entregar zip","token_budget":800}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/agentic/decide", bytes.NewReader(decideBody))
	req.Header.Set("Content-Type", "application/json")
	app.HTTPServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var advice map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &advice); err != nil {
		t.Fatal(err)
	}
	if advice["route"] == "" {
		t.Fatalf("expected route, got %#v", advice)
	}

	specBody := []byte(`{"tenant_id":"tenant_test","user_id":"user_1","project_id":"proj_1","title":"Control plane","goal":"Keep agent execution grounded with compact specs and rails.","acceptance_criteria":["context is bounded","plan has verification"]}`)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/agentic/specs", bytes.NewReader(specBody))
	req.Header.Set("Content-Type", "application/json")
	app.HTTPServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var spec map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	specID, _ := spec["id"].(string)
	if specID == "" {
		t.Fatalf("expected spec id, got %#v", spec)
	}

	planBody := []byte(`{"tenant_id":"tenant_test","user_id":"user_1","project_id":"proj_1","spec_id":"` + specID + `","title":"Plan","objective":"Implement bounded control context.","steps":[{"title":"Add service","description":"Create agentic service","verification":["go test ./internal/services/agentic"]}]}`)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/agentic/plans", bytes.NewReader(planBody))
	req.Header.Set("Content-Type", "application/json")
	app.HTTPServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var plan map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	planID, _ := plan["id"].(string)
	if planID == "" {
		t.Fatalf("expected plan id, got %#v", plan)
	}

	controlBody := []byte(`{"tenant_id":"tenant_test","user_id":"user_1","project_id":"proj_1","query":"implementar","spec_id":"` + specID + `","plan_id":"` + planID + `"}`)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/agentic/context", bytes.NewReader(controlBody))
	req.Header.Set("Content-Type", "application/json")
	app.HTTPServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var control map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &control); err != nil {
		t.Fatal(err)
	}
	if control["active_spec"] == nil || control["active_plan"] == nil {
		t.Fatalf("expected control context with active spec and plan, got %#v", control)
	}
}
