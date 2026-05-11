package ingestion

import (
	"context"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/ports"
	"github.com/agent-memory/agent-memory/internal/services/distillation"
	"github.com/agent-memory/agent-memory/internal/services/embedding"
)

type Dependencies struct {
	Events    ports.EventStore
	Memories  ports.MemoryStore
	Embedder  ports.Embedder
	Vectors   ports.VectorStore
	Graph     ports.GraphStore
	Distiller *distillation.Service
	EmbedSvc  *embedding.Service
	IDGen     ports.IDGenerator
	Clock     ports.Clock
}

type Service struct {
	deps Dependencies
}

type IngestRequest struct {
	TenantID   string           `json:"tenant_id"`
	UserID     string           `json:"user_id"`
	AgentID    string           `json:"agent_id"`
	SessionID  string           `json:"session_id"`
	ProjectID  string           `json:"project_id,omitempty"`
	Role       memory.EventRole `json:"role"`
	Content    string           `json:"content"`
	ToolName   string           `json:"tool_name,omitempty"`
	ToolResult string           `json:"tool_result,omitempty"`
	Metadata   map[string]any   `json:"metadata,omitempty"`
}

type IngestResponse struct {
	Event    memory.Event    `json:"event"`
	Memories []memory.Memory `json:"memories"`
}

func NewService(deps Dependencies) *Service {
	return &Service{deps: deps}
}

func (s *Service) Ingest(ctx context.Context, req IngestRequest) (IngestResponse, error) {
	now := s.deps.Clock.Now()
	event := memory.Event{
		ID:         s.deps.IDGen.NewID("evt"),
		TenantID:   req.TenantID,
		UserID:     req.UserID,
		AgentID:    req.AgentID,
		SessionID:  req.SessionID,
		ProjectID:  req.ProjectID,
		Role:       req.Role,
		Content:    NormalizeText(req.Content),
		ToolName:   req.ToolName,
		ToolResult: NormalizeText(req.ToolResult),
		CreatedAt:  now,
		Metadata:   req.Metadata,
	}
	if err := ValidateEvent(event); err != nil {
		return IngestResponse{}, err
	}
	if err := s.deps.Events.Append(ctx, event); err != nil {
		return IngestResponse{}, err
	}

	candidates, err := s.deps.Distiller.Extract(ctx, event)
	if err != nil {
		return IngestResponse{}, err
	}

	created := make([]memory.Memory, 0, len(candidates))
	for _, mem := range candidates {
		if IsDuplicate(created, mem) {
			continue
		}
		if err := s.deps.Memories.Upsert(ctx, mem); err != nil {
			return IngestResponse{}, err
		}
		created = append(created, mem)
	}
	if err := s.deps.EmbedSvc.IndexMemories(ctx, created); err != nil {
		return IngestResponse{}, err
	}
	if s.deps.Graph != nil {
		for _, mem := range created {
			if err := s.deps.Graph.UpsertNode(ctx, memoryNode(mem)); err != nil {
				return IngestResponse{}, err
			}
		}
	}

	return IngestResponse{Event: event, Memories: created}, nil
}

func memoryNode(mem memory.Memory) ports.GraphNode {
	return ports.GraphNode{
		ID:     mem.ID,
		Labels: []string{"Memory", string(mem.Type)},
		Payload: map[string]string{
			"tenant_id":  mem.TenantID,
			"user_id":    mem.UserID,
			"agent_id":   mem.AgentID,
			"project_id": mem.ProjectID,
			"type":       string(mem.Type),
			"scope":      string(mem.Scope),
			"status":     string(mem.Status),
		},
	}
}
