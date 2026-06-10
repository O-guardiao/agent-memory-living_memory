package integration

import (
	"context"
	"os"
	"testing"
	"time"

	neo4jstore "github.com/agent-memory/agent-memory/internal/adapters/storage/neo4j"
	"github.com/agent-memory/agent-memory/internal/ports"
)

func TestNeo4jPlaceholder(t *testing.T) { t.Skip("requires Neo4j") }

// TestNeo4jGraphStore exercises node/edge upsert and traversal.
// Gate: MEMORY_TEST_NEO4J_ENDPOINT (e.g. http://localhost:7474).
func TestNeo4jGraphStore(t *testing.T) {
	endpoint := os.Getenv("MEMORY_TEST_NEO4J_ENDPOINT")
	if endpoint == "" {
		t.Skip("set MEMORY_TEST_NEO4J_ENDPOINT to run this integration test")
	}
	reachableOrSkip(t, endpoint)

	store := neo4jstore.NewGraphStore(endpoint, os.Getenv("MEMORY_TEST_NEO4J_DATABASE"), os.Getenv("MEMORY_TEST_NEO4J_BASIC_AUTH"), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	payload := map[string]string{"tenant_id": "tenant_it", "type": "fact"}
	if err := store.UpsertNode(ctx, ports.GraphNode{ID: "mem_it_a", Labels: []string{"Memory"}, Payload: payload}); err != nil {
		t.Fatalf("upsert node a: %v", err)
	}
	if err := store.UpsertNode(ctx, ports.GraphNode{ID: "mem_it_b", Labels: []string{"Memory"}, Payload: payload}); err != nil {
		t.Fatalf("upsert node b: %v", err)
	}
	if err := store.UpsertEdge(ctx, ports.GraphEdge{FromID: "mem_it_a", ToID: "mem_it_b", Type: "related_to", Payload: payload}); err != nil {
		t.Fatalf("upsert edge: %v", err)
	}
	results, err := store.Traverse(ctx, ports.GraphQuery{TenantID: "tenant_it", StartID: "mem_it_a", Depth: 2})
	if err != nil {
		t.Fatalf("traverse: %v", err)
	}
	found := false
	for _, result := range results {
		if result.NodeID == "mem_it_b" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected mem_it_b in traversal, got %+v", results)
	}
}
