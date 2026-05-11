package embedding

import (
	"context"
	"testing"
	"time"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/ports"
)

func TestIndexMemoriesUsesBatchEmbeddingAndSingleVectorUpsert(t *testing.T) {
	embedder := &countingEmbedder{}
	vectors := &capturingVectorStore{}
	service := NewService(embedder, vectors)

	err := service.IndexMemories(context.Background(), []memory.Memory{
		{
			ID:        "mem_1",
			TenantID:  "tenant_a",
			UserID:    "user_1",
			ProjectID: "psi",
			Type:      memory.TypeFact,
			Scope:     memory.ScopeProject,
			Content:   "first memory",
			UpdatedAt: time.Now(),
		},
		{
			ID:        "mem_2",
			TenantID:  "tenant_a",
			UserID:    "user_1",
			ProjectID: "psi",
			Type:      memory.TypePreference,
			Scope:     memory.ScopeUser,
			Content:   "second memory",
			UpdatedAt: time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("index memories: %v", err)
	}
	if embedder.batchCalls != 1 || embedder.singleCalls != 0 {
		t.Fatalf("expected one batch call and no single calls, got batch=%d single=%d", embedder.batchCalls, embedder.singleCalls)
	}
	if len(vectors.items) != 2 {
		t.Fatalf("expected two vector items, got %d", len(vectors.items))
	}
	if vectors.items[0].Payload["tenant_id"] != "tenant_a" || vectors.items[0].Payload["project_id"] != "psi" {
		t.Fatalf("missing payload filters: %#v", vectors.items[0].Payload)
	}
}

type countingEmbedder struct {
	singleCalls int
	batchCalls  int
}

func (e *countingEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	_ = ctx
	_ = text
	e.singleCalls++
	return []float64{1, 0}, nil
}

func (e *countingEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	_ = ctx
	e.batchCalls++
	out := make([][]float64, 0, len(texts))
	for range texts {
		out = append(out, []float64{1, 0})
	}
	return out, nil
}

type capturingVectorStore struct {
	items []ports.VectorItem
}

func (s *capturingVectorStore) Upsert(ctx context.Context, items []ports.VectorItem) error {
	_ = ctx
	s.items = append(s.items, items...)
	return nil
}

func (s *capturingVectorStore) Search(context.Context, ports.VectorQuery) ([]ports.VectorResult, error) {
	return nil, nil
}

func (s *capturingVectorStore) Delete(context.Context, []string) error {
	return nil
}
