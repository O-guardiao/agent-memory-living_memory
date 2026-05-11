package memstorage

import (
	"context"
	"sync"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
)

type EventStore struct {
	mu     sync.RWMutex
	events map[string]memory.Event
}

func NewEventStore() *EventStore {
	return &EventStore{events: map[string]memory.Event{}}
}

func (s *EventStore) Append(ctx context.Context, event memory.Event) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[event.ID] = event
	return nil
}

func (s *EventStore) Get(ctx context.Context, tenantID, eventID string) (memory.Event, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	event, ok := s.events[eventID]
	if !ok || event.TenantID != tenantID {
		return memory.Event{}, memory.ErrNotFound
	}
	return event, nil
}

func (s *EventStore) ListBySession(ctx context.Context, tenantID, sessionID string, limit int) ([]memory.Event, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	out := make([]memory.Event, 0)
	for _, event := range s.events {
		if event.TenantID == tenantID && event.SessionID == sessionID {
			out = append(out, event)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}
