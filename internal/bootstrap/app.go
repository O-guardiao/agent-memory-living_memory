package bootstrap

import (
	"context"
	"fmt"
	"time"

	hashembed "github.com/agent-memory/agent-memory/internal/adapters/embeddings/hash"
	httpadapter "github.com/agent-memory/agent-memory/internal/adapters/http"
	memoryqueue "github.com/agent-memory/agent-memory/internal/adapters/queue/memory"
	postgresqueue "github.com/agent-memory/agent-memory/internal/adapters/queue/postgres"
	simplerank "github.com/agent-memory/agent-memory/internal/adapters/rerankers/simple"
	memstorage "github.com/agent-memory/agent-memory/internal/adapters/storage/memory"
	neo4jstore "github.com/agent-memory/agent-memory/internal/adapters/storage/neo4j"
	postgresstore "github.com/agent-memory/agent-memory/internal/adapters/storage/postgres"
	qdrantstore "github.com/agent-memory/agent-memory/internal/adapters/storage/qdrant"
	"github.com/agent-memory/agent-memory/internal/adapters/system"
	"github.com/agent-memory/agent-memory/internal/config"
	"github.com/agent-memory/agent-memory/internal/ports"
	agenticsvc "github.com/agent-memory/agent-memory/internal/services/agentic"
	auditservice "github.com/agent-memory/agent-memory/internal/services/audit"
	contextsvc "github.com/agent-memory/agent-memory/internal/services/context"
	"github.com/agent-memory/agent-memory/internal/services/distillation"
	"github.com/agent-memory/agent-memory/internal/services/embedding"
	"github.com/agent-memory/agent-memory/internal/services/forgetting"
	"github.com/agent-memory/agent-memory/internal/services/ingestion"
	retrievalsvc "github.com/agent-memory/agent-memory/internal/services/retrieval"
)

type App struct {
	HTTPServer *httpadapter.Server
	Worker     *ingestion.AsyncService
	Shutdown   *Shutdown
}

func NewApp(cfg config.Config) *App {
	shutdown := NewShutdown()
	stores := storesFor(cfg, shutdown)

	idgen := system.NewIDGenerator()
	clock := system.RealClock{}
	embedder := hashembed.New(cfg.QdrantVectorSize)
	reranker := simplerank.New()

	distiller := distillation.NewService(idgen, clock)
	embedSvc := embedding.NewService(embedder, stores.vectors)
	auditSvc := auditservice.NewService(stores.traces, idgen, clock)
	agenticSvc := agenticsvc.NewService(stores.memories, idgen, clock)

	ingestSvc := ingestion.NewService(ingestion.Dependencies{
		Events:    stores.events,
		Memories:  stores.memories,
		Embedder:  embedder,
		Vectors:   stores.vectors,
		Graph:     stores.graph,
		Distiller: distiller,
		EmbedSvc:  embedSvc,
		IDGen:     idgen,
		Clock:     clock,
	})
	asyncIngestSvc := ingestion.NewAsyncService(stores.queue, ingestSvc)

	retrieveSvc := retrievalsvc.NewService(retrievalsvc.Dependencies{
		Memories: stores.memories,
		Vectors:  stores.vectors,
		Graph:    stores.graph,
		Embedder: embedder,
		Reranker: reranker,
		Audit:    auditSvc,
		Clock:    clock,
	})

	contextSvc := contextsvc.NewAssemblerWithControl(retrieveSvc, agenticSvc)
	forgetSvc := forgetting.NewService(stores.memories, stores.vectors, stores.traces, clock)

	server := httpadapter.NewServer(httpadapter.Dependencies{
		Ingestion:      ingestSvc,
		AsyncIngestion: asyncIngestSvc,
		Retrieval:      retrieveSvc,
		Context:        contextSvc,
		Agentic:        agenticSvc,
		Forgetting:     forgetSvc,
		Memories:       stores.memories,
		Traces:         stores.traces,
		Config:         cfg,
		Limiter:        limiterFor(cfg),
	})

	return &App{HTTPServer: server, Worker: asyncIngestSvc, Shutdown: shutdown}
}

type storeBundle struct {
	events   ports.EventStore
	memories ports.MemoryStore
	vectors  ports.VectorStore
	traces   ports.TraceStore
	graph    ports.GraphStore
	queue    ports.Queue
}

func storesFor(cfg config.Config, shutdown *Shutdown) storeBundle {
	if cfg.StorageMode == "postgres_qdrant_neo4j" && cfg.PostgresDSN != "" {
		db, err := postgresstore.Open(cfg.PostgresDSN)
		if err != nil {
			panic(err)
		}
		if shutdown != nil {
			shutdown.Register("postgres", func(context.Context) error { return db.Close() })
		}
		vectors := qdrantstore.NewVectorStore(cfg.QdrantEndpoint, cfg.QdrantCollection, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := ensureQdrantCollection(ctx, vectors, cfg.QdrantVectorSize); err != nil {
			panic(fmt.Errorf("ensure qdrant collection: %w", err))
		}
		var graph ports.GraphStore = memstorage.NewGraphStore()
		if cfg.Neo4jEndpoint != "" {
			graph = neo4jstore.NewGraphStore(cfg.Neo4jEndpoint, cfg.Neo4jDatabase, cfg.Neo4jBasicAuth, nil)
		}
		return storeBundle{
			events:   postgresstore.NewEventStore(db),
			memories: postgresstore.NewMemoryStore(db),
			vectors:  vectors,
			traces:   postgresstore.NewTraceStore(db),
			graph:    graph,
			queue:    postgresqueue.New(db),
		}
	}
	return storeBundle{
		events:   memstorage.NewEventStore(),
		memories: memstorage.NewMemoryStore(),
		vectors:  memstorage.NewVectorStore(),
		traces:   memstorage.NewTraceStore(),
		graph:    memstorage.NewGraphStore(),
		queue:    memoryqueue.New(),
	}
}

func ensureQdrantCollection(ctx context.Context, vectors *qdrantstore.VectorStore, vectorSize int) error {
	var lastErr error
	for {
		if err := vectors.EnsureCollection(ctx, vectorSize); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return lastErr
		case <-time.After(time.Second):
		}
	}
}
