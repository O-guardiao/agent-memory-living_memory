package embedding

import (
	"context"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/ports"
)

type Service struct {
	embedder ports.Embedder
	vectors  ports.VectorStore
}

func NewService(embedder ports.Embedder, vectors ports.VectorStore) *Service {
	return &Service{embedder: embedder, vectors: vectors}
}

func (s *Service) IndexMemory(ctx context.Context, mem memory.Memory) error {
	vec, err := s.embedder.Embed(ctx, mem.Content)
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
