package ports

import "context"

type GenerateRequest struct {
	System string
	Prompt string
}

type GenerateResponse struct {
	Text string
}

type LLM interface {
	Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)
	GenerateJSON(ctx context.Context, req GenerateRequest, out any) error
}
