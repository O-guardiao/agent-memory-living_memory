package memstorage

import (
	"context"
	"strings"
	"sync"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

type MemoryStore struct {
	mu       sync.RWMutex
	memories map[string]memory.Memory
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{memories: map[string]memory.Memory{}}
}

func (s *MemoryStore) Upsert(ctx context.Context, mem memory.Memory) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memories[mem.ID] = mem
	return nil
}

func (s *MemoryStore) Get(ctx context.Context, tenantID, memoryID string) (memory.Memory, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	mem, ok := s.memories[memoryID]
	if !ok || mem.TenantID != tenantID || mem.Status == memory.StatusDeleted {
		return memory.Memory{}, memory.ErrNotFound
	}
	return mem, nil
}

func (s *MemoryStore) Delete(ctx context.Context, tenantID, memoryID string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	mem, ok := s.memories[memoryID]
	if !ok || mem.TenantID != tenantID {
		return memory.ErrNotFound
	}
	mem.Status = memory.StatusDeleted
	s.memories[memoryID] = mem
	return nil
}

func (s *MemoryStore) List(ctx context.Context, q retrieval.Query) ([]memory.Memory, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]memory.Memory, 0)
	for _, mem := range s.memories {
		if matchesQuery(mem, q) {
			out = append(out, mem)
		}
	}
	return out, nil
}

func (s *MemoryStore) SearchByText(ctx context.Context, q retrieval.Query) ([]memory.Memory, error) {
	_ = ctx
	needle := strings.ToLower(q.NormalizedText())
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]memory.Memory, 0)
	for _, mem := range s.memories {
		if !matchesQuery(mem, q) {
			continue
		}
		if needle == "" || strings.Contains(strings.ToLower(mem.Content), needle) || hasAnyToken(mem.Content, needle) {
			out = append(out, mem)
		}
	}
	return out, nil
}

func matchesQuery(mem memory.Memory, q retrieval.Query) bool {
	if mem.Status != memory.StatusActive {
		return false
	}
	if q.TenantID != "" && mem.TenantID != q.TenantID {
		return false
	}
	if q.UserID != "" && mem.UserID != q.UserID {
		return false
	}
	if q.AgentID != "" && mem.AgentID != "" && mem.AgentID != q.AgentID {
		return false
	}
	if q.ProjectID != "" && mem.ProjectID != "" && mem.ProjectID != q.ProjectID {
		return false
	}
	if q.SessionID != "" && mem.SessionID != "" && mem.SessionID != q.SessionID {
		return false
	}
	return true
}

func hasAnyToken(content, query string) bool {
	haystack := strings.ToLower(content)
	for _, token := range strings.Fields(strings.ToLower(query)) {
		token = strings.Trim(token, " .,;:!?()[]{}\"'")
		if len(token) > 2 && strings.Contains(haystack, token) {
			return true
		}
	}
	return false
}
