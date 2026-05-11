package ports

import "context"

type GraphNode struct {
	ID      string
	Labels  []string
	Payload map[string]string
}

type GraphEdge struct {
	FromID  string
	ToID    string
	Type    string
	Payload map[string]string
}

type GraphQuery struct {
	TenantID string
	StartID  string
	Depth    int
}

type GraphResult struct {
	NodeID string
	Score  float64
}

type GraphStore interface {
	UpsertNode(ctx context.Context, node GraphNode) error
	UpsertEdge(ctx context.Context, edge GraphEdge) error
	Traverse(ctx context.Context, query GraphQuery) ([]GraphResult, error)
}
