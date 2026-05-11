package ports

import (
	"context"

	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

type Reranker interface {
	Rerank(ctx context.Context, query string, candidates []retrieval.Candidate) ([]retrieval.Candidate, error)
}
