package mockllm

import (
	"context"
	"encoding/json"

	"github.com/agent-memory/agent-memory/internal/ports"
)

type Client struct{}

func New() Client { return Client{} }

func (Client) Generate(ctx context.Context, req ports.GenerateRequest) (ports.GenerateResponse, error) {
	_ = ctx
	return ports.GenerateResponse{Text: req.Prompt}, nil
}

func (Client) GenerateJSON(ctx context.Context, req ports.GenerateRequest, out any) error {
	_ = ctx
	return json.Unmarshal([]byte(req.Prompt), out)
}
