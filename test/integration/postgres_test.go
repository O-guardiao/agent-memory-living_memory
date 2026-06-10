package integration

import (
	"context"
	"testing"
	"time"

	postgresqueue "github.com/agent-memory/agent-memory/internal/adapters/queue/postgres"
	postgresstore "github.com/agent-memory/agent-memory/internal/adapters/storage/postgres"
	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
	"github.com/agent-memory/agent-memory/internal/ports"
)

func TestPostgresPlaceholder(t *testing.T) { t.Skip("requires Postgres") }

// TestPostgresStores runs migrations and exercises the stores end to end.
// Gate: MEMORY_TEST_POSTGRES_DSN.
func TestPostgresStores(t *testing.T) {
	dsn := dsnOrSkip(t, "MEMORY_TEST_POSTGRES_DSN")
	db, err := postgresstore.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := postgresstore.WaitFor(ctx, db); err != nil {
		t.Skipf("postgres unreachable: %v", err)
	}
	if err := postgresstore.RunMigrations(ctx, db, "../../migrations/postgres"); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	memories := postgresstore.NewMemoryStore(db)
	mem := memory.NewMemory(time.Now().UTC(), memory.TypeFact, memory.ScopeUser, "integração postgres funciona")
	mem.ID = "mem_it_pg"
	mem.TenantID = "tenant_it"
	mem.UserID = "user_it"
	if err := memories.Upsert(ctx, mem); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := memories.Get(ctx, "tenant_it", "mem_it_pg")
	if err != nil || got.Content != mem.Content {
		t.Fatalf("get: %+v %v", got, err)
	}
	found, err := memories.SearchByText(ctx, retrieval.Query{TenantID: "tenant_it", UserID: "user_it", Text: "integração postgres", Limit: 5})
	if err != nil || len(found) == 0 {
		t.Fatalf("search by text: %d results, %v", len(found), err)
	}

	// Queue round trip on memory_jobs.
	queue := postgresqueue.New(db)
	if err := queue.Publish(ctx, ports.QueueMessage{Topic: "it.topic", Key: "k", Body: []byte("payload")}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	received := make(chan []byte, 1)
	subCtx, stop := context.WithCancel(ctx)
	go func() {
		_ = queue.Subscribe(subCtx, "it.topic", func(_ context.Context, msg ports.QueueMessage) error {
			select {
			case received <- msg.Body:
			default:
			}
			return nil
		})
	}()
	select {
	case body := <-received:
		if string(body) != "payload" {
			t.Fatalf("unexpected payload: %q", body)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("queue message not delivered")
	}
	stop()

	if err := memories.Delete(ctx, "tenant_it", "mem_it_pg"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Scheduler maintenance SQL must be valid.
	if _, err := db.ExecContext(ctx,
		`DELETE FROM memory_jobs WHERE status = 'done' AND completed_at < now() - $1::interval`,
		"604800 seconds",
	); err != nil {
		t.Fatalf("jobs prune SQL: %v", err)
	}
	tenants, err := postgresstore.ListTenants(ctx, db)
	if err != nil {
		t.Fatalf("list tenants: %v", err)
	}
	t.Logf("tenants: %v", tenants)
}
