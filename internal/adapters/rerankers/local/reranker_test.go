package localrerank

import (
	"context"
	"testing"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

func TestBM25FavorsTermMatches(t *testing.T) {
	candidates := []retrieval.Candidate{
		{Memory: memory.Memory{ID: "off", Content: "completely unrelated text about cooking pasta"}, Score: 0.5},
		{Memory: memory.Memory{ID: "hit", Content: "the deployment pipeline uses kubernetes and helm charts"}, Score: 0.5},
	}
	out, err := New().Rerank(context.Background(), "kubernetes deployment pipeline", candidates)
	if err != nil {
		t.Fatalf("rerank: %v", err)
	}
	if out[0].Memory.ID != "hit" {
		t.Fatalf("expected term-matching candidate first, got %s", out[0].Memory.ID)
	}
	found := false
	for _, reason := range out[0].Reasons {
		if reason == "local_rerank" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected local_rerank reason, got %v", out[0].Reasons)
	}
}

func TestBM25EmptyQueryKeepsOrder(t *testing.T) {
	candidates := []retrieval.Candidate{
		{Memory: memory.Memory{ID: "a"}, Score: 0.9},
		{Memory: memory.Memory{ID: "b"}, Score: 0.1},
	}
	out, err := New().Rerank(context.Background(), "", candidates)
	if err != nil {
		t.Fatalf("rerank: %v", err)
	}
	if out[0].Memory.ID != "a" {
		t.Fatalf("expected original order, got %s first", out[0].Memory.ID)
	}
}
