package memstorage

import (
	"context"
	"math"
	"sort"
	"sync"

	"github.com/agent-memory/agent-memory/internal/ports"
)

type VectorStore struct {
	mu    sync.RWMutex
	items map[string]ports.VectorItem
}

func NewVectorStore() *VectorStore {
	return &VectorStore{items: map[string]ports.VectorItem{}}
}

func (s *VectorStore) Upsert(ctx context.Context, items []ports.VectorItem) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range items {
		s.items[item.ID] = item
	}
	return nil
}

func (s *VectorStore) Search(ctx context.Context, query ports.VectorQuery) ([]ports.VectorResult, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ports.VectorResult, 0)
	for _, item := range s.items {
		if query.TenantID != "" && item.Payload["tenant_id"] != query.TenantID {
			continue
		}
		if query.UserID != "" && item.Payload["user_id"] != query.UserID {
			continue
		}
		if !payloadMatches(item.Payload, query.Filters) {
			continue
		}
		out = append(out, ports.VectorResult{ID: item.ID, Score: cosine(query.Vector, item.Vector)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if query.Limit > 0 && len(out) > query.Limit {
		out = out[:query.Limit]
	}
	return out, nil
}

func (s *VectorStore) Delete(ctx context.Context, ids []string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		delete(s.items, id)
	}
	return nil
}

func payloadMatches(payload, filters map[string]string) bool {
	for k, v := range filters {
		if payload[k] != v {
			return false
		}
	}
	return true
}

func cosine(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var dot, aa, bb float64
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
		aa += a[i] * a[i]
		bb += b[i] * b[i]
	}
	if aa == 0 || bb == 0 {
		return 0
	}
	return dot / (math.Sqrt(aa) * math.Sqrt(bb))
}
