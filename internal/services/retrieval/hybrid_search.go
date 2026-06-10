package retrievalsvc

// Hybrid search is implemented in service.go for the MVP. Split it here as the ranking logic grows.

import (
	"context"

	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
	"github.com/agent-memory/agent-memory/internal/ports"
)

func (s *Service) textSearch(ctx context.Context, q retrieval.Query, candidates map[string]retrieval.Candidate) error {
	textual, err := s.deps.Memories.SearchByText(ctx, q)
	if err != nil {
		return err
	}
	for _, mem := range textual {
		if !mem.IsActive(q.Now) {
			continue
		}
		candidates[mem.ID] = retrieval.Candidate{Memory: mem, Score: 0.55 + 0.20*mem.Importance, Reasons: []string{"text_search"}, Source: "text"}
	}
	return nil
}

func (s *Service) vectorSearch(ctx context.Context, q retrieval.Query, limit int, candidates map[string]retrieval.Candidate) error {
	vec, err := s.deps.Embedder.Embed(ctx, q.Text)
	if err != nil {
		return err
	}
	vres, err := s.deps.Vectors.Search(ctx, ports.VectorQuery{
		TenantID: q.TenantID,
		UserID:   q.UserID,
		Text:     q.Text,
		Vector:   vec,
		Limit:    limit * 3,
		Filters:  q.Filters,
	})
	if err != nil {
		return err
	}
	for _, result := range vres {
		mem, err := s.deps.Memories.Get(ctx, q.TenantID, result.ID)
		if err != nil || !mem.IsActive(q.Now) {
			continue
		}
		score := 0.65*result.Score + 0.20*mem.Importance + 0.15*mem.Confidence
		existing, ok := candidates[mem.ID]
		if ok {
			existing.Score = max(existing.Score, score)
			existing.Reasons = append(existing.Reasons, "vector_search")
			existing.Source = "hybrid"
			candidates[mem.ID] = existing
		} else {
			candidates[mem.ID] = retrieval.Candidate{Memory: mem, Score: score, Reasons: []string{"vector_search"}, Source: "vector"}
		}
	}
	return nil
}

// keywordSearch merges the optional KeywordStore channel (wired in memory
// mode; postgres SearchByText already covers lexical search there).
func (s *Service) keywordSearch(ctx context.Context, q retrieval.Query, candidates map[string]retrieval.Candidate) error {
	if s.deps.Keywords == nil {
		return nil
	}
	results, err := s.deps.Keywords.Search(ctx, q)
	if err != nil {
		return err
	}
	for _, result := range results {
		mem, err := s.deps.Memories.Get(ctx, q.TenantID, result.ID)
		if err != nil || !mem.IsActive(q.Now) {
			continue
		}
		score := 0.45*result.Score + 0.20*mem.Importance
		existing, ok := candidates[mem.ID]
		if ok {
			existing.Score = max(existing.Score, score)
			existing.Reasons = append(existing.Reasons, "keyword_search")
			existing.Source = "hybrid"
			candidates[mem.ID] = existing
		} else {
			candidates[mem.ID] = retrieval.Candidate{Memory: mem, Score: score, Reasons: []string{"keyword_search"}, Source: "keyword"}
		}
	}
	return nil
}
