package memstorage

import (
	"context"
	"sync"

	"github.com/agent-memory/agent-memory/internal/ports"
)

type GraphStore struct {
	mu    sync.RWMutex
	nodes map[string]ports.GraphNode
	edges []ports.GraphEdge
}

func NewGraphStore() *GraphStore {
	return &GraphStore{nodes: map[string]ports.GraphNode{}, edges: []ports.GraphEdge{}}
}

func (s *GraphStore) UpsertNode(ctx context.Context, node ports.GraphNode) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes[node.ID] = node
	return nil
}

func (s *GraphStore) UpsertEdge(ctx context.Context, edge ports.GraphEdge) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.edges = append(s.edges, edge)
	return nil
}

func (s *GraphStore) Traverse(ctx context.Context, query ports.GraphQuery) ([]ports.GraphResult, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ports.GraphResult, 0)
	for _, edge := range s.edges {
		if edge.FromID == query.StartID {
			out = append(out, ports.GraphResult{NodeID: edge.ToID, Score: 1})
		}
	}
	return out, nil
}
