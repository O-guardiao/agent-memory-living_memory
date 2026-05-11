package memstorage

import (
	"context"
	"strings"
	"sync"

	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
	"github.com/agent-memory/agent-memory/internal/ports"
)

type KeywordStore struct {
	mu   sync.RWMutex
	docs map[string]string
	meta map[string]map[string]string
}

func NewKeywordStore() *KeywordStore {
	return &KeywordStore{docs: map[string]string{}, meta: map[string]map[string]string{}}
}

func (s *KeywordStore) Index(ctx context.Context, id, text string, payload map[string]string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docs[id] = text
	s.meta[id] = payload
	return nil
}

func (s *KeywordStore) Search(ctx context.Context, query retrieval.Query) ([]ports.VectorResult, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ports.VectorResult, 0)
	for id, doc := range s.docs {
		payload := s.meta[id]
		if query.TenantID != "" && payload["tenant_id"] != query.TenantID {
			continue
		}
		if query.UserID != "" && payload["user_id"] != query.UserID {
			continue
		}
		score := keywordScore(query.Text, doc)
		if score > 0 {
			out = append(out, ports.VectorResult{ID: id, Score: score})
		}
	}
	return out, nil
}

func (s *KeywordStore) Delete(ctx context.Context, id string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, id)
	delete(s.meta, id)
	return nil
}

func keywordScore(query, doc string) float64 {
	q := strings.Fields(strings.ToLower(query))
	if len(q) == 0 {
		return 0
	}
	d := strings.ToLower(doc)
	hits := 0
	for _, token := range q {
		if len(token) > 2 && strings.Contains(d, token) {
			hits++
		}
	}
	return float64(hits) / float64(len(q))
}
