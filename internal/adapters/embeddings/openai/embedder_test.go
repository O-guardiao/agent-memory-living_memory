package openaiembed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbedBatchPreservesOrder(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		// Return out of order to prove index-based reassembly.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"index": 1, "embedding": []float64{0.2}},
				{"index": 0, "embedding": []float64{0.1}},
			},
		})
	}))
	defer server.Close()

	embedder := New(server.URL, "sk", "text-embedding-3-small", 1, server.Client())
	vectors, err := embedder.EmbedBatch(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("embed batch: %v", err)
	}
	if vectors[0][0] != 0.1 || vectors[1][0] != 0.2 {
		t.Fatalf("order not preserved: %#v", vectors)
	}
	if body["dimensions"] != float64(1) {
		t.Fatalf("expected dimensions=1, got %#v", body["dimensions"])
	}
}

func TestEmbedBatchSizeMismatchFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer server.Close()

	embedder := New(server.URL, "sk", "m", 0, server.Client())
	if _, err := embedder.EmbedBatch(context.Background(), []string{"a"}); err == nil {
		t.Fatal("expected mismatch error")
	}
}
