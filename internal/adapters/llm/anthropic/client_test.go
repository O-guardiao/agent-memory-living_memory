package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agent-memory/agent-memory/internal/ports"
)

func TestGenerateSendsMessagesRequest(t *testing.T) {
	var body map[string]any
	var gotVersion, gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotVersion = r.Header.Get("anthropic-version")
		gotKey = r.Header.Get("x-api-key")
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content":     []map[string]any{{"type": "text", "text": "hello "}, {"type": "text", "text": "world"}},
			"stop_reason": "end_turn",
		})
	}))
	defer server.Close()

	client := New(server.URL, "test-key", "claude-opus-4-8", server.Client())
	resp, err := client.Generate(context.Background(), ports.GenerateRequest{System: "sys", Prompt: "hi"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if resp.Text != "hello world" {
		t.Fatalf("unexpected text: %q", resp.Text)
	}
	if gotVersion != apiVersion || gotKey != "test-key" {
		t.Fatalf("missing headers: version=%q key=%q", gotVersion, gotKey)
	}
	if body["model"] != "claude-opus-4-8" || body["system"] != "sys" {
		t.Fatalf("unexpected body: %#v", body)
	}
	if _, hasTemp := body["temperature"]; hasTemp {
		t.Fatal("temperature must not be sent")
	}
}

func TestGenerateJSONStripsFences(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{"type": "text", "text": "```json\n{\"ok\":true}\n```"}},
		})
	}))
	defer server.Close()

	client := New(server.URL, "k", "m", server.Client())
	var out struct {
		OK bool `json:"ok"`
	}
	if err := client.GenerateJSON(context.Background(), ports.GenerateRequest{Prompt: "p"}, &out); err != nil {
		t.Fatalf("generate json: %v", err)
	}
	if !out.OK {
		t.Fatal("expected parsed JSON")
	}
}

func TestGenerateSurfacesAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer server.Close()

	client := New(server.URL, "k", "m", server.Client())
	if _, err := client.Generate(context.Background(), ports.GenerateRequest{Prompt: "p"}); err == nil {
		t.Fatal("expected error for 400 response")
	}
}
