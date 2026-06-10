package retrievalsvc

// Add temporal graph traversal here when a GraphStore adapter is wired.

import (
	"context"

	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
	"github.com/agent-memory/agent-memory/internal/ports"
)

// graphExpand widens the candidate set by traversing relations from the
// current seeds. Temporal validity is enforced via Memory.IsActive at
// q.Now, so superseded/expired neighbors never come back.
func (s *Service) graphExpand(ctx context.Context, q retrieval.Query, depth int, candidates map[string]retrieval.Candidate) error {
	if s.deps.Graph == nil {
		return nil
	}
	seedIDs := make([]string, 0, len(candidates))
	for id := range candidates {
		seedIDs = append(seedIDs, id)
	}
	for _, seedID := range seedIDs {
		graphResults, err := s.deps.Graph.Traverse(ctx, ports.GraphQuery{
			TenantID: q.TenantID,
			StartID:  seedID,
			Depth:    depth,
		})
		if err != nil {
			return err
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
	return nil
}
