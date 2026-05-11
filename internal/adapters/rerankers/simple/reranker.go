package simplerank

import (
	"context"
	"strings"

	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

type Reranker struct{}

func New() Reranker { return Reranker{} }

func (Reranker) Rerank(ctx context.Context, query string, candidates []retrieval.Candidate) ([]retrieval.Candidate, error) {
	_ = ctx
	qTokens := tokenSet(query)
	for i := range candidates {
		overlap := overlapScore(qTokens, tokenSet(candidates[i].Memory.Content))
		candidates[i].Score = 0.75*candidates[i].Score + 0.25*overlap
		if overlap > 0 {
			candidates[i].Reasons = append(candidates[i].Reasons, "lexical_overlap")
		}
	}
	retrieval.SortCandidates(candidates)
	return candidates, nil
}

func tokenSet(text string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, token := range strings.Fields(strings.ToLower(text)) {
		token = strings.Trim(token, " .,;:!?()[]{}\"'")
		if len(token) > 2 {
			out[token] = struct{}{}
		}
	}
	return out
}

func overlapScore(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	hits := 0
	for token := range a {
		if _, ok := b[token]; ok {
			hits++
		}
	}
	return float64(hits) / float64(len(a))
}
