package anthropic

// Placeholder for Anthropic client adapter. Implement ports.LLM here.

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/agent-memory/agent-memory/internal/adapters/httpretry"
	"github.com/agent-memory/agent-memory/internal/adapters/llm/llmjson"
	"github.com/agent-memory/agent-memory/internal/ports"
)

const apiVersion = "2023-06-01"

type Client struct {
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
}

func New(endpoint, apiKey, model string, client *http.Client) *Client {
	if client == nil {
		client = http.DefaultClient
	}
	return &Client{
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   apiKey,
		model:    model,
		client:   client,
	}
}

type messagesRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type messagesResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
}

func (c *Client) Generate(ctx context.Context, req ports.GenerateRequest) (ports.GenerateResponse, error) {
	body := messagesRequest{
		Model:     c.model,
		MaxTokens: 4096,
		System:    req.System,
		Messages:  []message{{Role: "user", Content: req.Prompt}},
	}
	headers := map[string]string{
		"x-api-key":         c.apiKey,
		"anthropic-version": apiVersion,
	}
	var resp messagesResponse
	if err := httpretry.PostJSON(ctx, c.client, c.endpoint+"/v1/messages", headers, body, &resp); err != nil {
		return ports.GenerateResponse{}, err
	}
	var text strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	return ports.GenerateResponse{Text: text.String()}, nil
}

func (c *Client) GenerateJSON(ctx context.Context, req ports.GenerateRequest, out any) error {
	jsonReq := req
	jsonReq.System = strings.TrimSpace(req.System + "\nReturn ONLY a single valid JSON value. No markdown, no code fences, no commentary.")
	resp, err := c.Generate(ctx, jsonReq)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(llmjson.Extract(resp.Text)), out)
}
