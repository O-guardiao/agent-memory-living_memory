package retrievalsvc

import (
	"context"
	"strings"
	"time"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
	"github.com/agent-memory/agent-memory/internal/ports"
	auditservice "github.com/agent-memory/agent-memory/internal/services/audit"
)

type Dependencies struct {
	Memories ports.MemoryStore
	Vectors  ports.VectorStore
	Graph    ports.GraphStore
	Embedder ports.Embedder
	Reranker ports.Reranker
	Audit    *auditservice.Service
	Clock    ports.Clock
}

type Service struct {
	deps Dependencies
}

func NewService(deps Dependencies) *Service { return &Service{deps: deps} }

func (s *Service) Retrieve(ctx context.Context, q retrieval.Query) ([]retrieval.Candidate, retrieval.Trace, error) {
	start := time.Now()
	if q.Now.IsZero() {
		q.Now = s.deps.Clock.Now()
	}
	limit := q.NormalizedLimit()

	candidates := map[string]retrieval.Candidate{}

	textual, err := s.deps.Memories.SearchByText(ctx, q)
	if err != nil {
		return nil, retrieval.Trace{}, err
	}
	for _, mem := range textual {
		if !mem.IsActive(q.Now) {
			continue
		}
		candidates[mem.ID] = retrieval.Candidate{Memory: mem, Score: 0.55 + 0.20*mem.Importance, Reasons: []string{"text_search"}, Source: "text"}
	}

	vec, err := s.deps.Embedder.Embed(ctx, q.Text)
	if err != nil {
		return nil, retrieval.Trace{}, err
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
		return nil, retrieval.Trace{}, err
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
	if s.deps.Graph != nil {
		seedIDs := make([]string, 0, len(candidates))
		for id := range candidates {
			seedIDs = append(seedIDs, id)
		}
		for _, seedID := range seedIDs {
			graphResults, err := s.deps.Graph.Traverse(ctx, ports.GraphQuery{
				TenantID: q.TenantID,
				StartID:  seedID,
				Depth:    2,
			})
			if err != nil {
				return nil, retrieval.Trace{}, err
			}
			for _, graphResult := range graphResults {
				mem, err := s.deps.Memories.Get(ctx, q.TenantID, graphResult.NodeID)
				if err != nil || !mem.IsActive(q.Now) {
					continue
				}
				score := 0.50 + 0.25*graphResult.Score + 0.15*mem.Importance + 0.10*mem.Confidence
				existing, ok := candidates[mem.ID]
				if ok {
					existing.Score = max(existing.Score, score)
					existing.Reasons = append(existing.Reasons, "graph_search")
					existing.Source = "hybrid_graph"
					candidates[mem.ID] = existing
				} else {
					candidates[mem.ID] = retrieval.Candidate{Memory: mem, Score: score, Reasons: []string{"graph_search"}, Source: "graph"}
				}
			}
		}
	}

	ranked := make([]retrieval.Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		candidate.Score += recencyBoost(candidate.Memory, q.Now)
		ranked = append(ranked, candidate)
	}
	retrieval.SortCandidates(ranked)
	ranked, err = s.deps.Reranker.Rerank(ctx, q.Text, ranked)
	if err != nil {
		return nil, retrieval.Trace{}, err
	}
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	trace := s.buildTrace(q, ranked, time.Since(start))
	if s.deps.Audit != nil {
		if err := s.deps.Audit.Save(ctx, trace); err != nil {
			return nil, retrieval.Trace{}, err
		}
	}
	return ranked, trace, nil
}

func (s *Service) buildTrace(q retrieval.Query, selected []retrieval.Candidate, latency time.Duration) retrieval.Trace {
	ids := make([]string, 0, len(selected))
	scores := map[string]float64{}
	tokenEstimate := 0
	for _, c := range selected {
		ids = append(ids, c.Memory.ID)
		scores[c.Memory.ID] = c.Score
		tokenEstimate += estimateTokens(c.Memory.Content)
	}
	traceID := ""
	if s.deps.Audit != nil {
		traceID = s.deps.Audit.NewTraceID()
	}
	return retrieval.Trace{
		ID:            traceID,
		Query:         q.Text,
		TenantID:      q.TenantID,
		UserID:        q.UserID,
		CandidateIDs:  ids,
		SelectedIDs:   ids,
		Scores:        scores,
		Filters:       q.Filters,
		LatencyMS:     latency.Milliseconds(),
		TokenEstimate: tokenEstimate,
		CreatedAt:     s.deps.Clock.Now(),
	}
}

func recencyBoost(mem memory.Memory, now time.Time) float64 {
	age := now.Sub(mem.UpdatedAt)
	if age < 24*time.Hour {
		return 0.05
	}
	if age < 7*24*time.Hour {
		return 0.025
	}
	return 0
}

func estimateTokens(text string) int {
	n := len(strings.Fields(text))
	if n == 0 {
		return 0
	}
	return int(float64(n) * 1.35)
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
