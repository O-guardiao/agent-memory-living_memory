package neo4j

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agent-memory/agent-memory/internal/ports"
)

func TestTraverseSendsTenantBoundCypher(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/db/neo4j/tx/commit" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"data": []map[string]any{
						{"row": []any{"mem_beta", 0.5}},
					},
				},
			},
			"errors": []any{},
		})
	}))
	defer server.Close()

	store := NewGraphStore(server.URL, "neo4j", "", server.Client())
	results, err := store.Traverse(context.Background(), ports.GraphQuery{
		TenantID: "tenant_a",
		StartID:  "mem_alpha",
		Depth:    2,
	})
	if err != nil {
		t.Fatalf("traverse: %v", err)
	}
	if len(results) != 1 || results[0].NodeID != "mem_beta" || results[0].Score != 0.5 {
		t.Fatalf("unexpected results: %#v", results)
	}

	statements := body["statements"].([]any)
	statement := statements[0].(map[string]any)
	cypher := statement["statement"].(string)
	if !strings.Contains(cypher, "tenant_id") || !strings.Contains(cypher, "[*1..2]") {
		t.Fatalf("cypher does not constrain tenant/depth: %s", cypher)
	}
	params := statement["parameters"].(map[string]any)
	if params["tenant_id"] != "tenant_a" || params["start_id"] != "mem_alpha" {
		t.Fatalf("unexpected params: %#v", params)
	}
}
