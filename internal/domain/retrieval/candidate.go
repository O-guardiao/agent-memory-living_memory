package retrieval

import "github.com/agent-memory/agent-memory/internal/domain/memory"

type Candidate struct {
	Memory  memory.Memory `json:"memory"`
	Score   float64       `json:"score"`
	Reasons []string      `json:"reasons,omitempty"`
	Source  string        `json:"source"`
}
