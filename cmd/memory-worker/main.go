package main

import (
	"context"
	"errors"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/agent-memory/agent-memory/internal/bootstrap"
	"github.com/agent-memory/agent-memory/internal/config"
)

func main() {
	cfg := config.Load()
	app := bootstrap.NewApp(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("memory-worker subscribed to async ingestion in %s mode", cfg.StorageMode)
	if err := app.Worker.StartWorker(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("worker failed: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.Shutdown.Run(shutdownCtx); err != nil {
		log.Printf("shutdown hooks failed: %v", err)
	}
}
