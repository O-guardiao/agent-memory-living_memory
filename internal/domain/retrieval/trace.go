package retrieval

import "time"

type Trace struct {
	ID            string             `json:"id"`
	Query         string             `json:"query"`
	TenantID      string             `json:"tenant_id"`
	UserID        string             `json:"user_id"`
	CandidateIDs  []string           `json:"candidate_ids,omitempty"`
	SelectedIDs   []string           `json:"selected_ids,omitempty"`
	RejectedIDs   []string           `json:"rejected_ids,omitempty"`
	Scores        map[string]float64 `json:"scores,omitempty"`
	Filters       map[string]string  `json:"filters,omitempty"`
	LatencyMS     int64              `json:"latency_ms"`
	TokenEstimate int                `json:"token_estimate"`
	CreatedAt     time.Time          `json:"created_at"`
}
