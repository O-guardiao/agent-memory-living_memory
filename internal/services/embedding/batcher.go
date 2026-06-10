package embedding

// Add batch sizing, rate-limit handling and retry policy here for production embedding providers.

import (
	"context"
	"time"

	"github.com/agent-memory/agent-memory/internal/ports"
)

// Batcher decorates a remote embedder with chunked batches and retry with
// exponential backoff, preserving input order in the output.
type Batcher struct {
	inner      ports.Embedder
	maxBatch   int
	maxRetries int
	baseDelay  time.Duration
}

func NewBatcher(inner ports.Embedder, maxBatch, maxRetries int) *Batcher {
	if maxBatch <= 0 {
		maxBatch = 64
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	return &Batcher{
		inner:      inner,
		maxBatch:   maxBatch,
		maxRetries: maxRetries,
		baseDelay:  250 * time.Millisecond,
	}
}

func (b *Batcher) Embed(ctx context.Context, text string) ([]float64, error) {
	var vec []float64
	err := b.withRetry(ctx, func() error {
		var innerErr error
		vec, innerErr = b.inner.Embed(ctx, text)
		return innerErr
	})
	return vec, err
}

func (b *Batcher) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	out := make([][]float64, 0, len(texts))
	for start := 0; start < len(texts); start += b.maxBatch {
		end := start + b.maxBatch
		if end > len(texts) {
			end = len(texts)
		}
		chunk := texts[start:end]
		var vectors [][]float64
		err := b.withRetry(ctx, func() error {
			var innerErr error
			vectors, innerErr = b.inner.EmbedBatch(ctx, chunk)
			return innerErr
		})
		if err != nil {
			return nil, err
		}
		out = append(out, vectors...)
	}
	return out, nil
}

func (b *Batcher) withRetry(ctx context.Context, call func() error) error {
	var lastErr error
	for attempt := 0; attempt <= b.maxRetries; attempt++ {
		if attempt > 0 {
			delay := b.baseDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		if lastErr = call(); lastErr == nil {
			return nil
		}
	}
	return lastErr
}
