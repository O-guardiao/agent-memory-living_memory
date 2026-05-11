package qdrant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agent-memory/agent-memory/internal/ports"
)

func TestSearchSendsTenantFiltersBeforeANN(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/collections/memory_points/points/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": []map[string]any{
				{
					"id":    "c79eb4f2-0e99-54be-9775-ecf12227ff04",
					"score": 0.91,
					"payload": map[string]any{
						"memory_id": "mem_alpha",
					},
				},
			},
		})
	}))
	defer server.Close()

	store := NewVectorStore(server.URL, "memory_points", server.Client())
	results, err := store.Search(context.Background(), ports.VectorQuery{
		TenantID: "tenant_a",
		UserID:   "user_1",
		Vector:   []float64{0.1, 0.2},
		Limit:    3,
		Filters:  map[string]string{"project_id": "psi", "type": "fact"},
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].ID != "mem_alpha" || results[0].Score != 0.91 {
		t.Fatalf("unexpected results: %#v", results)
	}

	filter := body["filter"].(map[string]any)
	must := filter["must"].([]any)
	want := map[string]bool{
		"tenant_id=tenant_a": false,
		"user_id=user_1":     false,
		"project_id=psi":     false,
		"type=fact":          false,
	}
	for _, raw := range must {
		item := raw.(map[string]any)
		match := item["match"].(map[string]any)
		key := item["key"].(string) + "=" + match["value"].(string)
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, seen := range want {
		if !seen {
			t.Fatalf("missing qdrant filter %s in %#v", key, must)
		}
	}
}

func TestEnsureCollectionCreatesMissingCollection(t *testing.T) {
	requests := []string{}
	var createBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodGet {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPut || r.URL.Path != "/collections/memory_points" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": true})
	}))
	defer server.Close()

	store := NewVectorStore(server.URL, "memory_points", server.Client())
	if err := store.EnsureCollection(context.Background(), 128); err != nil {
		t.Fatalf("ensure collection: %v", err)
	}
	if len(requests) != 2 || requests[0] != "GET /collections/memory_points" || requests[1] != "PUT /collections/memory_points" {
		t.Fatalf("unexpected request sequence: %#v", requests)
	}
	vectors := createBody["vectors"].(map[string]any)
	if vectors["size"].(float64) != 128 || vectors["distance"].(string) != "Cosine" {
		t.Fatalf("unexpected vectors config: %#v", vectors)
	}
}
