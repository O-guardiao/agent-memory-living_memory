package voyageembed

// Placeholder for Voyage embedding adapter. Implement ports.Embedder here.

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/agent-memory/agent-memory/internal/adapters/httpretry"
)

const defaultEndpoint = "https://api.voyageai.com"

type Embedder struct {
	endpoint   string
	apiKey     string
	model      string
	dimensions int
	client     *http.Client
}

func New(apiKey, model string, dimensions int, client *http.Client) *Embedder {
	return NewWithEndpoint(defaultEndpoint, apiKey, model, dimensions, client)
}

func NewWithEndpoint(endpoint, apiKey, model string, dimensions int, client *http.Client) *Embedder {
	if client == nil {
		client = http.DefaultClient
	}
	return &Embedder{
		endpoint:   strings.TrimRight(endpoint, "/"),
		apiKey:     apiKey,
		model:      model,
		dimensions: dimensions,
		client:     client,
	}
}

type embedRequest struct {
	Model           string   `json:"model"`
	Input           []string `json:"input"`
	OutputDimension int      `json:"output_dimension,omitempty"`
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
	headers := map[string]string{"Authorization": "Bearer " + e.apiKey}
	var resp embedResponse
	err := httpretry.PostJSON(ctx, e.client, e.endpoint+"/v1/embeddings", headers, embedRequest{
		Model:           e.model,
		Input:           texts,
		OutputDimension: e.dimensions,
	}, &resp)
	if err != nil {
		return nil, err
	}
	if len(resp.Data) != len(texts) {
		return nil, errors.New("voyage embeddings: response size mismatch")
	}
	out := make([][]float64, len(texts))
	for _, item := range resp.Data {
		if item.Index < 0 || item.Index >= len(out) {
			return nil, errors.New("voyage embeddings: index out of range")
		}
		out[item.Index] = item.Embedding
	}
	return out, nil
}
