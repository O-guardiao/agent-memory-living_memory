package forgetting

import (
	"context"

	"github.com/agent-memory/agent-memory/internal/domain/policy"
	"github.com/agent-memory/agent-memory/internal/ports"
	auditservice "github.com/agent-memory/agent-memory/internal/services/audit"
)

type Service struct {
	memories ports.MemoryStore
	vectors  ports.VectorStore
	traces   ports.TraceStore
	clock    ports.Clock
	// receipts, when set, chains an immutable audit receipt per deletion.
	receipts *auditservice.ReceiptLog
	idgen    ports.IDGenerator
}

func NewService(memories ports.MemoryStore, vectors ports.VectorStore, traces ports.TraceStore, clock ports.Clock) *Service {
	return &Service{memories: memories, vectors: vectors, traces: traces, clock: clock}
}

// WithReceipts enables hash-chained audit receipts for deletions.
func (s *Service) WithReceipts(receipts *auditservice.ReceiptLog, idgen ports.IDGenerator) *Service {
	s.receipts = receipts
	s.idgen = idgen
	return s
}

func (s *Service) Delete(ctx context.Context, req policy.DeletionRequest) (policy.DeletionReceipt, error) {
	if err := s.memories.Delete(ctx, req.TenantID, req.MemoryID); err != nil {
		return policy.DeletionReceipt{}, err
	}
	_ = s.vectors.Delete(ctx, []string{req.MemoryID})
	_ = s.traces.DeleteByMemoryID(ctx, req.TenantID, req.MemoryID)
	if s.receipts != nil && s.idgen != nil {
		// Receipt persistence is best effort; deletion already happened.
		_, _ = s.receipts.Append(ctx, req.TenantID, "delete", req.MemoryID, req.Reason, s.idgen, s.clock.Now())
	}
	return policy.DeletionReceipt{
		ID:        "del_" + req.MemoryID,
		TenantID:  req.TenantID,
		MemoryID:  req.MemoryID,
		DeletedAt: s.clock.Now(),
		Reason:    req.Reason,
	}, nil
}
