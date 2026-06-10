package retrievalsvc

// Add provider-specific reranking orchestration here.

import (
	"context"

	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

// rerank applies the configured reranker. Provider rerankers can fail on
// network/auth issues; retrieval then falls back to the pre-rerank order
// instead of failing the request. The default simple reranker never errors,
// so MVP behavior is unchanged.
func (s *Service) rerank(ctx context.Context, q retrieval.Query, ranked []retrieval.Candidate) []retrieval.Candidate {
	reranked, err := s.deps.Reranker.Rerank(ctx, q.Text, ranked)
	if err != nil {
		for i := range ranked {
			ranked[i].Reasons = append(ranked[i].Reasons, "rerank_fallback")
		}
		return ranked
	}
	return reranked
}
