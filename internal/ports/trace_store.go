package ports

import (
	"context"

	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

type TraceStore interface {
	SaveTrace(ctx context.Context, trace retrieval.Trace) error
	GetTrace(ctx context.Context, tenantID, traceID string) (retrieval.Trace, error)
	DeleteByMemoryID(ctx context.Context, tenantID, memoryID string) error
}
