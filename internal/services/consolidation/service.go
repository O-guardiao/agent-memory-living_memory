package consolidation

// Add merge, contradiction detection, supersession and temporal versioning here.

import (
	"context"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
	"github.com/agent-memory/agent-memory/internal/ports"
)

type Service struct {
	memories ports.MemoryStore
	graph    ports.GraphStore
	idgen    ports.IDGenerator
	clock    ports.Clock
}

func NewService(memories ports.MemoryStore, graph ports.GraphStore, idgen ports.IDGenerator, clock ports.Clock) *Service {
	return &Service{memories: memories, graph: graph, idgen: idgen, clock: clock}
}

// Result describes what happened to a candidate during consolidation.
type Result struct {
	// Memory is the memory to persist (the merged neighbor or the candidate).
	Memory memory.Memory
	// Action is one of "merged", "superseded", "contradicts", "new".
	Action string
	// Updated holds neighbors that were modified and already persisted.
	Updated []memory.Memory
}

// Consolidate compares the candidate against active same-scope memories:
// near-duplicates merge (preserving source events and confidence), newer
// statements supersede the old version (snapshotting it first), and
// contradictions keep both linked with a contradicts relation.
func (s *Service) Consolidate(ctx context.Context, candidate memory.Memory) (Result, error) {
	now := s.clock.Now()
	neighbors, err := s.memories.List(ctx, retrieval.Query{
		TenantID: candidate.TenantID,
		UserID:   candidate.UserID,
		Limit:    50,
		Filters:  map[string]string{"type": string(candidate.Type)},
	})
	if err != nil {
		return Result{Memory: candidate, Action: "new"}, err
	}

	for i := range neighbors {
		neighbor := neighbors[i]
		if neighbor.ID == candidate.ID || !neighbor.IsActive(now) {
			continue
		}
		if Similar(neighbor.Content, candidate.Content) >= mergeThreshold {
			merged := Merge(neighbor, candidate, now)
			if err := s.memories.Upsert(ctx, merged); err != nil {
				return Result{Memory: candidate, Action: "new"}, err
			}
			return Result{Memory: merged, Action: "merged"}, nil
		}
		if ShouldSupersede(neighbor, candidate) {
			snapshot := Snapshot(neighbor, "superseded by "+candidate.ID, s.idgen, now)
			AppendVersion(&neighbor, snapshot)
			Supersede(&neighbor, now)
			if err := s.memories.Upsert(ctx, neighbor); err != nil {
				return Result{Memory: candidate, Action: "new"}, err
			}
			s.linkRelation(ctx, candidate, neighbor, memory.RelationSupersedes)
			return Result{Memory: candidate, Action: "superseded", Updated: []memory.Memory{neighbor}}, nil
		}
		if Contradicts(neighbor, candidate) {
			s.linkRelation(ctx, candidate, neighbor, memory.RelationContradicts)
			return Result{Memory: candidate, Action: "contradicts", Updated: []memory.Memory{neighbor}}, nil
		}
	}
	return Result{Memory: candidate, Action: "new"}, nil
}

// linkRelation records the relation as a graph edge; relations are
// advisory, so failures must not break ingestion.
func (s *Service) linkRelation(ctx context.Context, from, to memory.Memory, relation memory.RelationType) {
	if s.graph == nil {
		return
	}
	_ = s.graph.UpsertEdge(ctx, ports.GraphEdge{
		FromID: from.ID,
		ToID:   to.ID,
		Type:   string(relation),
		Payload: map[string]string{
			"tenant_id": from.TenantID,
			"relation":  string(relation),
		},
	})
}
