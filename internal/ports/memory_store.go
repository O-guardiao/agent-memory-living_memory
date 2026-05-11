package ports

import (
	"context"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

type MemoryStore interface {
	Upsert(ctx context.Context, mem memory.Memory) error
	Get(ctx context.Context, tenantID, memoryID string) (memory.Memory, error)
	Delete(ctx context.Context, tenantID, memoryID string) error
	List(ctx context.Context, q retrieval.Query) ([]memory.Memory, error)
	SearchByText(ctx context.Context, q retrieval.Query) ([]memory.Memory, error)
}
