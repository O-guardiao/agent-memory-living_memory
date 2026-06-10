package openaiembed

// Placeholder for OpenAI embedding adapter. Implement ports.Embedder here.

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/agent-memory/agent-memory/internal/adapters/httpretry"
)

type Embedder struct {
	endpoint   string
	apiKey     string
	model      string
	dimensions int
	client     *http.Client
}

// New builds an embedder for the OpenAI embeddings API or compatible
// endpoints. dimensions must match the vector store collection size; zero
// lets the provider use its default.
func New(endpoint, apiKey, model string, dimensions int, client *http.Client) *Embedder {
	if client == nil {
		client = http.DefaultClient
	}
	endpoint = strings.TrimSuffix(strings.TrimRight(endpoint, "/"), "/v1")
	return &Embedder{
		endpoint:   endpoint,
		apiKey:     apiKey,
		model:      model,
		dimensions: dimensions,
		client:     client,
	}
}

type embedRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

type embedResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

func (e *Embedder) Embed(ctx context.Context, text string) ([]float64, error) {
	vectors, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return vectors[0], nil
}

func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	headers := map[string]string{}
	if e.apiKey != "" {
		headers["Authorization"] = "Bearer " + e.apiKey
	}
	var resp embedResponse
	err := httpretry.PostJSON(ctx, e.client, e.endpoint+"/v1/embeddings", headers, embedRequest{
		Model:      e.model,
		Input:      texts,
		Dimensions: e.dimensions,
	}, &resp)
	if err != nil {
		return nil, err
	}
	if len(resp.Data) != len(texts) {
		return nil, errors.New("openai embeddings: response size mismatch")
	}
	out := make([][]float64, len(texts))
	for _, item := range resp.Data {
		if item.Index < 0 || item.Index >= len(out) {
			return nil, errors.New("openai embeddings: index out of range")
		}
		out[item.Index] = item.Embedding
	}
	return out, nil
}
