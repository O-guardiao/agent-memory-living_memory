package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agent-memory/agent-memory/internal/ports"
)

func TestGenerateSendsChatRequest(t *testing.T) {
	var body map[string]any
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		auth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": "answer"}}},
		})
	}))
	defer server.Close()

	client := New(server.URL, "sk-test", "gpt-4o-mini", server.Client())
	resp, err := client.Generate(context.Background(), ports.GenerateRequest{System: "sys", Prompt: "q"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if resp.Text != "answer" {
		t.Fatalf("unexpected text: %q", resp.Text)
	}
	if auth != "Bearer sk-test" {
		t.Fatalf("missing bearer header: %q", auth)
	}
	messages := body["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("expected system+user messages, got %d", len(messages))
	}
}

func TestGenerateJSONRequestsJSONObjectFormat(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": `{"n":3}`}}},
		})
	}))
	defer server.Close()

	client := New(server.URL, "", "m", server.Client())
	var out struct {
		N int `json:"n"`
	}
	if err := client.GenerateJSON(context.Background(), ports.GenerateRequest{Prompt: "p"}, &out); err != nil {
		t.Fatalf("generate json: %v", err)
	}
	if out.N != 3 {
		t.Fatalf("unexpected output: %+v", out)
	}
	format := body["response_format"].(map[string]any)
	if format["type"] != "json_object" {
		t.Fatalf("expected json_object format, got %#v", format)
	}
}

func TestEndpointAcceptsTrailingV1(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": "ok"}}},
		})
	}))
	defer server.Close()

	client := New(server.URL+"/v1", "", "m", server.Client())
	if _, err := client.Generate(context.Background(), ports.GenerateRequest{Prompt: "p"}); err != nil {
		t.Fatalf("generate: %v", err)
	}
}
