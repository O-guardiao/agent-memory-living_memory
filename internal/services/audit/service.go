package auditservice

import (
	"context"

	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
	"github.com/agent-memory/agent-memory/internal/ports"
)

type Service struct {
	traces ports.TraceStore
	idgen  ports.IDGenerator
	clock  ports.Clock
}

func NewService(traces ports.TraceStore, idgen ports.IDGenerator, clock ports.Clock) *Service {
	return &Service{traces: traces, idgen: idgen, clock: clock}
}

func (s *Service) NewTraceID() string { return s.idgen.NewID("trace") }

func (s *Service) Save(ctx context.Context, trace retrieval.Trace) error {
	if trace.ID == "" {
		trace.ID = s.NewTraceID()
	}
	if trace.CreatedAt.IsZero() {
		trace.CreatedAt = s.clock.Now()
	}
	return s.traces.SaveTrace(ctx, trace)
}
