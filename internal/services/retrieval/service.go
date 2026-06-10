package retrievalsvc

import (
	"context"
	"strings"
	"time"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
	"github.com/agent-memory/agent-memory/internal/ports"
	auditservice "github.com/agent-memory/agent-memory/internal/services/audit"
	"github.com/agent-memory/agent-memory/internal/services/evaluation"
)

type Dependencies struct {
	Memories ports.MemoryStore
	Vectors  ports.VectorStore
	Graph    ports.GraphStore
	Embedder ports.Embedder
	Reranker ports.Reranker
	Audit    *auditservice.Service
	Clock    ports.Clock
	// Keywords is an optional lexical channel (wired in memory mode).
	Keywords ports.KeywordStore
	// Recorder, when set, captures retrievals for offline replay.
	Recorder *evaluation.Recorder
	// PrivacyGates enables credential/PII filtering of results.
	PrivacyGates bool
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
	q = ApplyScopeFilters(q)
	limit := q.NormalizedLimit()
	plan := Plan(q, s.deps.Graph != nil, s.deps.Keywords != nil)

	candidates := map[string]retrieval.Candidate{}
	if plan.UseText {
		if err := s.textSearch(ctx, q, candidates); err != nil {
			return nil, retrieval.Trace{}, err
		}
	}
	if plan.UseVector {
		if err := s.vectorSearch(ctx, q, limit, candidates); err != nil {
			return nil, retrieval.Trace{}, err
		}
	}
	if plan.UseKeyword {
		if err := s.keywordSearch(ctx, q, candidates); err != nil {
			return nil, retrieval.Trace{}, err
		}
	}
	if plan.UseGraph {
		if err := s.graphExpand(ctx, q, plan.GraphDepth, candidates); err != nil {
			return nil, retrieval.Trace{}, err
		}
	}

	ranked := make([]retrieval.Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !PassesScope(candidate.Memory, q) {
			continue
		}
		if s.deps.PrivacyGates && !PassesPrivacy(candidate.Memory, q) {
			continue
		}
		candidate.Score += recencyBoost(candidate.Memory, q.Now)
		ranked = append(ranked, candidate)
	}
	retrieval.SortCandidates(ranked)
	ranked = s.rerank(ctx, q, ranked)
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	trace := s.buildTrace(q, ranked, time.Since(start))
	if s.deps.Audit != nil {
		if err := s.deps.Audit.Save(ctx, trace); err != nil {
			return nil, retrieval.Trace{}, err
		}
	}
	if s.deps.Recorder != nil {
		s.deps.Recorder.Record(evaluation.Record{
			TraceID:       trace.ID,
			Query:         q,
			CandidateIDs:  trace.CandidateIDs,
			SelectedIDs:   trace.SelectedIDs,
			Scores:        trace.Scores,
			LatencyMS:     trace.LatencyMS,
			TokenEstimate: trace.TokenEstimate,
			CreatedAt:     trace.CreatedAt,
		})
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
