package forgetting

import (
	"context"
	"strings"
	"testing"
	"time"

	hashembed "github.com/agent-memory/agent-memory/internal/adapters/embeddings/hash"
	memstorage "github.com/agent-memory/agent-memory/internal/adapters/storage/memory"
	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/policy"
	"github.com/agent-memory/agent-memory/internal/services/embedding"
)

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

func TestRedactPIIMasksKnownKinds(t *testing.T) {
	text := "contato: joao@example.com, CPF 123.456.789-09, fone (11) 91234-5678"
	redacted, changed := RedactPII(text)
	if !changed {
		t.Fatal("expected redaction")
	}
	for _, leak := range []string{"joao@example.com", "123.456.789-09", "91234-5678"} {
		if strings.Contains(redacted, leak) {
			t.Fatalf("leaked %q in %q", leak, redacted)
		}
	}
	if _, changed := RedactPII("nenhum dado pessoal aqui"); changed {
		t.Fatal("clean text must not change")
	}
}

func TestRedactorRedactsAndReindexes(t *testing.T) {
	store := memstorage.NewMemoryStore()
	vectors := memstorage.NewVectorStore()
	embedSvc := embedding.NewService(hashembed.New(8), vectors)
	clock := fixedClock{at: time.Unix(5000, 0)}
	redactor := NewRedactor(store, embedSvc, clock)

	mem := memory.NewMemory(time.Unix(1000, 0), memory.TypeFact, memory.ScopeUser, "email do cliente: cliente@example.com")
	mem.ID = "m1"
	mem.TenantID = "tenant_t"
	mem.SafetyLabels = []memory.SafetyLabel{memory.SafetyInternal, memory.SafetyPII}
	if err := store.Upsert(context.Background(), mem); err != nil {
		t.Fatal(err)
	}

	receipt, err := redactor.Redact(context.Background(), "tenant_t", "m1", "")
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if receipt.ID != "redact_m1" || receipt.Reason != "pii_redaction" {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	updated, err := store.Get(context.Background(), "tenant_t", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(updated.Content, "cliente@example.com") {
		t.Fatalf("content still has PII: %q", updated.Content)
	}
	if updated.Version != mem.Version+1 {
		t.Fatalf("expected version bump, got %d", updated.Version)
	}
	for _, label := range updated.SafetyLabels {
		if label == memory.SafetyPII {
			t.Fatal("PII label must be removed after redaction")
		}
	}
}

func TestSweeperExpiresByValidUntilAndTTL(t *testing.T) {
	store := memstorage.NewMemoryStore()
	clock := fixedClock{at: time.Unix(100000, 0)}
	forgetter := NewService(store, memstorage.NewVectorStore(), memstorage.NewTraceStore(), clock)
	sweeper := NewSweeper(store, forgetter, clock)
	ctx := context.Background()

	past := time.Unix(50000, 0)
	expired := memory.NewMemory(time.Unix(1000, 0), memory.TypeFact, memory.ScopeUser, "validade vencida")
	expired.ID = "m1"
	expired.TenantID = "tenant_t"
	expired.ValidUntil = &past
	stale := memory.NewMemory(time.Unix(1000, 0), memory.TypeFact, memory.ScopeUser, "muito antigo")
	stale.ID = "m2"
	stale.TenantID = "tenant_t"
	stale.UpdatedAt = time.Unix(1000, 0)
	fresh := memory.NewMemory(clock.Now(), memory.TypeFact, memory.ScopeUser, "recente")
	fresh.ID = "m3"
	fresh.TenantID = "tenant_t"
	for _, mem := range []memory.Memory{expired, stale, fresh} {
		if err := store.Upsert(ctx, mem); err != nil {
			t.Fatal(err)
		}
	}

	affected, err := sweeper.Sweep(ctx, "tenant_t", policy.RetentionPolicy{DefaultTTL: 24 * time.Hour})
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if affected != 2 {
		t.Fatalf("expected 2 affected, got %d", affected)
	}
	kept, err := store.Get(ctx, "tenant_t", "m3")
	if err != nil || kept.Status != memory.StatusActive {
		t.Fatalf("fresh memory must stay active: %+v %v", kept, err)
	}
	gone, err := store.Get(ctx, "tenant_t", "m1")
	if err != nil || gone.Status != memory.StatusExpired {
		t.Fatalf("expected expired status: %+v %v", gone, err)
	}
}
