package ports

import (
	"context"

	"github.com/agent-memory/agent-memory/internal/domain/retrieval"
)

type KeywordStore interface {
	Index(ctx context.Context, id, text string, payload map[string]string) error
	Search(ctx context.Context, query retrieval.Query) ([]VectorResult, error)
	Delete(ctx context.Context, id string) error
}
