package forgetting

import (
	"context"

	"github.com/agent-memory/agent-memory/internal/domain/policy"
	"github.com/agent-memory/agent-memory/internal/ports"
)

type Service struct {
	memories ports.MemoryStore
	vectors  ports.VectorStore
	traces   ports.TraceStore
	clock    ports.Clock
}

func NewService(memories ports.MemoryStore, vectors ports.VectorStore, traces ports.TraceStore, clock ports.Clock) *Service {
	return &Service{memories: memories, vectors: vectors, traces: traces, clock: clock}
}

func (s *Service) Delete(ctx context.Context, req policy.DeletionRequest) (policy.DeletionReceipt, error) {
	if err := s.memories.Delete(ctx, req.TenantID, req.MemoryID); err != nil {
		return policy.DeletionReceipt{}, err
	}
	_ = s.vectors.Delete(ctx, []string{req.MemoryID})
	_ = s.traces.DeleteByMemoryID(ctx, req.TenantID, req.MemoryID)
	return policy.DeletionReceipt{
		ID:        "del_" + req.MemoryID,
		TenantID:  req.TenantID,
		MemoryID:  req.MemoryID,
		DeletedAt: s.clock.Now(),
		Reason:    req.Reason,
	}, nil
}
