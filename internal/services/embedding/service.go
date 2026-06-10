package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/ports"
)

type Service struct {
	embedder ports.Embedder
	vectors  ports.VectorStore
	// cache, when set, memoizes embeddings keyed by content hash.
	cache    ports.Cache
	cacheTTL time.Duration
}

func NewService(embedder ports.Embedder, vectors ports.VectorStore) *Service {
	return &Service{embedder: embedder, vectors: vectors}
}

// NewServiceWithCache memoizes embeddings in the cache (sha256 of the
// text) so repeated contents skip the provider round-trip.
func NewServiceWithCache(embedder ports.Embedder, vectors ports.VectorStore, cache ports.Cache) *Service {
	return &Service{embedder: embedder, vectors: vectors, cache: cache, cacheTTL: 24 * time.Hour}
}

func (s *Service) embed(ctx context.Context, text string) ([]float64, error) {
	if s.cache == nil {
		return s.embedder.Embed(ctx, text)
	}
	key := cacheKey(text)
	if data, ok, err := s.cache.Get(ctx, key); err == nil && ok {
		var vec []float64
		if json.Unmarshal(data, &vec) == nil && len(vec) > 0 {
			return vec, nil
		}
	}
	vec, err := s.embedder.Embed(ctx, text)
	if err != nil {
		return nil, err
	}
	if data, err := json.Marshal(vec); err == nil {
		// Best effort: a cache outage must not fail indexing.
		_ = s.cache.Set(ctx, key, data, s.cacheTTL)
	}
	return vec, nil
}

func cacheKey(text string) string {
	sum := sha256.Sum256([]byte(text))
	return "embed:" + hex.EncodeToString(sum[:])
}

func (s *Service) IndexMemory(ctx context.Context, mem memory.Memory) error {
	vec, err := s.embed(ctx, mem.Content)
	if err != nil {
		return err
	}
	return s.vectors.Upsert(ctx, []ports.VectorItem{vectorItem(mem, vec)})
}

func (s *Service) IndexMemories(ctx context.Context, memories []memory.Memory) error {
	if len(memories) == 0 {
		return nil
	}
	texts := make([]string, 0, len(memories))
	for _, mem := range memories {
		texts = append(texts, mem.Content)
	}
	vectors, err := s.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return err
	}
	items := make([]ports.VectorItem, 0, len(memories))
	for index, mem := range memories {
		if index >= len(vectors) {
			break
		}
		items = append(items, vectorItem(mem, vectors[index]))
	}
	return s.vectors.Upsert(ctx, items)
}

func vectorItem(mem memory.Memory, vec []float64) ports.VectorItem {
	return ports.VectorItem{
		ID:     mem.ID,
		Vector: vec,
		Payload: map[string]string{
			"tenant_id":  mem.TenantID,
			"user_id":    mem.UserID,
			"agent_id":   mem.AgentID,
			"project_id": mem.ProjectID,
			"type":       string(mem.Type),
			"scope":      string(mem.Scope),
		},
	}
}
