package retrievalsvc

import (
	"context"
	"testing"
	"time"

	hashembed "github.com/agent-memory/agent-memory/internal/adapters/embeddings/hash"
	memstorage "github.com/agent-memory/agent-memory/internal/adapters/storage/memory"
	"github.com/agent-memory/agent-memory/internal/adapters/system"
	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
	"github.com/agent-memory/agent-memory/internal/ports"
	"github.com/agent-memory/agent-memory/internal/services/embedding"
)

func TestRetrieveExpandsGraphProjectionCandidates(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	memories := memstorage.NewMemoryStore()
	vectors := memstorage.NewVectorStore()
	graph := memstorage.NewGraphStore()
	embedder := hashembed.New(16)
	embedSvc := embedding.NewService(embedder, vectors)

	seed := memory.NewMemory(now, memory.TypeFact, memory.ScopeProject, "MemoryOps uses vector search")
	seed.ID = "mem_seed"
	seed.TenantID = "tenant_a"
	seed.UserID = "user_1"
	seed.ProjectID = "psi"
	related := memory.NewMemory(now, memory.TypeProcedure, memory.ScopeProject, "Graph projection validates related operational policy")
	related.ID = "mem_related"
	related.TenantID = "tenant_a"
	related.UserID = "user_1"
	related.ProjectID = "psi"
	for _, mem := range []memory.Memory{seed, related} {
		if err := memories.Upsert(ctx, mem); err != nil {
			t.Fatalf("upsert memory: %v", err)
		}
	}
	if err := embedSvc.IndexMemories(ctx, []memory.Memory{seed}); err != nil {
		t.Fatalf("index seed: %v", err)
	}
	if err := graph.UpsertEdge(ctx, ports.GraphEdge{
		FromID: seed.ID,
		ToID:   related.ID,
		Type:   "supports",
		Payload: map[string]string{
			"tenant_id": "tenant_a",
		},
	}); err != nil {
		t.Fatalf("upsert edge: %v", err)
	}

	service := NewService(Dependencies{
		Memories: memories,
		Vectors:  vectors,
		Graph:    graph,
		Embedder: embedder,
		Reranker: passthroughReranker{},
		Clock:    system.RealClock{},
	})
	candidates, _, err := service.Retrieve(ctx, retrieval.Query{
		TenantID:  "tenant_a",
		UserID:    "user_1",
		ProjectID: "psi",
		Text:      "MemoryOps vector",
		Limit:     5,
		Now:       now,
	})
	if err != nil {
		t.Fatalf("retrieve: %v", err)
	}
	found := false
	for _, candidate := range candidates {
		if candidate.Memory.ID == related.ID {
			found = true
			if candidate.Source != "graph" {
				t.Fatalf("expected graph source, got %#v", candidate)
			}
		}
	}
	if !found {
		t.Fatalf("expected graph-related memory in candidates: %#v", candidates)
	}
}

type passthroughReranker struct{}

func (passthroughReranker) Rerank(ctx context.Context, query string, candidates []retrieval.Candidate) ([]retrieval.Candidate, error) {
	_ = ctx
	_ = query
	return candidates, nil
}
