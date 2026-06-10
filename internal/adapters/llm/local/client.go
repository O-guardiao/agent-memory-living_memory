package local

// Placeholder for local model adapter. Implement ports.LLM here.

import (
	"context"
	"net/http"

	"github.com/agent-memory/agent-memory/internal/adapters/llm/openai"
	"github.com/agent-memory/agent-memory/internal/ports"
)

// Client serves OpenAI-compatible local endpoints (Ollama /v1, vLLM,
// llama.cpp server) by delegating to the OpenAI adapter without a key.
type Client struct {
	inner *openai.Client
}

func New(endpoint, model string, client *http.Client) *Client {
	return &Client{inner: openai.New(endpoint, "", model, client)}
}

func (c *Client) Generate(ctx context.Context, req ports.GenerateRequest) (ports.GenerateResponse, error) {
	return c.inner.Generate(ctx, req)
}

func (c *Client) GenerateJSON(ctx context.Context, req ports.GenerateRequest, out any) error {
	return c.inner.GenerateJSON(ctx, req, out)
}
