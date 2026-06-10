package retrieval

import (
	"github.com/agent-memory/agent-memory/internal/domain/agentic"
	"github.com/agent-memory/agent-memory/internal/domain/memory"
)

type ContextPack struct {
	Profile        []memory.Memory         `json:"profile,omitempty"`
	Preferences    []memory.Memory         `json:"preferences,omitempty"`
	ProjectMemory  []memory.Memory         `json:"project_memory,omitempty"`
	RecentEpisodes []memory.Memory         `json:"recent_episodes,omitempty"`
	RelevantFacts  []memory.Memory         `json:"relevant_facts,omitempty"`
	Procedures     []memory.Memory         `json:"procedures,omitempty"`
	Warnings       []string                `json:"warnings,omitempty"`
	Citations      []Citation              `json:"citations,omitempty"`
	TokenEstimate  int                     `json:"token_estimate"`
	TraceID        string                  `json:"trace_id,omitempty"`
	Control        *agentic.ControlContext `json:"control,omitempty"`
}

// Citation links a delivered memory back to its source events and the
// retrieval signals that selected it.
type Citation struct {
	MemoryID       string   `json:"memory_id"`
	SourceEventIDs []string `json:"source_event_ids,omitempty"`
	Score          float64  `json:"score"`
	Source         string   `json:"source"`
	Reasons        []string `json:"reasons,omitempty"`
}
