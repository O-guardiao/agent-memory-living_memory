package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"syscall"
	"time"

	"os/signal"

	"github.com/agent-memory/agent-memory/internal/bootstrap"
	"github.com/agent-memory/agent-memory/internal/config"
	"github.com/agent-memory/agent-memory/internal/telemetry"
)

func main() {
	// Wires retention, reindexing and periodic compaction jobs.
	cfg := config.Load()
	slog.SetDefault(telemetry.NewLogger(cfg.LogLevel))
	telemetry.Init(cfg.OTLPEndpoint)
	app := bootstrap.NewSchedulerApp(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("memory-scheduler running retention/reindex/compaction jobs in %s mode", cfg.StorageMode)
	if err := app.Runner.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("scheduler failed: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.Shutdown.Run(shutdownCtx); err != nil {
		log.Printf("shutdown hooks failed: %v", err)
	}
}
