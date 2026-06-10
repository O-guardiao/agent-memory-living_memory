package distillation

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/agent-memory/agent-memory/internal/adapters/system"
	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/ports"
)

type fakeLLM struct {
	payload string
	err     error
}

func (f fakeLLM) Generate(ctx context.Context, req ports.GenerateRequest) (ports.GenerateResponse, error) {
	return ports.GenerateResponse{Text: f.payload}, f.err
}

func (f fakeLLM) GenerateJSON(ctx context.Context, req ports.GenerateRequest, out any) error {
	if f.err != nil {
		return f.err
	}
	return json.Unmarshal([]byte(f.payload), out)
}

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

func testEvent() memory.Event {
	return memory.Event{
		ID:       "evt_1",
		TenantID: "tenant_t",
		UserID:   "user_1",
		Role:     memory.RoleUser,
		Content:  "Eu prefiro Go para o core de produção.",
	}
}

func TestLLMExtractorEmitsTypedCitedMemories(t *testing.T) {
	llm := fakeLLM{payload: `[{"type":"preference","content":"User prefers Go for the production core.","scope":"user","confidence":0.9,"importance":0.8}]`}
	svc := NewLLMService(llm, system.NewIDGenerator(), fixedClock{at: time.Unix(0, 0)})

	memories, err := svc.Extract(context.Background(), testEvent())
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(memories))
	}
	mem := memories[0]
	if mem.Type != memory.TypePreference {
		t.Fatalf("unexpected type: %s", mem.Type)
	}
	if len(mem.SourceEventIDs) != 1 || mem.SourceEventIDs[0] != "evt_1" {
		t.Fatalf("expected citation to source event, got %v", mem.SourceEventIDs)
	}
	if mem.Metadata["distiller"] != "llm_v1" {
		t.Fatalf("expected llm_v1 distiller marker, got %#v", mem.Metadata)
	}
	if mem.Confidence != 0.9 || mem.Importance != 0.8 {
		t.Fatalf("unexpected scores: %f/%f", mem.Confidence, mem.Importance)
	}
}

func TestLLMExtractorFallsBackToHeuristic(t *testing.T) {
	llm := fakeLLM{err: errors.New("provider down")}
	svc := NewLLMService(llm, system.NewIDGenerator(), fixedClock{at: time.Unix(0, 0)})

	memories, err := svc.Extract(context.Background(), testEvent())
	if err != nil {
		t.Fatalf("extract must not fail when LLM is down: %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("expected heuristic fallback memory, got %d", len(memories))
	}
	if memories[0].Metadata["distiller"] != "heuristic_v1" {
		t.Fatalf("expected heuristic fallback, got %#v", memories[0].Metadata)
	}
}

func TestLLMExtractorSanitizesInvalidTypeAndScope(t *testing.T) {
	llm := fakeLLM{payload: `[{"type":"banana","content":"Eu prefiro chá.","scope":"galaxy","confidence":2.5,"importance":-1}]`}
	svc := NewLLMService(llm, system.NewIDGenerator(), fixedClock{at: time.Unix(0, 0)})

	memories, err := svc.Extract(context.Background(), testEvent())
	if err != nil || len(memories) != 1 {
		t.Fatalf("extract: %v (%d memories)", err, len(memories))
	}
	mem := memories[0]
	if mem.Type != memory.TypePreference {
		t.Fatalf("expected classify fallback, got %s", mem.Type)
	}
	if mem.Scope != memory.ScopeUser {
		t.Fatalf("expected user scope fallback, got %s", mem.Scope)
	}
	if mem.Confidence > 1 || mem.Importance < 0 {
		t.Fatalf("scores not clamped: %f/%f", mem.Confidence, mem.Importance)
	}
}
