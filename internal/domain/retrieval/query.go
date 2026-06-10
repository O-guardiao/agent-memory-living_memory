package retrieval

import (
	"strings"
	"time"
)

type Query struct {
	TenantID     string            `json:"tenant_id"`
	UserID       string            `json:"user_id"`
	AgentID      string            `json:"agent_id,omitempty"`
	ProjectID    string            `json:"project_id,omitempty"`
	SessionID    string            `json:"session_id,omitempty"`
	Text         string            `json:"query"`
	Limit        int               `json:"limit,omitempty"`
	TokenBudget  int               `json:"token_budget,omitempty"`
	IncludeTrace bool              `json:"include_trace,omitempty"`
	Now          time.Time         `json:"-"`
	Filters      map[string]string `json:"filters,omitempty"`
}

func (q Query) NormalizedLimit() int {
	if q.Limit <= 0 {
		return 8
	}
	if q.Limit > 50 {
		return 50
	}
	return q.Limit
}

func (q Query) NormalizedText() string {
	return strings.TrimSpace(q.Text)
}
