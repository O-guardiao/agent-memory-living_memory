package embedding

import (
	"context"
	"errors"
	"testing"
)

type fakeEmbedder struct {
	batchSizes []int
	failures   int
	calls      int
}

func (f *fakeEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	vectors, err := f.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return vectors[0], nil
}

func (f *fakeEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	f.calls++
	if f.failures > 0 {
		f.failures--
		return nil, errors.New("transient provider failure")
	}
	f.batchSizes = append(f.batchSizes, len(texts))
	out := make([][]float64, len(texts))
	for i := range texts {
		out[i] = []float64{float64(len(texts[i]))}
	}
	return out, nil
}

func TestBatcherChunksAndPreservesOrder(t *testing.T) {
	fake := &fakeEmbedder{}
	batcher := NewBatcher(fake, 2, 0)

	texts := []string{"a", "bb", "ccc", "dddd", "eeeee"}
	vectors, err := batcher.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("embed batch: %v", err)
	}
	if len(vectors) != len(texts) {
		t.Fatalf("expected %d vectors, got %d", len(texts), len(vectors))
	}
	for i, text := range texts {
		if vectors[i][0] != float64(len(text)) {
			t.Fatalf("order broken at %d: %#v", i, vectors[i])
		}
	}
	if len(fake.batchSizes) != 3 || fake.batchSizes[0] != 2 || fake.batchSizes[2] != 1 {
		t.Fatalf("unexpected chunking: %v", fake.batchSizes)
	}
}

func TestBatcherRetriesTransientFailures(t *testing.T) {
	fake := &fakeEmbedder{failures: 2}
	batcher := NewBatcher(fake, 10, 3)
	batcher.baseDelay = 0

	if _, err := batcher.EmbedBatch(context.Background(), []string{"x"}); err != nil {
		t.Fatalf("expected retry to succeed: %v", err)
	}
	if fake.calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", fake.calls)
	}
}

func TestBatcherGivesUpAfterMaxRetries(t *testing.T) {
	fake := &fakeEmbedder{failures: 10}
	batcher := NewBatcher(fake, 10, 1)
	batcher.baseDelay = 0

	if _, err := batcher.EmbedBatch(context.Background(), []string{"x"}); err == nil {
		t.Fatal("expected failure after exhausting retries")
	}
}
