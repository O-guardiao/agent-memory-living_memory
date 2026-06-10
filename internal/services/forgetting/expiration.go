package forgetting

// Add retention sweeps for ValidUntil and tenant policies here.

import (
	"context"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/policy"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
	"github.com/agent-memory/agent-memory/internal/ports"
)

const sweepBatchSize = 500

// Sweeper expires memories whose validity window or tenant TTL elapsed.
type Sweeper struct {
	memories  ports.MemoryStore
	forgetter *Service
	clock     ports.Clock
}

func NewSweeper(memories ports.MemoryStore, forgetter *Service, clock ports.Clock) *Sweeper {
	return &Sweeper{memories: memories, forgetter: forgetter, clock: clock}
}

// Sweep marks expired memories (ValidUntil passed, or older than the
// policy TTL) and, when the policy demands it, hard-deletes them through
// the forgetting service so vectors and traces go too. Returns the number
// of memories affected.
func (s *Sweeper) Sweep(ctx context.Context, tenantID string, pol policy.RetentionPolicy) (int, error) {
	now := s.clock.Now()
	memories, err := s.memories.List(ctx, retrieval.Query{TenantID: tenantID, Limit: sweepBatchSize})
	if err != nil {
		return 0, err
	}
	affected := 0
	for _, mem := range memories {
		if mem.Status != memory.StatusActive {
			continue
		}
		expired := mem.ValidUntil != nil && now.After(*mem.ValidUntil)
		if !expired && pol.DefaultTTL > 0 && now.Sub(mem.UpdatedAt) > pol.DefaultTTL {
			expired = true
		}
		if !expired {
			continue
		}
		if pol.DeleteExpiredHard {
			if _, err := s.forgetter.Delete(ctx, policy.DeletionRequest{
				TenantID: tenantID,
				MemoryID: mem.ID,
				Reason:   "retention_sweep",
			}); err != nil {
				return affected, err
			}
		} else {
			mem.Status = memory.StatusExpired
			mem.UpdatedAt = now
			if err := s.memories.Upsert(ctx, mem); err != nil {
				return affected, err
			}
		}
		affected++
	}
	return affected, nil
}
