package ingestion

import (
	"context"
	"testing"

	hashembed "github.com/agent-memory/agent-memory/internal/adapters/embeddings/hash"
	memoryqueue "github.com/agent-memory/agent-memory/internal/adapters/queue/memory"
	memstorage "github.com/agent-memory/agent-memory/internal/adapters/storage/memory"
	"github.com/agent-memory/agent-memory/internal/adapters/system"
	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
	"github.com/agent-memory/agent-memory/internal/services/distillation"
	"github.com/agent-memory/agent-memory/internal/services/embedding"
)

func TestAsyncWorkerProcessesQueuedIngestRequest(t *testing.T) {
	ctx := context.Background()
	events := memstorage.NewEventStore()
	memories := memstorage.NewMemoryStore()
	vectors := memstorage.NewVectorStore()
	ids := system.NewIDGenerator()
	clock := system.RealClock{}
	embedder := hashembed.New(16)
	queue := memoryqueue.New()

	service := NewService(Dependencies{
		Events:    events,
		Memories:  memories,
		Embedder:  embedder,
		Vectors:   vectors,
		Distiller: distillation.NewService(ids, clock),
		EmbedSvc:  embedding.NewService(embedder, vectors),
		IDGen:     ids,
		Clock:     clock,
	})
	async := NewAsyncService(queue, service)
	if err := async.StartWorker(ctx); err != nil {
		t.Fatalf("start worker: %v", err)
	}
	if err := async.Enqueue(ctx, IngestRequest{
		TenantID:  "tenant_a",
		UserID:    "user_1",
		ProjectID: "psi",
		Role:      memory.RoleUser,
		Content:   "Eu prefiro Go para infraestrutura de memória.",
	}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	stored, err := memories.SearchByText(ctx, retrieval.Query{
		TenantID:  "tenant_a",
		UserID:    "user_1",
		ProjectID: "psi",
		Text:      "prefiro Go",
		Limit:     5,
	})
	if err != nil {
		t.Fatalf("search memories: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected queued ingest to create one memory, got %d", len(stored))
	}
}
