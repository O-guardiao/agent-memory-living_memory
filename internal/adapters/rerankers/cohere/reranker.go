package cohere

// Placeholder for Cohere reranker adapter. Implement ports.Reranker here.

import (
	"context"
	"net/http"
	"strings"

	"github.com/agent-memory/agent-memory/internal/adapters/httpretry"
	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

const defaultEndpoint = "https://api.cohere.com"

type Reranker struct {
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
}

func New(apiKey, model string, client *http.Client) *Reranker {
	return NewWithEndpoint(defaultEndpoint, apiKey, model, client)
}

func NewWithEndpoint(endpoint, apiKey, model string, client *http.Client) *Reranker {
	if client == nil {
		client = http.DefaultClient
	}
	return &Reranker{
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   apiKey,
		model:    model,
		client:   client,
	}
}

type rerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n"`
}

type rerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
	} `json:"results"`
}

func (r *Reranker) Rerank(ctx context.Context, query string, candidates []retrieval.Candidate) ([]retrieval.Candidate, error) {
	if len(candidates) == 0 {
		return candidates, nil
	}
	documents := make([]string, len(candidates))
	for i, cand := range candidates {
		documents[i] = cand.Memory.Content
	}
	headers := map[string]string{"Authorization": "Bearer " + r.apiKey}
	var resp rerankResponse
	err := httpretry.PostJSON(ctx, r.client, r.endpoint+"/v2/rerank", headers, rerankRequest{
		Model:     r.model,
		Query:     query,
		Documents: documents,
		TopN:      len(documents),
	}, &resp)
	if err != nil {
		return nil, err
	}
	for _, result := range resp.Results {
		if result.Index < 0 || result.Index >= len(candidates) {
			continue
		}
		cand := &candidates[result.Index]
		cand.Score = 0.5*cand.Score + 0.5*result.RelevanceScore
		cand.Reasons = append(cand.Reasons, "cohere_rerank")
	}
	retrieval.SortCandidates(candidates)
	return candidates, nil
}
