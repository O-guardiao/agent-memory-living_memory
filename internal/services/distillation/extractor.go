package distillation

// Replace the heuristic extractor with an LLM-backed extractor that emits typed, cited memory candidates.

import (
	"context"
	"strings"

	"github.com/agent-memory/agent-memory/internal/domain/memory"
	"github.com/agent-memory/agent-memory/internal/ports"
)

// Extractor is satisfied by *Service and by custom distillers.
type Extractor interface {
	Extract(ctx context.Context, event memory.Event) ([]memory.Memory, error)
}

// NewLLMService builds a Service that asks the LLM for typed, cited memory
// candidates and falls back to the heuristic path when the provider fails.
func NewLLMService(llm ports.LLM, idgen ports.IDGenerator, clock ports.Clock) *Service {
	svc := NewService(idgen, clock)
	svc.llm = llm
	return svc
}

// llmExtract emits typed memory candidates citing the source event. Any
// provider error returns nil so ingestion falls back to the heuristic path.
func (s *Service) llmExtract(ctx context.Context, event memory.Event, text string) []memory.Memory {
	var extracted []ExtractedMemorySchema
	req := ports.GenerateRequest{
		System: ExtractionSystemPrompt,
		Prompt: extractionUserPrompt(event, text),
	}
	if err := s.llm.GenerateJSON(ctx, req, &extracted); err != nil || len(extracted) == 0 {
		return nil
	}

	now := s.clock.Now()
	memories := make([]memory.Memory, 0, len(extracted))
	for _, item := range extracted {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		typ := memory.Type(item.Type)
		switch typ {
		case memory.TypeEpisode, memory.TypeFact, memory.TypePreference, memory.TypeProcedure:
		default:
			typ = classify(content)
		}
		scope := memory.Scope(item.Scope)
		switch scope {
		case memory.ScopeUser, memory.ScopeProject:
		default:
			scope = memory.ScopeUser
			if event.ProjectID != "" {
				scope = memory.ScopeProject
			}
		}

		mem := memory.NewMemory(now, typ, scope, normalizeMemoryText(content))
		mem.ID = s.idgen.NewID("mem")
		mem.TenantID = event.TenantID
		mem.UserID = event.UserID
		mem.AgentID = event.AgentID
		mem.ProjectID = event.ProjectID
		mem.SessionID = event.SessionID
		mem.SourceEventIDs = []string{event.ID}
		mem.Confidence = memory.ClampScore(item.Confidence)
		mem.Importance = memory.ClampScore(item.Importance)
		mem.SafetyLabels = safetyLabels(content)
		mem.Metadata = map[string]any{"distiller": "llm_v1"}
		memories = append(memories, mem)
	}
	return memories
}
