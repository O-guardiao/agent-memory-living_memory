package ports

import "context"

type VectorItem struct {
	ID      string
	Vector  []float64
	Payload map[string]string
}

type VectorQuery struct {
	TenantID string
	UserID   string
	Text     string
	Vector   []float64
	Limit    int
	Filters  map[string]string
}

type VectorResult struct {
	ID    string
	Score float64
}

type VectorStore interface {
	Upsert(ctx context.Context, items []VectorItem) error
	Search(ctx context.Context, query VectorQuery) ([]VectorResult, error)
	Delete(ctx context.Context, ids []string) error
}
