package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	postgresstore "github.com/agent-memory/agent-memory/internal/adapters/storage/postgres"
	"github.com/agent-memory/agent-memory/internal/adapters/system"
	"github.com/agent-memory/agent-memory/internal/config"
	"github.com/agent-memory/agent-memory/internal/domain/policy"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
	auditservice "github.com/agent-memory/agent-memory/internal/services/audit"
	"github.com/agent-memory/agent-memory/internal/services/consolidation"
	"github.com/agent-memory/agent-memory/internal/services/embedding"
	"github.com/agent-memory/agent-memory/internal/services/forgetting"
	"github.com/agent-memory/agent-memory/internal/services/scheduler"
)

type SchedulerApp struct {
	Runner   *scheduler.Runner
	Shutdown *Shutdown
}

// NewSchedulerApp wires the periodic maintenance jobs: retention sweeps,
// memory_jobs upkeep (postgres mode), vector reindexing and compaction.
// In postgres mode every tick is guarded by an advisory lock so multiple
// scheduler replicas stay single-flight.
func NewSchedulerApp(cfg config.Config) *SchedulerApp {
	shutdown := NewShutdown()
	stores := storesFor(cfg, shutdown)

	idgen := system.NewIDGenerator()
	clock := system.RealClock{}
	embedSvc := embedding.NewService(embedderFor(cfg), stores.vectors)
	forgetSvc := forgetting.NewService(stores.memories, stores.vectors, stores.traces, clock).
		WithReceipts(auditservice.NewReceiptLog(objectStoreFor(cfg)), idgen)
	sweeper := forgetting.NewSweeper(stores.memories, forgetSvc, clock)

	jobs := []scheduler.Job{
		{
			Name:     "retention",
			Interval: cfg.SchedulerRetentionInterval,
			Run: leaderSafe(stores.db, "memory-scheduler:retention", func(ctx context.Context) error {
				tenants := tenantsFor(ctx, cfg, stores.db)
				for _, tenant := range tenants {
					if affected, err := sweeper.Sweep(ctx, tenant, policy.RetentionPolicy{TenantID: tenant}); err != nil {
						return err
					} else if affected > 0 {
						log.Printf("retention: expired %d memories for %s", affected, tenant)
					}
				}
				return nil
			}),
		},
		{
			Name:     "reindex",
			Interval: cfg.SchedulerReindexInterval,
			Run: leaderSafe(stores.db, "memory-scheduler:reindex", func(ctx context.Context) error {
				for _, tenant := range tenantsFor(ctx, cfg, stores.db) {
					memories, err := stores.memories.List(ctx, retrieval.Query{TenantID: tenant, Limit: 200})
					if err != nil {
						return err
					}
					if err := embedSvc.IndexMemories(ctx, memories); err != nil {
						return err
					}
				}
				return nil
			}),
		},
	}

	if stores.db != nil {
		db := stores.db
		retention := cfg.SchedulerJobsRetention
		jobs = append(jobs, scheduler.Job{
			Name:     "jobs-maintenance",
			Interval: cfg.SchedulerRetentionInterval,
			Run: leaderSafe(db, "memory-scheduler:jobs", func(ctx context.Context) error {
				if _, err := db.ExecContext(ctx,
					`DELETE FROM memory_jobs WHERE status = 'done' AND completed_at < now() - $1::interval`,
					fmt.Sprintf("%f seconds", retention.Seconds()),
				); err != nil {
					return err
				}
				// Belt and braces beyond the queue's own stale reclaim.
				_, err := db.ExecContext(ctx,
					`UPDATE memory_jobs SET status = 'retry', locked_at = NULL
					 WHERE status = 'processing' AND locked_at < now() - interval '15 minutes'`)
				return err
			}),
		})
	}

	if cfg.ConsolidationEnabled {
		consolidator := consolidation.NewService(stores.memories, stores.graph, idgen, clock)
		jobs = append(jobs, scheduler.Job{
			Name:     "compaction",
			Interval: cfg.SchedulerCompactInterval,
			Run: leaderSafe(stores.db, "memory-scheduler:compaction", func(ctx context.Context) error {
				for _, tenant := range tenantsFor(ctx, cfg, stores.db) {
					memories, err := stores.memories.List(ctx, retrieval.Query{TenantID: tenant, Limit: 200})
					if err != nil {
						return err
					}
					for _, mem := range memories {
						if !mem.IsActive(clock.Now()) {
							continue
						}
						if _, err := consolidator.Consolidate(ctx, mem); err != nil {
							return err
						}
					}
				}
				return nil
			}),
		})
	}

	return &SchedulerApp{Runner: scheduler.NewRunner(jobs...), Shutdown: shutdown}
}

// leaderSafe wraps a job with a postgres advisory lock when a db is
// available; in memory mode the job runs directly.
func leaderSafe(db *sql.DB, name string, fn func(context.Context) error) func(context.Context) error {
	if db == nil {
		return fn
	}
	return func(ctx context.Context) error {
		_, err := postgresstore.WithAdvisoryLock(ctx, db, name, fn)
		return err
	}
}

func tenantsFor(ctx context.Context, cfg config.Config, db *sql.DB) []string {
	if db != nil {
		if tenants, err := postgresstore.ListTenants(ctx, db); err == nil && len(tenants) > 0 {
			return tenants
		}
	}
	return []string{cfg.DefaultTenant}
}
