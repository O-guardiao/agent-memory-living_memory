package cohere

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

func TestRerankBlendsProviderScores(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/rerank" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"index": 1, "relevance_score": 1.0},
				{"index": 0, "relevance_score": 0.0},
			},
		})
	}))
	defer server.Close()

	reranker := NewWithEndpoint(server.URL, "key", "rerank-v3.5", server.Client())
	candidates := []retrieval.Candidate{
		{Memory: memory.Memory{ID: "a", Content: "first"}, Score: 0.6},
		{Memory: memory.Memory{ID: "b", Content: "second"}, Score: 0.4},
	}
	out, err := reranker.Rerank(context.Background(), "query", candidates)
	if err != nil {
		t.Fatalf("rerank: %v", err)
	}
	if out[0].Memory.ID != "b" {
		t.Fatalf("expected provider-favored candidate first, got %s", out[0].Memory.ID)
	}
	if out[0].Score != 0.5*0.4+0.5*1.0 {
		t.Fatalf("unexpected blended score: %f", out[0].Score)
	}
	docs := body["documents"].([]any)
	if len(docs) != 2 || docs[0] != "first" {
		t.Fatalf("unexpected documents: %#v", docs)
	}
}

func TestRerankPropagatesErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	reranker := NewWithEndpoint(server.URL, "bad", "m", server.Client())
	_, err := reranker.Rerank(context.Background(), "q", []retrieval.Candidate{{Memory: memory.Memory{Content: "x"}}})
	if err == nil {
		t.Fatal("expected error from provider failure")
	}
}
