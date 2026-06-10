package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agent-memory/agent-memory/internal/bootstrap"
	"github.com/agent-memory/agent-memory/internal/config"
	"github.com/agent-memory/agent-memory/internal/telemetry"
)

func main() {
	cfg := config.Load()
	slog.SetDefault(telemetry.NewLogger(cfg.LogLevel))
	telemetry.Init(cfg.OTLPEndpoint)
	app := bootstrap.NewApp(cfg)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           app.HTTPServer,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("memory-api listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	if err := app.Shutdown.Run(ctx); err != nil {
		log.Printf("shutdown hooks failed: %v", err)
	}
}
