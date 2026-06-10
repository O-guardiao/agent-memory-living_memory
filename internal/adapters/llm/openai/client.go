package openai

// Placeholder for OpenAI client adapter. Implement ports.LLM here.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/agent-memory/agent-memory/internal/adapters/httpretry"
	"github.com/agent-memory/agent-memory/internal/adapters/llm/llmjson"
	"github.com/agent-memory/agent-memory/internal/ports"
)

type Client struct {
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
}

// New builds a client for the OpenAI chat completions API or any
// OpenAI-compatible endpoint. An empty apiKey omits the bearer header,
// which local servers accept.
func New(endpoint, apiKey, model string, client *http.Client) *Client {
	if client == nil {
		client = http.DefaultClient
	}
	// Accept endpoints given with or without a trailing /v1 (Ollama-style).
	endpoint = strings.TrimSuffix(strings.TrimRight(endpoint, "/"), "/v1")
	return &Client{
		endpoint: endpoint,
		apiKey:   apiKey,
		model:    model,
		client:   client,
	}
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *Client) Generate(ctx context.Context, req ports.GenerateRequest) (ports.GenerateResponse, error) {
	return c.generate(ctx, req, nil)
}

func (c *Client) GenerateJSON(ctx context.Context, req ports.GenerateRequest, out any) error {
	resp, err := c.generate(ctx, req, &responseFormat{Type: "json_object"})
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(llmjson.Extract(resp.Text)), out)
}

func (c *Client) generate(ctx context.Context, req ports.GenerateRequest, format *responseFormat) (ports.GenerateResponse, error) {
	messages := make([]chatMessage, 0, 2)
	if req.System != "" {
		messages = append(messages, chatMessage{Role: "system", Content: req.System})
	}
	messages = append(messages, chatMessage{Role: "user", Content: req.Prompt})

	headers := map[string]string{}
	if c.apiKey != "" {
		headers["Authorization"] = "Bearer " + c.apiKey
	}
	var resp chatResponse
	err := httpretry.PostJSON(ctx, c.client, c.endpoint+"/v1/chat/completions", headers, chatRequest{
		Model:          c.model,
		Messages:       messages,
		ResponseFormat: format,
	}, &resp)
	if err != nil {
		return ports.GenerateResponse{}, err
	}
	if len(resp.Choices) == 0 {
		return ports.GenerateResponse{}, errors.New("openai: empty choices in response")
	}
	return ports.GenerateResponse{Text: resp.Choices[0].Message.Content}, nil
}
