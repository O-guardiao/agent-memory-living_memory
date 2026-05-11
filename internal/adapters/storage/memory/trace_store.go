package memstorage

import (
	"context"
	"sync"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

type TraceStore struct {
	mu     sync.RWMutex
	traces map[string]retrieval.Trace
}

func NewTraceStore() *TraceStore {
	return &TraceStore{traces: map[string]retrieval.Trace{}}
}

func (s *TraceStore) SaveTrace(ctx context.Context, trace retrieval.Trace) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traces[trace.ID] = trace
	return nil
}

func (s *TraceStore) GetTrace(ctx context.Context, tenantID, traceID string) (retrieval.Trace, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	trace, ok := s.traces[traceID]
	if !ok || trace.TenantID != tenantID {
		return retrieval.Trace{}, memory.ErrNotFound
	}
	return trace, nil
}

func (s *TraceStore) DeleteByMemoryID(ctx context.Context, tenantID, memoryID string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, trace := range s.traces {
		if trace.TenantID != tenantID {
			continue
		}
		trace.CandidateIDs = removeString(trace.CandidateIDs, memoryID)
		trace.SelectedIDs = removeString(trace.SelectedIDs, memoryID)
		trace.RejectedIDs = removeString(trace.RejectedIDs, memoryID)
		delete(trace.Scores, memoryID)
		s.traces[id] = trace
	}
	return nil
}

func removeString(in []string, target string) []string {
	out := in[:0]
	for _, v := range in {
		if v != target {
			out = append(out, v)
		}
	}
	return out
}
