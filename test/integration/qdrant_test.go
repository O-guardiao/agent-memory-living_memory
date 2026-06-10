package integration

import (
	"context"
	"os"
	"testing"
	"time"

	qdrantstore "github.com/agent-memory/agent-memory/internal/adapters/storage/qdrant"
	"github.com/agent-memory/agent-memory/internal/ports"
)

func TestQdrantPlaceholder(t *testing.T) { t.Skip("requires Qdrant") }

// TestQdrantVectorStore exercises collection/upsert/search/delete.
// Gate: MEMORY_TEST_QDRANT_ENDPOINT (e.g. http://localhost:6333).
func TestQdrantVectorStore(t *testing.T) {
	endpoint := os.Getenv("MEMORY_TEST_QDRANT_ENDPOINT")
	if endpoint == "" {
		t.Skip("set MEMORY_TEST_QDRANT_ENDPOINT to run this integration test")
	}
	reachableOrSkip(t, endpoint)

	store := qdrantstore.NewVectorStore(endpoint, "agent_memory_it", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := store.EnsureCollection(ctx, 8); err != nil {
		t.Fatalf("ensure collection: %v", err)
	}
	vector := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8}
	err := store.Upsert(ctx, []ports.VectorItem{{
		ID:     "mem_it_q",
		Vector: vector,
		Payload: map[string]string{
			"tenant_id": "tenant_it",
			"user_id":   "user_it",
			"type":      "fact",
		},
	}})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	results, err := store.Search(ctx, ports.VectorQuery{
		TenantID: "tenant_it",
		UserID:   "user_it",
		Vector:   vector,
		Limit:    3,
		Filters:  map[string]string{"type": "fact"},
	})
	if err != nil || len(results) == 0 || results[0].ID != "mem_it_q" {
		t.Fatalf("search: %+v %v", results, err)
	}
	if err := store.Delete(ctx, []string{"mem_it_q"}); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
