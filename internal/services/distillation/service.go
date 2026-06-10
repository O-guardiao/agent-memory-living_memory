package distillation

import (
	"context"
	"strings"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/ports"
	"github.com/agent-memory/agent-memory/internal/telemetry"
)

type Service struct {
	idgen ports.IDGenerator
	clock ports.Clock
	// llm, when set, produces typed candidates with heuristic fallback.
	llm ports.LLM
}

func NewService(idgen ports.IDGenerator, clock ports.Clock) *Service {
	return &Service{idgen: idgen, clock: clock}
}

func (s *Service) Extract(ctx context.Context, event memory.Event) (memories []memory.Memory, err error) {
	ctx, endSpan := telemetry.StartSpan(ctx, "distillation")
	defer func() { endSpan(err) }()

	text := strings.TrimSpace(event.Content)
	if text == "" {
		text = strings.TrimSpace(event.ToolResult)
	}
	if text == "" {
		return nil, nil
	}
	if s.llm != nil {
		if memories := s.llmExtract(ctx, event, text); len(memories) > 0 {
			return memories, nil
		}
		// Provider failure or empty output: fall back to the heuristic path
		// so ingestion never fails because an LLM is down.
	}
	now := s.clock.Now()
	typ := classify(text)
	scope := memory.ScopeUser
	if event.ProjectID != "" {
		scope = memory.ScopeProject
	}

	mem := memory.NewMemory(now, typ, scope, normalizeMemoryText(text))
	mem.ID = s.idgen.NewID("mem")
	mem.TenantID = event.TenantID
	mem.UserID = event.UserID
	mem.AgentID = event.AgentID
	mem.ProjectID = event.ProjectID
	mem.SessionID = event.SessionID
	mem.SourceEventIDs = []string{event.ID}
	mem.Confidence = confidenceFor(event, typ)
	mem.Importance = importanceFor(text, typ)
	mem.SafetyLabels = safetyLabels(text)
	mem.Metadata = map[string]any{"distiller": "heuristic_v1"}
	return []memory.Memory{mem}, nil
}

func classify(text string) memory.Type {
	t := strings.ToLower(text)
	switch {
	case containsAny(t, "prefiro", "prefer", "gosto", "não gosto", "quero que", "my preference"):
		return memory.TypePreference
	case containsAny(t, "quando", "sempre que", "use este processo", "passo", "workflow"):
		return memory.TypeProcedure
	case containsAny(t, "estou", "sou", "trabalho", "construindo", "projeto", "empresa"):
		return memory.TypeFact
	default:
		return memory.TypeEpisode
	}
}

func normalizeMemoryText(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func containsAny(text string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func confidenceFor(event memory.Event, typ memory.Type) float64 {
	if event.Role == memory.RoleUser {
		if typ == memory.TypePreference || typ == memory.TypeFact {
			return 0.86
		}
		return 0.74
	}
	return 0.62
}

func importanceFor(text string, typ memory.Type) float64 {
	base := 0.45
	if typ == memory.TypePreference || typ == memory.TypeProcedure {
		base = 0.70
	}
	if len(text) > 160 {
		base += 0.10
	}
	return memory.ClampScore(base)
}

func safetyLabels(text string) []memory.SafetyLabel {
	t := strings.ToLower(text)
	labels := []memory.SafetyLabel{memory.SafetyInternal}
	if containsAny(t, "senha", "password", "api key", "token", "secret") {
		labels = append(labels, memory.SafetyCredential)
	}
	if strings.Contains(t, "@") || containsAny(t, "cpf", "rg", "telefone") {
		labels = append(labels, memory.SafetyPII)
	}
	return labels
}
