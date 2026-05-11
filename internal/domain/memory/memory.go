package memory

import "time"

type Memory struct {
	ID             string         `json:"id"`
	TenantID       string         `json:"tenant_id"`
	UserID         string         `json:"user_id"`
	AgentID        string         `json:"agent_id,omitempty"`
	ProjectID      string         `json:"project_id,omitempty"`
	SessionID      string         `json:"session_id,omitempty"`
	Type           Type           `json:"type"`
	Content        string         `json:"content"`
	Summary        string         `json:"summary,omitempty"`
	SourceEventIDs []string       `json:"source_event_ids,omitempty"`
	Confidence     float64        `json:"confidence"`
	Importance     float64        `json:"importance"`
	ValidFrom      time.Time      `json:"valid_from"`
	ValidUntil     *time.Time     `json:"valid_until,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	Status         Status         `json:"status"`
	Scope          Scope          `json:"scope"`
	Version        int            `json:"version"`
	SafetyLabels   []SafetyLabel  `json:"safety_labels,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

func NewMemory(now time.Time, typ Type, scope Scope, content string) Memory {
	return Memory{
		Type:       typ,
		Scope:      scope,
		Content:    content,
		Confidence: 0.70,
		Importance: 0.50,
		ValidFrom:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
		Status:     StatusActive,
		Version:    1,
	}
}

func (m Memory) IsActive(at time.Time) bool {
	if m.Status != StatusActive {
		return false
	}
	if !m.ValidFrom.IsZero() && at.Before(m.ValidFrom) {
		return false
	}
	if m.ValidUntil != nil && at.After(*m.ValidUntil) {
		return false
	}
	return true
}
