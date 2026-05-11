package ports

import (
	"context"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
)

type EventStore interface {
	Append(ctx context.Context, event memory.Event) error
	Get(ctx context.Context, tenantID, eventID string) (memory.Event, error)
	ListBySession(ctx context.Context, tenantID, sessionID string, limit int) ([]memory.Event, error)
}
