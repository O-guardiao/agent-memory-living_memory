package telemetry

// Define spans for ingestion, distillation, vector search, rerank and context assembly.

import (
	"context"
	"log/slog"
	"time"
)

// StartSpan records a named span: the returned func logs the duration and
// error, and forwards the span to the OTLP exporter when configured.
// Used at the ingestion, distillation, vector search, rerank and context
// assembly sites.
func StartSpan(ctx context.Context, name string) (context.Context, func(err error)) {
	start := time.Now()
	return ctx, func(err error) {
		duration := time.Since(start)
		attrs := []any{
			slog.String("span", name),
			slog.Duration("duration", duration),
		}
		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
		}
		slog.Debug("span", attrs...)
		exportSpan(name, start, duration, err)
	}
}
