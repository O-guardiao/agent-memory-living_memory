package localembed

// Placeholder for local embedding adapter. Implement ports.Embedder here.

import (
	"context"
	"net/http"

	openaiembed "github.com/agent-memory/agent-memory/internal/adapters/embeddings/openai"
)

// Embedder serves OpenAI-compatible local embedding endpoints (Ollama /v1,
// llama.cpp server) by delegating to the OpenAI adapter without a key.
type Embedder struct {
	inner *openaiembed.Embedder
}

func New(endpoint, model string, dimensions int, client *http.Client) *Embedder {
	return &Embedder{inner: openaiembed.New(endpoint, "", model, dimensions, client)}
}

func (e *Embedder) Embed(ctx context.Context, text string) ([]float64, error) {
	return e.inner.Embed(ctx, text)
}

func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	return e.inner.EmbedBatch(ctx, texts)
}
