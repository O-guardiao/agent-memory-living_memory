package consolidation

import (
	"context"
	"testing"
	"time"

	memstorage "github.com/agent-memory/agent-memory/internal/adapters/storage/memory"
	"github.com/agent-memory/agent-memory/internal/adapters/system"
	"github.com/agent-memory/agent-memory/internal/domain/memory"
)

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

func newMemory(id, content string, typ memory.Type) memory.Memory {
	mem := memory.NewMemory(time.Unix(1000, 0), typ, memory.ScopeUser, content)
	mem.ID = id
	mem.TenantID = "tenant_t"
	mem.UserID = "user_1"
	mem.SourceEventIDs = []string{"evt_" + id}
	return mem
}

func TestSimilarAndMerge(t *testing.T) {
	a := newMemory("a", "Prefiro Go para serviços de produção no backend", memory.TypePreference)
	b := newMemory("b", "Prefiro Go para serviços de produção no backend agora", memory.TypePreference)
	if Similar(a.Content, b.Content) < mergeThreshold {
		t.Fatalf("expected near-duplicates above merge threshold: %f", Similar(a.Content, b.Content))
	}
	b.Confidence = 0.95
	merged := Merge(a, b, time.Unix(2000, 0))
	if merged.Confidence != 0.95 {
		t.Fatalf("expected max confidence, got %f", merged.Confidence)
	}
	if len(merged.SourceEventIDs) != 2 {
		t.Fatalf("expected united source events, got %v", merged.SourceEventIDs)
	}
	if merged.Version != a.Version+1 {
		t.Fatalf("expected version bump, got %d", merged.Version)
	}
}

func TestContradicts(t *testing.T) {
	a := newMemory("a", "usuário gosta de trabalhar com kubernetes em produção", memory.TypePreference)
	b := newMemory("b", "usuário não gosta de trabalhar com kubernetes em produção", memory.TypePreference)
	if !Contradicts(a, b) {
		t.Fatal("expected contradiction for negated statement")
	}
	c := newMemory("c", "usuário gosta de trabalhar com kubernetes em produção sempre", memory.TypePreference)
	if Contradicts(a, c) {
		t.Fatal("same polarity must not contradict")
	}
}

func TestConsolidateMergesDuplicates(t *testing.T) {
	store := memstorage.NewMemoryStore()
	svc := NewService(store, memstorage.NewGraphStore(), system.NewIDGenerator(), fixedClock{at: time.Unix(3000, 0)})
	ctx := context.Background()

	existing := newMemory("m1", "Prefiro Go para serviços de produção no backend", memory.TypePreference)
	if err := store.Upsert(ctx, existing); err != nil {
		t.Fatal(err)
	}
	candidate := newMemory("m2", "Prefiro Go para serviços de produção no backend agora", memory.TypePreference)
	result, err := svc.Consolidate(ctx, candidate)
	if err != nil {
		t.Fatalf("consolidate: %v", err)
	}
	if result.Action != "merged" || result.Memory.ID != "m1" {
		t.Fatalf("expected merge into existing, got %+v", result)
	}
}

func TestConsolidateSupersedesChangedFacts(t *testing.T) {
	store := memstorage.NewMemoryStore()
	clock := fixedClock{at: time.Unix(3000, 0)}
	svc := NewService(store, memstorage.NewGraphStore(), system.NewIDGenerator(), clock)
	ctx := context.Background()

	existing := newMemory("m1", "trabalho atualmente na empresa Acme como engenheiro", memory.TypeFact)
	if err := store.Upsert(ctx, existing); err != nil {
		t.Fatal(err)
	}
	candidate := newMemory("m2", "trabalho atualmente na empresa Globex como engenheiro", memory.TypeFact)
	result, err := svc.Consolidate(ctx, candidate)
	if err != nil {
		t.Fatalf("consolidate: %v", err)
	}
	if result.Action != "superseded" {
		t.Fatalf("expected supersede, got %+v", result)
	}
	old, err := store.Get(ctx, "tenant_t", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if old.Status != memory.StatusSuperseded || old.ValidUntil == nil {
		t.Fatalf("expected old memory closed, got %+v", old)
	}
	if _, ok := old.Metadata["versions"]; !ok {
		t.Fatal("expected version snapshot in metadata")
	}
}

func TestConsolidatePassThroughForNewContent(t *testing.T) {
	store := memstorage.NewMemoryStore()
	svc := NewService(store, nil, system.NewIDGenerator(), fixedClock{at: time.Unix(3000, 0)})
	candidate := newMemory("m9", "conteúdo totalmente inédito sobre observabilidade", memory.TypeFact)
	result, err := svc.Consolidate(context.Background(), candidate)
	if err != nil || result.Action != "new" {
		t.Fatalf("expected pass-through: %+v %v", result, err)
	}
}
